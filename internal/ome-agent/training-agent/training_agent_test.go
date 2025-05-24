package training_agent

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/sgl-ome/pkg/casper"
)

func TestNewTrainingAgent(t *testing.T) {
	var config Config

	config.Runtime = "peft"
	config.TrainingName = "tname"
	config.ModelDirectory = "model-dir"
	config.ZippedModelPath = "zipped-model-path"
	config.ZippedMergedModelPath = "zipped-merged-model-path"
	config.TrainingDataStoreDirectory = "training-data-dir"
	config.TrainingDataObjectStoreURI = &casper.ObjectURI{
		BucketName: "bucket-name",
	}
	config.ModelObjectStoreURI = &casper.ObjectURI{
		BucketName: "bucket-name",
	}
	config.TrainingMetricsObjectStoreURI = &casper.ObjectURI{
		BucketName: "bucket-name",
	}
	config.InputObjectStorageDataStore = &casper.CasperDataStore{
		Client: &objectstorage.ObjectStorageClient{},
	}
	config.OutputObjectStorageDataStore = &casper.CasperDataStore{
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
