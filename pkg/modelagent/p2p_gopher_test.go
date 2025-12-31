package modelagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeModelHash(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		revision string
	}{
		{
			name:     "model without revision",
			modelID:  "meta-llama/Llama-2-7b-hf",
			revision: "",
		},
		{
			name:     "model with revision",
			modelID:  "meta-llama/Llama-2-7b-hf",
			revision: "main",
		},
		{
			name:     "model with specific commit",
			modelID:  "meta-llama/Llama-2-7b-hf",
			revision: "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := computeModelHash(tt.modelID, tt.revision)

			// Hash should be 64 characters (SHA256 hex encoded)
			assert.Equal(t, 64, len(hash), "hash should be 64 characters")

			// Same input should produce same hash
			hash2 := computeModelHash(tt.modelID, tt.revision)
			assert.Equal(t, hash, hash2, "same input should produce same hash")
		})
	}
}

func TestComputeModelHashDifferentInputs(t *testing.T) {
	// Different model IDs should produce different hashes
	hash1 := computeModelHash("meta-llama/Llama-2-7b-hf", "")
	hash2 := computeModelHash("meta-llama/Llama-2-13b-hf", "")
	assert.NotEqual(t, hash1, hash2, "different models should have different hashes")

	// Same model with different revisions should produce different hashes
	hash3 := computeModelHash("meta-llama/Llama-2-7b-hf", "main")
	hash4 := computeModelHash("meta-llama/Llama-2-7b-hf", "dev")
	assert.NotEqual(t, hash3, hash4, "different revisions should have different hashes")

	// Model without revision should differ from model with revision
	hash5 := computeModelHash("meta-llama/Llama-2-7b-hf", "")
	hash6 := computeModelHash("meta-llama/Llama-2-7b-hf", "main")
	assert.NotEqual(t, hash5, hash6, "no revision vs with revision should differ")
}

func TestComputeModelHashConsistency(t *testing.T) {
	// Test that hashes are consistent across multiple calls
	modelID := "nvidia/Llama-3.1-Nemotron-70B-Instruct-HF"
	revision := "v1.0.0"

	hashes := make([]string, 100)
	for i := 0; i < 100; i++ {
		hashes[i] = computeModelHash(modelID, revision)
	}

	// All hashes should be identical
	for i := 1; i < 100; i++ {
		assert.Equal(t, hashes[0], hashes[i], "all hashes should be identical")
	}
}

func TestComputeModelHashSpecialCharacters(t *testing.T) {
	// Test with special characters in model names
	tests := []struct {
		name     string
		modelID  string
		revision string
	}{
		{
			name:     "with underscores",
			modelID:  "org_name/model_name_v2",
			revision: "branch_name",
		},
		{
			name:     "with dots",
			modelID:  "org.name/model.name.v2",
			revision: "v1.2.3",
		},
		{
			name:     "with hyphens and numbers",
			modelID:  "org-123/model-456-v2",
			revision: "commit-abc-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := computeModelHash(tt.modelID, tt.revision)
			assert.Equal(t, 64, len(hash), "hash should be 64 characters")

			// Hash should only contain hex characters
			for _, c := range hash {
				assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
					"hash should only contain hex characters")
			}
		})
	}
}

// TestGopherP2PConfiguration tests the P2P configuration methods
func TestGopherP2PConfiguration(t *testing.T) {
	t.Run("EnableP2P sets fields correctly", func(t *testing.T) {
		g := &Gopher{}

		assert.False(t, g.p2pEnabled)
		assert.Nil(t, g.p2pDistributor)
		assert.Nil(t, g.p2pLeaseManager)

		// Note: We can't test EnableP2P without creating actual distributor/lease manager
		// which would require more setup. This is a limitation of unit testing.
	})
}

// Edge case tests for P2P flow
func TestP2PEdgeCases(t *testing.T) {
	t.Run("empty model ID hash", func(t *testing.T) {
		hash := computeModelHash("", "")
		assert.Equal(t, 64, len(hash), "empty input should still produce valid hash")
	})

	t.Run("very long model ID", func(t *testing.T) {
		longModelID := "organization-with-very-long-name/model-name-that-is-also-very-long-for-testing-purposes-v123456789"
		hash := computeModelHash(longModelID, "main")
		assert.Equal(t, 64, len(hash), "long input should still produce 64-char hash")
	})

	t.Run("unicode in model ID", func(t *testing.T) {
		// While unlikely, test unicode handling
		hash := computeModelHash("org/模型-v1", "main")
		assert.Equal(t, 64, len(hash), "unicode should be handled correctly")
	})
}
