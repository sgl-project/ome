package model_agent

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/casperagent"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/utils"
)

var (
	BigFileSizeInMB                  = 200
	DefaultDownloadChunkSizeInMB     = 200
	DefaultDownloadThreads           = 5
	DefaultMultipartDownloadThreads  = 5
	DefaultSmallFilesDownloadThreads = 10
	DefaultFilterFilesThreads        = 10
)

type SyncerTaskType string

const (
	Download           SyncerTaskType = "Download"
	DownloadOverride   SyncerTaskType = "DownloadOverride"
	Delete             SyncerTaskType = "Delete"
)

type SyncerTask struct {
	TaskType         SyncerTaskType
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

type Syncer struct {
	downloadRetry      int
	modelRootDir       string
	modelRootDirOnHost string
	casperDataStore    casper.CasperDataStore
	syncerChan         <-chan *SyncerTask
	nodeLabeler        *NodeLabeler
	logger             *zap.SugaredLogger
}

func NewSyncer(authType string,
			   downloadRetry int,
			   modelRootDir string,
			   modelRootDirOnHost string,
			   syncerChan <-chan *SyncerTask,
			   nodeLabeler *NodeLabeler,
			   logger *zap.SugaredLogger) (*Syncer, error) {
	casperDataStore, err := NewCasperDataStore(authType)
	if err != nil {
		logger.Errorf("Not able to initalize the casper data store: %s", err.Error())
		return nil, err
	}

	return &Syncer{
		downloadRetry: downloadRetry,
		modelRootDir: modelRootDir,
		modelRootDirOnHost: modelRootDirOnHost,
		casperDataStore: casperDataStore,
		syncerChan: syncerChan,
		nodeLabeler: nodeLabeler,
		logger: logger,
	}, nil
}

func (s *Syncer) Run(stopCh <-chan struct{}, numWorker int) {
	s.logger.Info("Starting syncer")

	for i := 0; i < numWorker; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	s.logger.Info("Started syncer workers")
	<-stopCh
	s.logger.Info("Shutting down syncer workers")
}

func (s *Syncer) runWorker() {
	for {
		select {
		case task, ok := <-s.syncerChan:
			if ok {
				err := s.processTask(task)
				if err != nil {
					s.logger.Errorf("Syncer task failed with error: %s", err.Error())
				}
			} else {
				s.logger.Info("syncer channel closed, worker exits.")
				return
			}
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *Syncer) processTask(task *SyncerTask) error {
	if task.BaseModel == nil && task.ClusterBaseModel == nil {
		return fmt.Errorf("syncer got empty task")
	}

	casperUri, destPath, err := getTargetDirPath(task.BaseModel, task.ClusterBaseModel, s.modelRootDir, s.modelRootDirOnHost)
	if err != nil {
		s.markModelOnNodeFailed(task)
		return err
	}

	switch task.TaskType {
	case Download:
		// we might implement a "delete/cleanup and then download" logic to update a model in the future
		// use a single download function for now
		fallthrough
	case DownloadOverride:
		err := utils.Retry(s.downloadRetry, 100 * time.Millisecond, func() error {
			return s.downloadModel(casperUri, destPath)
		})
		if err != nil {
			s.markModelOnNodeFailed(task)
			return err
		}

		if task.BaseModel != nil {
			s.logger.Info("successfully downloaded the BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
		} else {
			s.logger.Info("successfully downloaded the ClusterBaseModel %s", task.ClusterBaseModel.Name)
		}

		// mark model as Ready
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Ready,
			BaseModel: task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		} 

		err = s.nodeLabeler.processOp(nodeLabelOp)
		if err != nil {
			return err
		}
	case Delete:
		err := s.deleteModel(destPath)
		if err != nil {
			return err
		}
		if task.BaseModel != nil {
			s.logger.Info("successfully deleted the BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
		} else {
			s.logger.Info("successfully deleted the ClusterBaseModel %s", task.ClusterBaseModel.Name)
		}
	}

	return nil
}

func (s *Syncer) markModelOnNodeFailed(task *SyncerTask) {
	nodeLabelOp := &NodeLabelOp{
		ModelStateOnNode: Failed,
		BaseModel: task.BaseModel,
		ClusterBaseModel: task.ClusterBaseModel,
	} 

	err := s.nodeLabeler.processOp(nodeLabelOp)
	if err != nil {
		s.logger.Errorf("node label failed with error: %s", err.Error())
	}
}

func getTargetDirPath(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel, modelRootDir string, modelRootDirOnHost string) (*casper.ObjectURI, string, error) {
	var destPath string 
	var storagePath string
	if baseModel != nil {
		if baseModel.Spec.Storage.Path == nil {
			return nil, "", fmt.Errorf("Got empty storge path in baseModel %s in namespace %s", baseModel.Name, baseModel.Namespace)
		}
		storagePath = *baseModel.Spec.Storage.Path
		if baseModel.Spec.Storage.StorageUri != nil {
			destPath = *baseModel.Spec.Storage.StorageUri
		}
	} else {
		if clusterBaseModel.Spec.Storage.Path == nil {
			return nil, "", fmt.Errorf("Got empty storge path in clusterBaseModel %s", clusterBaseModel.Name)
		}
		storagePath = *clusterBaseModel.Spec.Storage.Path
		if clusterBaseModel.Spec.Storage.StorageUri != nil {
			destPath = *clusterBaseModel.Spec.Storage.StorageUri
		}
	}

	osUri, err := NewObjectStorageUri(storagePath) 
	if err != nil {
		return nil, "", err
	}

	if !strings.HasSuffix(osUri.Prefix, "/") {
		osUri.Prefix = osUri.Prefix + "/"
	}

	if len(destPath) == 0 {
		if strings.HasSuffix(modelRootDir, "/") {
			destPath = modelRootDir + osUri.Prefix
		} else {
			destPath = modelRootDir + "/" + osUri.Prefix
		}
	} else {
		if !strings.HasPrefix(destPath, modelRootDirOnHost) {
			return nil, "", fmt.Errorf("user defined destination path {%s} is not under model root dir {%s} of the host", destPath, modelRootDir)
		} else {
			destPath = strings.Replace(destPath, modelRootDirOnHost, modelRootDir, 1)
		}
	}

	return osUri, destPath, nil
}

func (s *Syncer) downloadModel(uri *casper.ObjectURI, destPath string) error {
	s.logger.Infof("Making call to object storage with endpoint %s", s.casperDataStore.CasperClient.Endpoint())
	objects, err := s.casperDataStore.ListObjects(*uri)
	if err != nil {
		return err
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	s.logger.Infof("Done with list all %d objects in model bucket folder", len(objects))

	objectsChannel := prepareObjectsChannel(objects)

	s.logger.Info("Start to filter objects...")
	filteredObjects := s.casperDataStore.FilterObjectsMultiThreads(DefaultFilterFilesThreads, s.logger, uri, destPath, objectsChannel, uri.Prefix)

	// 4. Split files per size into two groups
	smallFiles := make([]objectstorage.ObjectSummary, 0)
	largeFiles := make([]objectstorage.ObjectSummary, 0)
	for object := range filteredObjects {
		if object.Size == nil || *object.Size < int64(BigFileSizeInMB)*int64(casper.MB) {
			smallFiles = append(smallFiles, object)
		} else {
			largeFiles = append(largeFiles, object)
		}
	}

	// Download small files with multi threads
	s.logger.Infof("Downloading small files, %d in total", len(smallFiles))
	downloadSmallFiles(smallFiles, s.casperDataStore, uri, destPath, s.logger)

	// Download large files in multipart way with multi threads
	s.logger.Infof("Downloading large files, %d in total", len(largeFiles))
	downloadLargeFilesWithMultiThreads(largeFiles, s.casperDataStore, uri, destPath, s.logger)

	return nil
}

func (s *Syncer) deleteModel(destPath string) error {
	return os.RemoveAll(destPath)
}

func downloadSmallFiles(files []objectstorage.ObjectSummary, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, logger *zap.SugaredLogger) {
	// prepare downloading for small files (setting up local target file folder)
	filesToDownload := prepareFilesToDownload(files, casperDataStore, originalUri, target)

	// Multithread downloading objects and saving to FSS
	downloadedFiles := casperDataStore.DownloadWithMultiThreads(DefaultSmallFilesDownloadThreads, logger, filesToDownload)
	for downloadedFile := range downloadedFiles {
		if downloadedFile.Err != nil {
			panic(downloadedFile.Err)
		}
	}
}

func downloadLargeFilesWithMultiThreads(objects []objectstorage.ObjectSummary, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, logger *zap.SugaredLogger) {
	// Multi-thread downloading objects and saving to NFS
	downloadedFiles := MultipartDownloadWithMultiThreads(objects, DefaultDownloadThreads, casperDataStore, originalUri, target, originalUri.Prefix, logger)
	for downloadedFile := range downloadedFiles {
		if downloadedFile.Err != nil {
			panic(downloadedFile.Err)
		}
	}
}

func prepareFilesToDownload(objects []objectstorage.ObjectSummary, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string) chan *casper.FileToDownload {
	filesToDownload := make(chan *casper.FileToDownload)
	go func() {
		defer func() {
			close(filesToDownload)
		}()

		for _, object := range objects {
			objectURI := casper.ObjectURI{
				Namespace:  originalUri.Namespace,
				BucketName: originalUri.BucketName,
				ObjectName: *object.Name,
			}

			fileToDownload, err := casperDataStore.PrepareDownload(objectURI, target, originalUri.Prefix)
			if err != nil {
				panic(err)
			}
			filesToDownload <- fileToDownload
		}
	}()

	return filesToDownload
}

func prepareObjectsChannel(objects []objectstorage.ObjectSummary) chan objectstorage.ObjectSummary {
	objectsChannel := make(chan objectstorage.ObjectSummary)
	go func() {
		defer func() {
			fmt.Println("close objects channel")
			close(objectsChannel)
		}()

		for _, object := range objects {
			objectsChannel <- object
		}
	}()

	return objectsChannel
}

func MultipartDownloadWithMultiThreads(objects []objectstorage.ObjectSummary, downloadThreads int, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, prefix string, logger *zap.SugaredLogger) chan *casper.DownloadedFile {
	logger.Infof("Download objects with %d threads", downloadThreads)
	objectChan := prepareObjectsChannel(objects)
	resultChan := make(chan *casper.DownloadedFile, len(objects))

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			defer wg.Done()
			multipartDownload(objectChan, resultChan, casperDataStore, originalUri, target, prefix, logger)
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

func multipartDownload(objects chan objectstorage.ObjectSummary, result chan *casper.DownloadedFile, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, prefix string, logger *zap.SugaredLogger) {
	for object := range objects {
		logger.Infof("Multipart download %s, size: %d", *object.Name, *(object.Size))
		objectURI := casper.ObjectURI{
			Namespace:  originalUri.Namespace,
			BucketName: originalUri.BucketName,
			ObjectName: *object.Name,
		}
		targetFilePath := filepath.Join(target, casper.ExtractNonPrefixObjectName(*object.Name, prefix))
		downloadedFile := casper.NewDownloadedFile(objectURI, targetFilePath)

		err := os.MkdirAll(path.Dir(targetFilePath), os.ModePerm)
		if err != nil {
			logger.Infof("Error creating file path for object %s: %+v", *object.Name, err)
			downloadedFile.Err = err
		}

		err = casperDataStore.MultipartDownload(objectURI, target, prefix, &object, DefaultDownloadChunkSizeInMB, DefaultMultipartDownloadThreads)
		if err != nil {
			logger.Errorf("Error in downloading object %s: %+v", *object.Name, err)
			downloadedFile.Err = err
		}

		result <- downloadedFile
	}
}