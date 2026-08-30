package storage

import "testing"

func TestParseOCIStorageURIParsesAModelPackReference(t *testing.T) {
	got, err := ParseOCIStorageURI("oci://ghcr.io/org/model:tag")
	if err != nil {
		t.Fatal(err)
	}
	// The whole remainder is the reference: a repository name may contain
	// slashes, so there is nothing further to split.
	if got.Reference != "ghcr.io/org/model:tag" {
		t.Fatalf("got %q", got.Reference)
	}

	got, err = ParseOCIStorageURI("oci://registry.example.com:5000/a/b/c@sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Reference != "registry.example.com:5000/a/b/c@sha256:abc" {
		t.Fatalf("got %q", got.Reference)
	}
}

func TestParseOCIStorageURIRejectsBadInput(t *testing.T) {
	for _, uri := range []string{
		"ghcr.io/org/model:tag", // no scheme
		"oci://",                // empty reference
		"oci://   ",
		"oci://model", // no registry component
	} {
		if _, err := ParseOCIStorageURI(uri); err == nil {
			t.Errorf("expected an error for %q", uri)
		}
	}
}

func TestParseOCIStorageURIGivesAMigrationHintForObjectStorage(t *testing.T) {
	// The most likely mistake after this change: an old Oracle URI.
	_, err := ParseOCIStorageURI("oci://n/mynamespace/b/mybucket/o/models/llama")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !contains(got, "Oracle Object Storage") || !contains(got, "ModelPack") {
		t.Fatalf("error should explain the migration, got: %s", got)
	}
}

func TestObjectStorageMovedToItsOwnScheme(t *testing.T) {
	got, err := ParseOCIObjectStoreURI("ocios://n/mynamespace/b/mybucket/o/models/llama")
	if err != nil {
		t.Fatal(err)
	}
	if got.Namespace != "mynamespace" || got.Bucket != "mybucket" || got.Prefix != "models/llama" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetStorageTypeDistinguishesTheTwoSchemes(t *testing.T) {
	cases := map[string]StorageType{
		"oci://ghcr.io/org/model:tag":  StorageTypeOCI,
		"ocios://n/ns/b/bucket/o/path": StorageTypeOCIObjectStore,
		"hf://org/model":               StorageTypeHuggingFace,
		"s3://bucket/key":              StorageTypeS3,
		"pvc://my-pvc/sub":             StorageTypePVC,
	}
	for uri, want := range cases {
		got, err := GetStorageType(uri)
		if err != nil {
			t.Errorf("%s: %v", uri, err)
			continue
		}
		if got != want {
			t.Errorf("GetStorageType(%q) = %q, want %q", uri, got, want)
		}
	}
}

func TestValidateStorageURICoversBothSchemes(t *testing.T) {
	if err := ValidateStorageURI("oci://ghcr.io/org/model:tag"); err != nil {
		t.Errorf("ModelPack URI should validate: %v", err)
	}
	if err := ValidateStorageURI("ocios://n/ns/b/bucket/o/path"); err != nil {
		t.Errorf("Object Storage URI should validate: %v", err)
	}
	if err := ValidateStorageURI("oci://n/ns/b/bucket/o/path"); err == nil {
		t.Error("an Oracle URI under oci:// should now fail")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
