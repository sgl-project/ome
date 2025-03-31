package utils

import "testing"

func TestExtractObjectFileNameFromObjectStorageUri(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/sales_pitch_generation_train.jsonl"

	wanted := "sales_pitch_generation_train.jsonl"
	actual := ExtractObjectFileNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractBucketNameFromObjectStorageUri(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/sales_pitch_generation_train.jsonl"

	wanted := "beiwen-test"
	actual := ExtractBucketNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractNamespaceFromObjectStorageUrii(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/sales_pitch_generation_train.jsonl"

	wanted := "idqj093njucb"
	actual := ExtractNamespaceFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}
