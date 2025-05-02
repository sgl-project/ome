package modelagent

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils/storage"

	casper "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casperagent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	BigFileSizeInMB                  = 200
	DefaultDownloadChunkSizeInMB     = 200
	DefaultDownloadThreads           = 5
	DefaultMultipartDownloadThreads  = 5
	DefaultSmallFilesDownloadThreads = 10
	DefaultFilterFilesThreads        = 10
)

type GopherTaskType string

const (
	Download         GopherTaskType = "Download"
	DownloadOverride GopherTaskType = "DownloadOverride"
	Delete           GopherTaskType = "Delete"
)

type GopherTask struct {
	TaskType               GopherTaskType
	BaseModel              *v1beta1.BaseModel
	ClusterBaseModel       *v1beta1.ClusterBaseModel
	TensorRTLLMShapeFilter *TensorRTLLMShapeFilter
}

type Gopher struct {
	downloadRetry      int
	modelRootDir       string
	modelRootDirOnHost string
	casperDataStore    casper.CasperDataStore
	gopherChan         <-chan *GopherTask
	nodeLabeler        *NodeLabeler
	metrics            *Metrics
	logger             *zap.SugaredLogger
}

func NewGopher(authType string,
	downloadRetry int,
	modelRootDir string,
	modelRootDirOnHost string,
	gopherChan <-chan *GopherTask,
	nodeLabeler *NodeLabeler,
	metrics *Metrics,
	logger *zap.SugaredLogger) (*Gopher, error) {
	casperDataStore, err := NewCasperDataStore(authType)
	if err != nil {
		logger.Errorf("Not able to initalize the casper data store: %s", err.Error())
		return nil, err
	}

	return &Gopher{
		downloadRetry:      downloadRetry,
		modelRootDir:       modelRootDir,
		modelRootDirOnHost: modelRootDirOnHost,
		casperDataStore:    casperDataStore,
		gopherChan:         gopherChan,
		nodeLabeler:        nodeLabeler,
		metrics:            metrics,
		logger:             logger,
	}, nil
}

func (s *Gopher) Run(stopCh <-chan struct{}, numWorker int) {
	s.logger.Info("Starting gopher workers")

	for i := 0; i < numWorker; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	s.logger.Info("Started gopher workers")
	<-stopCh
	s.logger.Info("Shutting down gopher workers")
}

