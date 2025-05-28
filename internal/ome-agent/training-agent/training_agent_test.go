package training_agent

import (
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociobjectstore"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

func TestNewTrainingAgent(t *testing.T) {
	var config Config

	config.Runtime = "peft"
	config.TrainingName = "tname"
	config.ModelDirectory = "model-dir"
	config.ZippedModelPath = "zipped-model-path"
	config.ZippedMergedModelPath = "zipped-merged-model-path"
	config.TrainingDataStoreDirectory = "training-data-dir"
	config.TrainingDataObjectStoreURI = &ociobjectstore.ObjectURI{
		BucketName: "bucket-name",
	}
	config.ModelObjectStoreURI = &ociobjectstore.ObjectURI{
		BucketName: "bucket-name",
	}
	config.TrainingMetricsObjectStoreURI = &ociobjectstore.ObjectURI{
		BucketName: "bucket-name",
	}
	config.InputObjectStorageDataStore = &ociobjectstore.OCIOSDataStore{
		Client: &objectstorage.ObjectStorageClient{},
	}
	config.OutputObjectStorageDataStore = &ociobjectstore.OCIOSDataStore{
		Client: &objectstorage.ObjectStorageClient{},
	}
	config.PeftFineTuneDetails = &PeftFineTuneDetails{}
	agent, err := NewTrainingAgent(&config)

	if err != nil {
		t.Errorf("errro: %v", err)
	}

	if agent == nil {
		t.Errorf("agent was nil: %v", agent)
	}
}
