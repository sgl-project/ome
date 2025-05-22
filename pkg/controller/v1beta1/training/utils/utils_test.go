package utils

import "testing"

func TestExtractModelNameFromObjectStorageUriWithSubpath(t *testing.T) {
	uri := "oci://n/idlsnvn0f2is/b/model-store/o/meta/llama-3-3-70b-instruct"

	wanted := "llama-3-3-70b-instruct"
	actual := ExtractModelNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractModelNameFromObjectStorageUri(t *testing.T) {
	uri := "oci://n/idlsnvn0f2is/b/model-store/o/llama-3-3-70b-instruct"

	wanted := "llama-3-3-70b-instruct"
	actual := ExtractModelNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

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

func TestExtractObjectFileNameFromObjectStorageUriWithPathPrefix(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/data/sales_pitch_generation_train.jsonl"

	wanted := "data/sales_pitch_generation_train.jsonl"
	actual := ExtractObjectFileNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractBucketNameFromObjectStorageUriWithPathPrefix(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/data/sales_pitch_generation_train.jsonl"

	wanted := "beiwen-test"
	actual := ExtractBucketNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractNamespaceFromObjectStorageUriWithPathPrefix(t *testing.T) {
	uri := "oci://n/idqj093njucb/b/beiwen-test/o/data/sales_pitch_generation_train.jsonl"

	wanted := "idqj093njucb"
	actual := ExtractNamespaceFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractObjectFileNameFromObjectStorageHttpUriWithPathPrefix(t *testing.T) {
	uri := "https://objectstorage.us-chicago-1.oraclecloud.com/n/axk4z7krhqfx/b/beiwen_test/o/data%2Ftest%2Fsales_pitch_generation_train.jsonl"

	wanted := "data%2Ftest%2Fsales_pitch_generation_train.jsonl"
	actual := ExtractObjectFileNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractBucketNameFromObjectStorageHttpUriWithPathPrefix(t *testing.T) {
	uri := "https://objectstorage.us-chicago-1.oraclecloud.com/n/axk4z7krhqfx/b/beiwen_test/o/data%2Ftest%2Fsales_pitch_generation_train.jsonl"

	wanted := "beiwen_test"
	actual := ExtractBucketNameFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}

func TestExtractNamespaceFromObjectStorageHttpUriWithPathPrefix(t *testing.T) {
	uri := "https://objectstorage.us-chicago-1.oraclecloud.com/n/axk4z7krhqfx/b/beiwen_test/o/data%2Ftest%2Fsales_pitch_generation_train.jsonl"

	wanted := "axk4z7krhqfx"
	actual := ExtractNamespaceFromObjectStorageUri(uri)

	if actual != wanted {
		t.Errorf("wanted %s, got %s", wanted, actual)
	}
}