func (s *Gopher) runWorker() {
	for {
		select {
		case task, ok := <-s.gopherChan:
			if ok {
				err := s.processTask(task)
				if err != nil {
					s.logger.Errorf("Gopher task failed with error: %s", err.Error())
				}
			} else {
				s.logger.Info("gopher channel closed, worker exits.")
				return
			}
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *Gopher) processTask(task *GopherTask) error {
	if task.BaseModel == nil && task.ClusterBaseModel == nil {
		return fmt.Errorf("gopher got empty task")
	}

	// Get model info for logging
	modelInfo := getModelInfoForLogging(task)
	s.logger.Infof("Processing gopher task: %s, type: %s", modelInfo, task.TaskType)

	// Get model type, namespace, and name for metrics
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	casperUri, destPath, err := getTargetDirPath(task.BaseModel, task.ClusterBaseModel, s.modelRootDir, s.modelRootDirOnHost)
	if err != nil {
		s.logger.Errorf("Failed to get target directory path for model %s: %v", modelInfo, err)

		// Record failed download in metrics
		if task.TaskType == Download || task.TaskType == DownloadOverride {
			s.metrics.RecordFailedDownload(modelType, namespace, name, "target_path_error")
		}

		s.markModelOnNodeFailed(task)
		return err
	}

	switch task.TaskType {
	case Download:
		// we might implement a "delete/cleanup and then download" logic to update a model in the future
		// use a single download function for now
		fallthrough
	case DownloadOverride:
		s.logger.Infof("Starting download for model %s", modelInfo)

		// Record time for metrics
		downloadStartTime := time.Now()

		err := utils.Retry(s.downloadRetry, 100*time.Millisecond, func() error {
			downloadErr := s.downloadModel(casperUri, destPath, task.TensorRTLLMShapeFilter, task)
			if downloadErr != nil {
				s.logger.Errorf("Failed to download model %s (attempt %d/%d): %v",
					modelInfo, s.downloadRetry, s.downloadRetry, downloadErr)
			}
			return downloadErr
		})

		// Calculate download duration
		downloadDuration := time.Since(downloadStartTime)

		if err != nil {
			s.logger.Errorf("All download attempts failed for model %s: %v", modelInfo, err)

			// Record download failure in metrics
			errorType := "download_error"
			if strings.Contains(err.Error(), "MD5") {
				errorType = "md5_verification_error"
			}
			s.metrics.RecordFailedDownload(modelType, namespace, name, errorType)

			s.markModelOnNodeFailed(task)
			return err
		}

		// Record successful download in metrics
		s.metrics.RecordSuccessfulDownload(modelType, namespace, name)
		s.metrics.ObserveDownloadDuration(modelType, namespace, name, downloadDuration)

		if task.BaseModel != nil {
			s.logger.Infof("Successfully downloaded BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
		} else {
			s.logger.Infof("Successfully downloaded ClusterBaseModel %s", task.ClusterBaseModel.Name)
		}

		// mark model as Ready
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Ready,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		err = s.nodeLabeler.processOp(nodeLabelOp)
		if err != nil {
			s.logger.Errorf("Failed to mark model %s as Ready: %v", modelInfo, err)
			return err
		}
	case Delete:
		s.logger.Infof("Starting deletion for model %s", modelInfo)
		err := s.deleteModel(destPath)
		if err != nil {
			s.logger.Errorf("Failed to delete model %s: %v", modelInfo, err)
			return err
		}
		if task.BaseModel != nil {
			s.logger.Infof("Successfully deleted the BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
		} else {
			s.logger.Infof("Successfully deleted the ClusterBaseModel %s", task.ClusterBaseModel.Name)
		}
	}

	return nil
}

func getModelInfoForLogging(task *GopherTask) string {
	if task.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", task.BaseModel.Namespace, task.BaseModel.Name)
	} else if task.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", task.ClusterBaseModel.Name)
	}
	return "unknown model"
}

func (s *Gopher) markModelOnNodeFailed(task *GopherTask) {
	modelInfo := getModelInfoForLogging(task)
	s.logger.Infof("Marking model %s as Failed on node", modelInfo)

	nodeLabelOp := &NodeLabelOp{
		ModelStateOnNode: Failed,
		BaseModel:        task.BaseModel,
		ClusterBaseModel: task.ClusterBaseModel,
	}

	err := s.nodeLabeler.processOp(nodeLabelOp)
	if err != nil {
		s.logger.Errorf("Failed to mark model %s as Failed on node: %v", modelInfo, err)
	} else {
		s.logger.Infof("Successfully marked model %s as Failed on node", modelInfo)
	}
}

// getStorageInfo extracts storage URI and path from BaseModel or ClusterBaseModel
func getStorageInfo(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) (storagePath, destPath string, err error) {
	if baseModel != nil {
		if baseModel.Spec.Storage.StorageUri == nil {
			return "", "", fmt.Errorf("got empty storage uri in baseModel %s in namespace %s", baseModel.Name, baseModel.Namespace)
		}
		storagePath = *baseModel.Spec.Storage.StorageUri
		if baseModel.Spec.Storage.Path != nil {
			destPath = *baseModel.Spec.Storage.Path
		}
		return
	}

	if clusterBaseModel.Spec.Storage.StorageUri == nil {
		return "", "", fmt.Errorf("got empty storage uri in clusterBaseModel %s", clusterBaseModel.Name)
	}
	storagePath = *clusterBaseModel.Spec.Storage.StorageUri
	if clusterBaseModel.Spec.Storage.Path != nil {
		destPath = *clusterBaseModel.Spec.Storage.Path
	}
	return
}

// validateAndTransformDestPath ensures destPath is under modelRootDir and transforms it if needed
func validateAndTransformDestPath(destPath, modelRootDir, modelRootDirOnHost string) (string, error) {
	if len(destPath) == 0 {
		return "", nil // empty path is valid, caller should handle default path
	}

	if !strings.HasPrefix(destPath, modelRootDirOnHost) {
		return "", fmt.Errorf("user defined destination path {%s} is not under model root dir {%s} of the host", destPath, modelRootDir)
	}

	return strings.Replace(destPath, modelRootDirOnHost, modelRootDir, 1), nil
}

// handleVendorStorage processes vendor storage URIs
func handleVendorStorage(vendorComponents *storage.VendorStorageComponents, destPath, modelRootDir string) (*casper.ObjectURI, string) {
	osUri := &casper.ObjectURI{
		Namespace:  vendorComponents.VendorName,
		BucketName: vendorComponents.ResourceType,
		Prefix:     vendorComponents.ResourcePath,
		IsVendor:   true,
	}

	if len(destPath) == 0 {
		destPath = path.Join(modelRootDir, vendorComponents.VendorName, vendorComponents.ResourceType, vendorComponents.ResourcePath)
	}

	return osUri, destPath
}

// handleObjectStorage processes object storage URIs
func handleObjectStorage(osUri *casper.ObjectURI, destPath, modelRootDir string) (string, error) {
	if !strings.HasSuffix(osUri.Prefix, "/") {
		osUri.Prefix = osUri.Prefix + "/"
	}

	if len(destPath) == 0 {
		if strings.HasSuffix(modelRootDir, "/") {
			destPath = modelRootDir + osUri.Prefix
		} else {
			destPath = modelRootDir + "/" + osUri.Prefix
		}
	}

	return destPath, nil
}

// getTargetDirPath determines the target directory path for a model based on its storage configuration
func getTargetDirPath(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel, modelRootDir string, modelRootDirOnHost string) (*casper.ObjectURI, string, error) {
	// Get storage URI and path from model
	storagePath, destPath, err := getStorageInfo(baseModel, clusterBaseModel)
	if err != nil {
		return nil, "", err
	}

	// Validate and transform destination path if provided
	if len(destPath) > 0 {
		destPath, err = validateAndTransformDestPath(destPath, modelRootDir, modelRootDirOnHost)
		if err != nil {
			return nil, "", err
		}
	}

	// Determine storage type and handle accordingly
	storageType, err := storage.GetStorageType(storagePath)
	if err != nil {
		return nil, "", err
	}

	switch storageType {
	case storage.StorageTypeVendor:
		vendorComponents, err := storage.ParseVendorStorageURI(storagePath)
		if err != nil {
			return nil, "", err
		}
		osUri, destPath := handleVendorStorage(vendorComponents, destPath, modelRootDir)
		return osUri, destPath, nil

	case storage.StorageTypeOCI:
		osUri, err := NewObjectStorageUri(storagePath)
		if err != nil {
			return nil, "", err
		}
		destPath, err = handleObjectStorage(osUri, destPath, modelRootDir)
		if err != nil {
			return nil, "", err
		}
		return osUri, destPath, nil

	default:
		return nil, "", fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

func (s *Gopher) downloadModel(uri *casper.ObjectURI, destPath string, shapeFilter *TensorRTLLMShapeFilter, task *GopherTask) error {
	// If this is a vendor storage URI, skip download operations
	if uri.IsVendor {
		s.logger.Infof("Vendor storage URI detected, skipping download operations for %s/%s/%s", uri.Namespace, uri.BucketName, uri.Prefix)
		return nil
	}

	startTime := time.Now()
	defer func() {
		// Record download duration regardless of success/failure
		s.logger.Infof("Download process took %v", time.Since(startTime).Round(time.Millisecond))
	}()

	s.logger.Infof("Making call to object storage with endpoint %s", s.casperDataStore.CasperClient.Endpoint())
	objects, err := s.casperDataStore.ListObjects(*uri)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	s.logger.Infof("Done with list all %d objects in model bucket folder", len(objects))

	if shapeFilter.IsTensorrtLLMModel && shapeFilter.ModelType == string(constants.ServingBaseModel) {
		s.logger.Infof("TensorRTLLM Serving model detected. Start filtering model files that doesn't belong to the node shape %s in model bucket folder", shapeFilter.ShapeAlias)
		shapeFilteredObjects := make([]objectstorage.ObjectSummary, 0)
		for _, object := range objects {
			if object.Name != nil {
				if strings.Contains(*object.Name, fmt.Sprintf("/%s/", shapeFilter.ShapeAlias)) {
					shapeFilteredObjects = append(shapeFilteredObjects, object)
				}
			}
		}
		objects = shapeFilteredObjects

		if len(objects) == 0 {
			return fmt.Errorf("no suitable objects found for shape %s", shapeFilter.ShapeAlias)
		}
		s.logger.Infof("Found %d objects applicable for shape %s", len(objects), shapeFilter.ShapeAlias)
	}

	objectsChannel := prepareObjectsChannel(objects)

	s.logger.Info("Start to filter objects...")
	filteredObjects := s.casperDataStore.FilterObjectsMultiThreads(DefaultFilterFilesThreads, s.logger, uri, destPath, objectsChannel, uri.Prefix)

	// 4. Split files per size into two groups
	smallFiles := make([]objectstorage.ObjectSummary, 0)
	largeFiles := make([]objectstorage.ObjectSummary, 0)
	var totalFiles int
	var totalBytes int64
	for object := range filteredObjects {
		totalFiles++
		if object.Size != nil {
			totalBytes += *object.Size
		}
		if object.Size == nil || *object.Size < int64(BigFileSizeInMB)*int64(casper.MB) {
			smallFiles = append(smallFiles, object)
		} else {
			largeFiles = append(largeFiles, object)
		}
	}

	if totalFiles == 0 {
		s.logger.Info("No files need to be downloaded or updated (all files exist with matching MD5 checksums)")
		return nil
	}

	// Download small files with multi threads
	if len(smallFiles) > 0 {
		s.logger.Infof("Downloading small files, %d in total", len(smallFiles))
		smallFilesErrors := downloadSmallFiles(smallFiles, s.casperDataStore, uri, destPath, s.logger)
		if len(smallFilesErrors) > 0 {
			errMsgs := make([]string, 0, len(smallFilesErrors))
			for file, err := range smallFilesErrors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", file, err))
			}
			return fmt.Errorf("errors downloading small files: %s", strings.Join(errMsgs, "; "))
		}
	}

	// Download large files in a multipart way with multi threads
	if len(largeFiles) > 0 {
		s.logger.Infof("Downloading large files, %d in total", len(largeFiles))
		largeFilesErrors := downloadLargeFilesWithMultiThreads(largeFiles, s.casperDataStore, uri, destPath, s.logger)
		if len(largeFilesErrors) > 0 {
			errMsgs := make([]string, 0, len(largeFilesErrors))
			for file, err := range largeFilesErrors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", file, err))
			}
			return fmt.Errorf("errors downloading large files: %s", strings.Join(errMsgs, "; "))
		}
	}

	// Perform final verification of all downloaded files
	s.logger.Info("Performing final integrity verification of all downloaded files...")
	verificationStartTime := time.Now()
	verificationErrors := s.verifyDownloadedFiles(objects, uri, destPath, task)
	verificationDuration := time.Since(verificationStartTime)

	// Record verification duration
	s.metrics.ObserveVerificationDuration(verificationDuration)

	if len(verificationErrors) > 0 {
		s.logger.Errorf("Final verification failed for %d files", len(verificationErrors))
		errMsgs := make([]string, 0, len(verificationErrors))
		for file, err := range verificationErrors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", file, err))
			s.logger.Errorf("Verification failed for %s: %v", file, err)
		}
		return fmt.Errorf("integrity verification failed for %d/%d files: %s",
			len(verificationErrors), len(objects), strings.Join(errMsgs, "; "))
	}

	// Record bytes transferred - only accurate for new/updated files
	if totalBytes > 0 {
		s.logger.Infof("Downloaded a total of %d bytes (%d MB)", totalBytes, totalBytes/int64(1024*1024))
	}

	s.logger.Infof("All files downloaded and verified successfully (%d files, verification took %v)",
		len(objects), verificationDuration.Round(time.Millisecond))
	return nil
}

func (s *Gopher) verifyDownloadedFiles(objects []objectstorage.ObjectSummary, uri *casper.ObjectURI, destPath string, task *GopherTask) map[string]error {
	errors := make(map[string]error)
	totalFiles := len(objects)
	verifiedCount := 0

	s.logger.Infof("Starting verification of %d files", totalFiles)

	// Get proper model information for metrics from the task
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	for _, object := range objects {
		if object.Name == nil || object.Md5 == nil {
			s.logger.Warnf("Skipping verification for object without name or MD5")
			continue // Skip objects without a name or MD5
		}

		objectPath := *object.Name
		filePath := filepath.Join(destPath, casper.ExtractNonPrefixObjectName(objectPath, uri.Prefix))

		// Check if the file exists
		_, err := os.Stat(filePath)
		if err != nil {
			errors[objectPath] = fmt.Errorf("file does not exist: %w", err)
			s.metrics.RecordVerification(modelType, namespace, name, false)
			continue
		}

		// Verify MD5
		match, err := s.casperDataStore.VerifyFileMd5(filePath, object.Md5, s.logger)
		if err != nil {
			errors[objectPath] = fmt.Errorf("MD5 verification error: %w", err)
			s.metrics.RecordVerification(modelType, namespace, name, false)
			continue
		}

		if !match {
			errors[objectPath] = fmt.Errorf("MD5 mismatch detected in final verification")
			s.metrics.RecordVerification(modelType, namespace, name, false)
		} else {
			verifiedCount++
			s.metrics.RecordVerification(modelType, namespace, name, true)
			if verifiedCount%10 == 0 || verifiedCount == totalFiles {
				s.logger.Infof("Verified %d/%d files", verifiedCount, totalFiles)
			}
		}
	}

	if len(errors) == 0 {
		s.logger.Infof("All %d files successfully verified", totalFiles)
	} else {
		s.logger.Warnf("%d/%d files failed verification", len(errors), totalFiles)
	}

	return errors
}

func (s *Gopher) deleteModel(destPath string) error {
	return os.RemoveAll(destPath)
}

func downloadSmallFiles(files []objectstorage.ObjectSummary, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, logger *zap.SugaredLogger) map[string]error {
	// prepare downloading for small files (setting up local target file folder)
	filesToDownload := prepareFilesToDownload(files, casperDataStore, originalUri, target)

	// Multi-thread downloading all small files (in memory)
	downloadedFiles := casperDataStore.DownloadWithMultiThreads(DefaultSmallFilesDownloadThreads, logger, filesToDownload)
	errors := make(map[string]error)

	// Create a map of objects by name for easier lookup
	fileMap := make(map[string]objectstorage.ObjectSummary)
	for _, file := range files {
		if file.Name != nil {
			fileMap[*file.Name] = file
		}
	}

	// Process downloaded files
	fileNum := 1
	for downloadedFile := range downloadedFiles {
		if downloadedFile.Err != nil {
			// Since we can't access the unexported fields directly, just use a generic name with an index
			errorKey := fmt.Sprintf("file-%d", fileNum)
			errors[errorKey] = downloadedFile.Err
			fileNum++
		}
	}
	return errors
}

func downloadLargeFilesWithMultiThreads(objects []objectstorage.ObjectSummary, casperDataStore casper.CasperDataStore, originalUri *casper.ObjectURI, target string, logger *zap.SugaredLogger) map[string]error {
	// Multi-thread downloading objects and saving to NFS
	downloadedFiles := MultipartDownloadWithMultiThreads(objects, DefaultDownloadThreads, casperDataStore, originalUri, target, originalUri.Prefix, logger)
	errors := make(map[string]error)

	// Process downloaded files
	fileNum := 1
	for downloadedFile := range downloadedFiles {
		if downloadedFile.Err != nil {
			// Since we can't access the unexported fields directly, just use a generic name with an index
			errorKey := fmt.Sprintf("file-%d", fileNum)
			errors[errorKey] = downloadedFile.Err
			fileNum++
		}
	}
	return errors
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
