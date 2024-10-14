package generic_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 8
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	GB                           = 1073741824
)

// GenericDownloadAgent represents a Download Agent application
type GenericDownloadAgent struct {
	logger logging.Interface

	Config GenericConfig
}

type GenericReplicationResult struct {
	replicationSource casper.ObjectURI
	replicationTarget casper.ObjectURI
	error             error
}

// NewGenericDownloadAgent constructs a new model download agent from the given configuration.
func NewGenericDownloadAgent(config *GenericConfig) (*GenericDownloadAgent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("generic download agent configuration invalid: %v", err)
	}

	return &GenericDownloadAgent{
		logger: config.AnotherLogger,
		Config: *config,
	}, nil
}

// Start starts the application
func (d *GenericDownloadAgent) Start() {
	d.logger.Infof("Starting replication for %s", d.Config.ModelName)

	// 1. List all model weights in Partner bucket
	d.logger.Info("Start to download model weights from source")
	objects, err := d.Config.SourceCasperDataStore.ListObjects(*d.Config.SourceObjectStoreURI)
	if err != nil {
		panic(err)
	}
	d.logger.Infof("Done with listing all %d model weight objects under prefix %s", len(objects), d.Config.SourceObjectStoreURI.Prefix)

	// 2. Check if all files' size exceed the limit
	d.validateModelSize(objects)

	// 3. Replicate all objects to target using multi threads
	results := d.ReplicateWithMultiThreads(objects, d.Config.NumberOfThreadsForReplication)
	for result := range results {
		if result.error != nil {
			panic(result.error)
		}
	}
	d.logger.Infof("Replication succeeded for model %s with %d model weight objects", d.Config.ModelName, len(objects))
}

func (d *GenericDownloadAgent) ReplicateWithMultiThreads(objects []objectstorage.ObjectSummary, numOfThreads int) chan *GenericReplicationResult {
	d.logger.Infof("Start replication of objects with %d threads", numOfThreads)
	objectChan := d.prepareObjectsChannel(objects)
	resultChan := make(chan *GenericReplicationResult, len(objects))

	var wg sync.WaitGroup
	wg.Add(numOfThreads)

	for i := 0; i < numOfThreads; i++ {
		go func(i int) {
			defer wg.Done()
			d.replicateObjects(objectChan, resultChan)
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

func (d *GenericDownloadAgent) replicateObjects(objects <-chan objectstorage.ObjectSummary, results chan<- *GenericReplicationResult) {
	// Iterate over each object:
	//   1). Get from source object store
	//   2). Push to target object store
	count := 0
	for object := range objects {
		if *object.Name == d.Config.SourceObjectStoreURI.Prefix {
			continue
		}

		if count == 1 {
			d.logger.Infof("Done with first replication flow")
		}

		if count == len(objects)/2 {
			d.logger.Infof("Done with half replication flow")
		}

		// 1). Download object from source
		sourceObjectURI := casper.ObjectURI{
			Namespace:  d.Config.SourceObjectStoreURI.Namespace,
			BucketName: d.Config.SourceObjectStoreURI.BucketName,
			ObjectName: *object.Name,
		}
		result := GenericReplicationResult{
			replicationSource: sourceObjectURI,
		}

		err := d.Config.SourceCasperDataStore.MultipartDownload(sourceObjectURI, d.Config.TempModelStorePath, false, &object, DefaultDownloadChunkSizeInMB, DefaultDownloadThreads)
		if err != nil {
			d.logger.Errorf("Failed to download object %s from source object store: %+v", *object.Name, err)
			result.error = err
		}

		currentModelFilePath := filepath.Join(d.Config.TempModelStorePath, *object.Name)
		targetObjectName := strings.Replace(*object.Name, d.Config.SourceObjectStoreURI.Prefix, d.Config.TargetObjectStoreURI.Prefix, 1)

		// 2). Push object to target
		targetObjectURI := casper.ObjectURI{
			Namespace:  d.Config.TargetObjectStoreURI.Namespace,
			BucketName: d.Config.TargetObjectStoreURI.BucketName,
			ObjectName: targetObjectName,
		}
		result.replicationTarget = targetObjectURI

		if err = d.Config.TargetCasperDataStore.MultipartFileUpload(currentModelFilePath, targetObjectURI, DefaultUploadChunkSizeInMB, DefaultUploadThreads); err != nil {
			d.logger.Errorf("Failed to upload object %s to target object store: %+v", targetObjectName, err)
			result.error = err
		}
		count++

		d.logger.Info("One object replicated successfully")
		d.logger.Infof("sourceObjectNamespace: %s; sourceObjectBucket: %s; sourceObjectName: %s; sourceCompartment: %s", sourceObjectURI.Namespace, sourceObjectURI.BucketName, sourceObjectURI.ObjectName, *d.Config.SourceCasperDataStore.Config.CompartmentId)
		d.logger.Infof("targetObjectNamespace: %s; targetObjectBucket: %s; targetObjectName: %s; targetCompartment: %s", targetObjectURI.Namespace, targetObjectURI.BucketName, targetObjectURI.ObjectName, *d.Config.TargetCasperDataStore.Config.CompartmentId)

		results <- &result
	}
}

func (d *GenericDownloadAgent) prepareObjectsChannel(objects []objectstorage.ObjectSummary) chan objectstorage.ObjectSummary {
	objectsChan := make(chan objectstorage.ObjectSummary, len(objects))
	go func() {
		defer func() {
			close(objectsChan)
		}()
		for _, object := range objects {
			objectsChan <- object
		}
	}()
	return objectsChan
}

func (d *GenericDownloadAgent) validateModelSize(objects []objectstorage.ObjectSummary) {
	d.logger.Info("Start to calculate model size from source")

	sizeLimit := int64(d.Config.DownloadSizeLimitGB) * GB
	var allSize int64 = 0
	for _, object := range objects {
		if object.Name == nil {
			panic(fmt.Errorf("one of the name of the object from source is unknown"))
		}

		if object.Size == nil {
			panic(fmt.Errorf("one of the size of the object from source is unknown, object name: %s", *object.Name))
		}

		allSize += *object.Size

		if d.Config.EnableSizeLimitCheck {
			if allSize > sizeLimit {
				panic(fmt.Errorf("model weights files exceed the size limit. limit: %d bytes", sizeLimit))
			}
		}
	}
	if allSize == 0 {
		panic(fmt.Errorf("no model weights exist in the model folder"))
	}
}
