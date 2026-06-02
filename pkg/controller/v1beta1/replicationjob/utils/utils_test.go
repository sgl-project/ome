package utils

import (
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/stretchr/testify/assert"
	opensourcev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const (
	validSrc        = "oci://n/source_namespace/b/source_bucket/o/source_prefix"
	validDest       = "oci://n/dest_namespace/b/dest_bucket/o/dest_prefix"
	invalidSrc      = "unsupported://src"
	invalidDest     = "unsupported://dest"
	unsupportedDest = "hf://src"
)

func TestBuildEnvVars_SuccessOCI(t *testing.T) {
	spec := v1beta1.ReplicationJobSpec{
		Source: &opensourcev1beta1.StorageSpec{
			StorageUri: utils.Ptr("oci://n/source_namespace/b/source_bucket/o/source_prefix"),
			Path:       utils.Ptr("/source/path"),
			Parameters: utils.Ptr(map[string]string{
				regionParamKey:   "us-ashburn-1",
				oboTokenParamKey: "oboToken",
			}),
		},
		Destination: &opensourcev1beta1.StorageSpec{
			StorageUri: utils.Ptr("oci://n/dest_namespace/b/dest_bucket/o/dest_prefix"),
			Path:       utils.Ptr("/dest/path"),
			Parameters: utils.Ptr(map[string]string{
				regionParamKey: "us-phoenix-1",
			}),
		},
	}

	config := controllerconfig.ReplicationJobConfig{
		CompartmentId:        "compartment123",
		DownloadSizeLimit:    "100GB",
		EnableSizeLimitCheck: true,
		EnableChecksumUpload: true,
		ChecksumAlgorithm:    "sha256",
		Source: controllerconfig.StorageAccessConfig{
			AuthType:       "InstancePrincipal",
			EnableOboToken: true,
		},
		Target: controllerconfig.StorageAccessConfig{
			AuthType:       "InstancePrincipal",
			EnableOboToken: false,
		},
	}

	envVars, err := BuildEnvVars(spec, &config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedEnvVars := map[string]string{
		constants.AgentLocalPathEnvVarKey:                     "/source/path",
		constants.AgentSourceStorageURIEnvVarKey:              "oci://n/source_namespace/b/source_bucket/o/source_prefix",
		constants.AgentSourceOCIEnabledEnvVarKey:              "true",
		constants.AgentSourcePVCEnabledEnvVarKey:              "false",
		constants.AgentSourceOCIAuthTypeEnvVarKey:             "InstancePrincipal",
		constants.AgentSourceOCIEnableOboTokenEnvVarKey:       "true",
		constants.AgentSourceOCIOboTokenEnvVarKey:             "oboToken",
		constants.AgentSourceOCIRegionEnvVarKey:               "us-ashburn-1",
		constants.AgentTargetStorageURIEnvVarKey:              "oci://n/dest_namespace/b/dest_bucket/o/dest_prefix",
		constants.AgentTargetOCIEnabledEnvVarKey:              "true",
		constants.AgentTargetPVCEnabledEnvVarKey:              "false",
		constants.AgentTargetOCIAuthTypeEnvVarKey:             "InstancePrincipal",
		constants.AgentTargetOCIEnableOboTokenEnvVarKey:       "false",
		constants.AgentTargetOCIRegionEnvVarKey:               "us-phoenix-1",
		constants.AgentTargetOCIEnableChecksumUploadEnvVarKey: "true",
		constants.AgentTargetOCIChecksumAlgorithmEnvVarKey:    "sha256",
		constants.AgentCompartmentIDEnvVarKey:                 "compartment123",
		constants.AgentDownloadSizeLimitEnvVarKey:             "100GB",
		constants.AgentEnableSizeLimitCheckEnvVarKey:          "true",
	}

	for _, envVar := range envVars {
		expectedValue, ok := expectedEnvVars[envVar.Name]
		if !ok {
			t.Errorf("unexpected env var: %s", envVar.Name)
			continue
		}
		if envVar.Value != expectedValue {
			t.Errorf("env var %s: expected %q, got %q", envVar.Name, expectedValue, envVar.Value)
		}
	}
}

func TestBuildEnvVars_UnsupportedStorageType(t *testing.T) {
	spec := v1beta1.ReplicationJobSpec{
		Source: &opensourcev1beta1.StorageSpec{
			StorageUri: utils.Ptr("unsupported://source"),
			Path:       utils.Ptr(""),
		},
		Destination: &opensourcev1beta1.StorageSpec{
			StorageUri: utils.Ptr("unsupported://dest"),
		},
	}

	config := controllerconfig.ReplicationJobConfig{}

	_, err := BuildEnvVars(spec, &config)
	if err == nil {
		t.Fatal("expected error for unsupported storage type, got nil")
	}
}

func TestValidateStorageUris(t *testing.T) {

	testCases := []struct {
		name           string
		source         *opensourcev1beta1.StorageSpec
		destination    *opensourcev1beta1.StorageSpec
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:           "Both source and destination nil",
			source:         nil,
			destination:    nil,
			wantErr:        true,
			wantErrMessage: "storageSpec cannot be nil",
		},
		{
			name:           "Source nil",
			source:         nil,
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validDest)}),
			wantErr:        true,
			wantErrMessage: "storageSpec cannot be nil",
		},
		{
			name:           "Destination nil",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validSrc)}),
			destination:    nil,
			wantErr:        true,
			wantErrMessage: "storageSpec cannot be nil",
		},
		{
			name:           "Source StorageUri nil",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: nil}),
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validDest)}),
			wantErr:        true,
			wantErrMessage: "storageUri cannot be nil",
		},
		{
			name:           "Destination StorageUri nil",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validSrc)}),
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: nil}),
			wantErr:        true,
			wantErrMessage: "storageUri cannot be nil",
		},
		{
			name:           "Invalid source URI",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(invalidSrc)}),
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validDest)}),
			wantErr:        true,
			wantErrMessage: "unknown storage type for URI",
		},
		{
			name:           "Invalid destination URI",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validSrc)}),
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(invalidDest)}),
			wantErr:        true,
			wantErrMessage: "unknown storage type for URI",
		},
		{
			name:           "Unsupported destination URI",
			source:         &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validSrc)}),
			destination:    &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(unsupportedDest)}),
			wantErr:        true,
			wantErrMessage: "destination storageType HUGGINGFACE is not supported for replication",
		},
		{
			name:        "Valid source and destination types",
			source:      &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validSrc)}),
			destination: &(opensourcev1beta1.StorageSpec{StorageUri: utils.Ptr(validDest)}),
			wantErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStorageUris(tc.source, tc.destination)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrMessage != "" {
					assert.Contains(t, err.Error(), tc.wantErrMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatClientErrorAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMsg    string
		wantStatus string
	}{
		{
			name:       "Empty message",
			input:      "",
			wantMsg:    "",
			wantStatus: "",
		},
		{
			name:       "Unauthorized - 401",
			input:      "HTTP 401 error",
			wantMsg:    "Unauthorized: Authentication failed. Please check your HuggingFace authentication token.",
			wantStatus: constants.AuthFailed,
		},
		{
			name:       "Unauthorized - word",
			input:      "User is unauthorized to access this model",
			wantMsg:    "Unauthorized: Authentication failed. Please check your HuggingFace authentication token.",
			wantStatus: constants.AuthFailed,
		},
		{
			name:       "Forbidden - 403",
			input:      "http 403: forbidden",
			wantMsg:    "Forbidden: Access denied. Please check your HuggingFace authentication token and repository permissions.",
			wantStatus: constants.PermissionDenied,
		},
		{
			name:       "Forbidden - gated",
			input:      "The model is gated and requires authentication",
			wantMsg:    "Forbidden: Access denied. Please check your HuggingFace authentication token and repository permissions.",
			wantStatus: constants.PermissionDenied,
		},
		{
			name:       "Not Found - 404",
			input:      "HTTP 404",
			wantMsg:    "Not found: The requested model or file could not be found in the repository. Please verify the HuggingFace model or file name.",
			wantStatus: constants.NotFound,
		},
		{
			name:       "Not Found - file not found",
			input:      "file not found on server",
			wantMsg:    "Not found: The requested model or file could not be found in the repository. Please verify the HuggingFace model or file name.",
			wantStatus: constants.NotFound,
		},
		{
			name:       "Other message",
			input:      "some other error occurred",
			wantMsg:    "some other error occurred",
			wantStatus: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotMsg, gotStatus := FormatClientErrorAndStatus(tc.input)
			assert.Equal(t, tc.wantMsg, gotMsg)
			assert.Equal(t, tc.wantStatus, gotStatus)
		})
	}
}
