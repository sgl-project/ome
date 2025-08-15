package replicator

import (
	"os"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPrepareObjectChannel(t *testing.T) {
	objName1 := "test1.bin"
	objName2 := "test2.bin"

	objects := []common.ReplicationObject{
		common.ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName1,
			},
		},
		common.ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName2,
			},
		},
	}

	objChan := PrepareObjectChannel(objects)

	// Collect objects from channel
	var receivedObjects []common.ReplicationObject
	for obj := range objChan {
		receivedObjects = append(receivedObjects, obj)
	}

	assert.Equal(t, len(objects), len(receivedObjects))
	assert.Equal(t, objects[0].GetName(), receivedObjects[0].GetName())
	assert.Equal(t, objects[1].GetName(), receivedObjects[1].GetName())
}

func TestLogProgress(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()

	startTime := time.Now().Add(-10 * time.Second)
	LogProgress(5, 1, 10, startTime, mockLogger)

	// Verify the logger was called with the expected info
	mockLogger.AssertCalled(t, "Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGetFileChecksum(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test_checksum_*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write test content to the file
	testContent := "Hello, World! This is a test file for checksum calculation."
	_, err = tempFile.WriteString(testContent)
	assert.NoError(t, err)
	tempFile.Close()

	tests := []struct {
		name        string
		filePath    string
		algorithm   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "MD5 checksum calculation",
			filePath:    tempFile.Name(),
			algorithm:   MD5ChecksumAlgorithm,
			expectError: false,
		},
		{
			name:        "SHA256 checksum calculation",
			filePath:    tempFile.Name(),
			algorithm:   SHA256ChecksumAlgorithm,
			expectError: false,
		},
		{
			name:        "unsupported algorithm",
			filePath:    tempFile.Name(),
			algorithm:   "unsupported",
			expectError: true,
			errorMsg:    "unsupported checksum algorithm: unsupported",
		},
		{
			name:        "non-existent file",
			filePath:    "/non/existent/file.txt",
			algorithm:   MD5ChecksumAlgorithm,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum, err := GetFileChecksum(tt.filePath, tt.algorithm)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Empty(t, checksum)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, checksum)
				// Verify the checksum is base64 encoded
				if tt.algorithm == MD5ChecksumAlgorithm {
					assert.Len(t, checksum, 24) // MD5 base64 length
				} else if tt.algorithm == SHA256ChecksumAlgorithm {
					assert.Len(t, checksum, 44) // SHA256 base64 length
				}
			}
		})
	}
}

func TestGetObjectMetadatWithFileChecksum(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test_metadata_*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write test content to the file
	testContent := "Hello, World! This is a test file for metadata generation."
	_, err = tempFile.WriteString(testContent)
	assert.NoError(t, err)
	tempFile.Close()

	tests := []struct {
		name           string
		config         *common.ChecksumConfig
		filePath       string
		expectedResult map[string]string
		expectWarning  bool
	}{
		{
			name: "MD5 checksum with upload enabled",
			config: &common.ChecksumConfig{
				UploadEnabled:     true,
				ChecksumAlgorithm: MD5ChecksumAlgorithm,
			},
			filePath: tempFile.Name(),
			expectedResult: map[string]string{
				OCIObjectMD5MetadataKey: mock.Anything,
			},
			expectWarning: false,
		},
		{
			name: "SHA256 checksum with upload enabled",
			config: &common.ChecksumConfig{
				UploadEnabled:     true,
				ChecksumAlgorithm: SHA256ChecksumAlgorithm,
			},
			filePath: tempFile.Name(),
			expectedResult: map[string]string{
				OCIObjectSHA256MetadataKey: mock.Anything,
			},
			expectWarning: false,
		},
		{
			name: "upload disabled",
			config: &common.ChecksumConfig{
				UploadEnabled:     false,
				ChecksumAlgorithm: MD5ChecksumAlgorithm,
			},
			filePath:       tempFile.Name(),
			expectedResult: nil,
			expectWarning:  false,
		},
		{
			name:           "nil config",
			config:         nil,
			filePath:       tempFile.Name(),
			expectedResult: nil,
			expectWarning:  false,
		},
		{
			name: "non-existent file with upload enabled",
			config: &common.ChecksumConfig{
				UploadEnabled:     true,
				ChecksumAlgorithm: MD5ChecksumAlgorithm,
			},
			filePath: "/non/existent/file.txt",
			expectedResult: map[string]string{
				OCIObjectMD5MetadataKey: "",
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh mock logger for each test
			testLogger := testingPkg.SetupMockLogger()

			// Set up mock expectations for Warnf if we expect a warning
			if tt.expectWarning {
				testLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()
			}

			result := GetObjectMetadatWithFileChecksum(tt.config, tt.filePath, testLogger)

			if tt.expectedResult == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Len(t, result, 1)

				// Check that the appropriate metadata key exists
				if tt.config.ChecksumAlgorithm == MD5ChecksumAlgorithm {
					assert.Contains(t, result, OCIObjectMD5MetadataKey)
					if tt.expectWarning {
						assert.Empty(t, result[OCIObjectMD5MetadataKey])
					} else {
						assert.NotEmpty(t, result[OCIObjectMD5MetadataKey])
					}
				} else if tt.config.ChecksumAlgorithm == SHA256ChecksumAlgorithm {
					assert.Contains(t, result, OCIObjectSHA256MetadataKey)
					if tt.expectWarning {
						assert.Empty(t, result[OCIObjectSHA256MetadataKey])
					} else {
						assert.NotEmpty(t, result[OCIObjectSHA256MetadataKey])
					}
				}
			}

			if tt.expectWarning {
				testLogger.AssertCalled(t, "Warnf", mock.Anything, mock.Anything, mock.Anything)
			} else {
				testLogger.AssertNotCalled(t, "Warnf")
			}
		})
	}
}

func TestGetFileChecksum_Consistency(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test_consistency_*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write test content to the file
	testContent := "Consistent checksum test content"
	_, err = tempFile.WriteString(testContent)
	assert.NoError(t, err)
	tempFile.Close()

	// Calculate MD5 checksum twice
	checksum1, err1 := GetFileChecksum(tempFile.Name(), MD5ChecksumAlgorithm)
	checksum2, err2 := GetFileChecksum(tempFile.Name(), MD5ChecksumAlgorithm)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, checksum1, checksum2, "MD5 checksums should be consistent")

	// Calculate SHA256 checksum twice
	checksum3, err3 := GetFileChecksum(tempFile.Name(), SHA256ChecksumAlgorithm)
	checksum4, err4 := GetFileChecksum(tempFile.Name(), SHA256ChecksumAlgorithm)

	assert.NoError(t, err3)
	assert.NoError(t, err4)
	assert.Equal(t, checksum3, checksum4, "SHA256 checksums should be consistent")

	// Verify different algorithms produce different checksums
	assert.NotEqual(t, checksum1, checksum3, "MD5 and SHA256 checksums should be different")
}
