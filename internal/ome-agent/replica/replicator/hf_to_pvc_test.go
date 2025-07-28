package replicator

import (
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"testing"
)

func TestHFToPVCReplicator_Replicate_Success(t *testing.T) {
	// Save the original function so we can restore it later
	originalDownloadFunc := downloadFromHFFunc

	// Defer restoring original function
	defer func() {
		downloadFromHFFunc = originalDownloadFunc
	}()

	// Replace downloadFromHFFunc with a mock version
	downloadFromHFFunc = func(input common.ReplicationInput, client *hub.HubClient, path string, logger logging.Interface) (string, error) {
		if path != "/mnt/data/meta/lama-Guard-4-12B" {
			t.Errorf("unexpected path: got %s, want /mnt/data/meta/lama-Guard-4-12B", path)
		}
		return "/mock/downloaded_path", nil
	}

	replicator := &HFToPVCReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: HFToPVCReplicatorConfig{
			LocalPath: "/mnt/data/",
			HubClient: &hub.HubClient{},
		},
		ReplicationInput: common.ReplicationInput{
			Source: ociobjectstore.ObjectURI{
				Namespace:  "huggingface",
				BucketName: "meta-llama/Llama-Guard-4-12B",
				Prefix:     "main",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "amaaaaaax7756raaolxvbyk7toite23tbfkarxhiipv6jdy3tgwjjq4l6zma",
				BucketName: "model-pvc",
				Prefix:     "meta/lama-Guard-4-12B",
			},
		},
	}

	err := replicator.Replicate([]common.ReplicationObject{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHFToPVCReplicator_Replicate_Failure(t *testing.T) {
	originalDownloadFunc := downloadFromHFFunc
	defer func() {
		downloadFromHFFunc = originalDownloadFunc
	}()

	downloadFromHFFunc = func(input common.ReplicationInput, client *hub.HubClient, path string, logger logging.Interface) (string, error) {
		return "", fmt.Errorf("mock error")
	}

	replicator := &HFToPVCReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: HFToPVCReplicatorConfig{
			LocalPath: "/tmp",
			HubClient: &hub.HubClient{},
		},
		ReplicationInput: common.ReplicationInput{
			Source: ociobjectstore.ObjectURI{
				Namespace:  "huggingface",
				BucketName: "fake-hf-model",
				Prefix:     "main",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "amaaaaaax7756raaolxvbyk7toite23tbfkarxhiipv6jdy3tgwjjq4l6zma",
				BucketName: "model-pvc",
				Prefix:     "models",
			},
		},
	}

	err := replicator.Replicate([]common.ReplicationObject{})
	if err == nil {
		t.Error("expected error, got nil")
	}
}
