package utils

import (
	"fmt"
	"strconv"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils/storage"
	v1 "k8s.io/api/core/v1"
)

const (
	sourcePrefix      = "Source"
	destinationPrefix = "Destination"

	regionParamKey   = "region"
	oboTokenParamKey = "obo_token"
)

var (
	supportedSourceStorageTypes = map[storage.StorageType]bool{
		storage.StorageTypeOCI:         true,
		storage.StorageTypePVC:         true,
		storage.StorageTypeVendor:      true,
		storage.StorageTypeHuggingFace: true,
	}
	supportedDestinationStorageTypes = map[storage.StorageType]bool{
		storage.StorageTypeOCI: true,
		storage.StorageTypePVC: true,
	}
)

func BuildEnvVars(spec v1beta1.ReplicationJobSpec, config *controllerconfig.ReplicationJobConfig) ([]v1.EnvVar, error) {
	envVars := buildGeneralEnvVars(spec, config)

	// Handle source and destination storage separately, as they may have different storage types.
	sourceEnvVars, err := buildEnvVarsFromStorage(spec.Source, config, sourcePrefix)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, sourceEnvVars...)

	destEnvVars, err := buildEnvVarsFromStorage(spec.Destination, config, destinationPrefix)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, destEnvVars...)

	return envVars, nil
}

func buildGeneralEnvVars(spec v1beta1.ReplicationJobSpec, config *controllerconfig.ReplicationJobConfig) []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name:  constants.AgentLocalPathEnvVarKey,
			Value: utils.DerefString(spec.Source.Path),
		},
		{
			Name:  constants.AgentDownloadSizeLimitEnvVarKey,
			Value: config.DownloadSizeLimit,
		},
		{
			Name:  constants.AgentEnableSizeLimitCheckEnvVarKey,
			Value: strconv.FormatBool(config.EnableSizeLimitCheck),
		},
	}
}

func buildEnvVarsFromStorage(storageSpec *v1beta1.StorageSpec, config *controllerconfig.ReplicationJobConfig, storagePrefix string) ([]v1.EnvVar, error) {
	storageType, err := storage.GetStorageType(*storageSpec.StorageUri)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case storage.StorageTypeOCI:
		return buildOsEnvVars(storageSpec, config, storagePrefix)
	case storage.StorageTypeHuggingFace:
		return buildHfEnvVars(storageSpec)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

func buildOsEnvVars(storageSpec *v1beta1.StorageSpec, config *controllerconfig.ReplicationJobConfig, storagePrefix string) ([]v1.EnvVar, error) {
	var envVars []v1.EnvVar

	switch storagePrefix {
	case sourcePrefix:
		envVars = append(envVars,
			v1.EnvVar{Name: constants.AgentSourceStorageURIEnvVarKey, Value: *storageSpec.StorageUri},
			v1.EnvVar{Name: constants.AgentSourceOCIEnabledEnvVarKey, Value: "true"},
			v1.EnvVar{Name: constants.AgentSourcePVCEnabledEnvVarKey, Value: "false"},
			v1.EnvVar{Name: constants.AgentSourceOCIAuthTypeEnvVarKey, Value: config.Source.AuthType},
			v1.EnvVar{Name: constants.AgentSourceOCIEnableOboTokenEnvVarKey, Value: strconv.FormatBool(config.Source.EnableOboToken)},
		)

	case destinationPrefix:
		envVars = append(envVars,
			v1.EnvVar{Name: constants.AgentTargetStorageURIEnvVarKey, Value: *storageSpec.StorageUri},
			v1.EnvVar{Name: constants.AgentTargetOCIEnabledEnvVarKey, Value: "true"},
			v1.EnvVar{Name: constants.AgentTargetPVCEnabledEnvVarKey, Value: "false"},
			v1.EnvVar{Name: constants.AgentTargetOCIAuthTypeEnvVarKey, Value: config.Target.AuthType},
			v1.EnvVar{Name: constants.AgentTargetOCIEnableOboTokenEnvVarKey, Value: strconv.FormatBool(config.Target.EnableOboToken)},
			v1.EnvVar{Name: constants.AgentTargetOCIEnableChecksumUploadEnvVarKey, Value: strconv.FormatBool(config.EnableChecksumUpload)},
			v1.EnvVar{Name: constants.AgentTargetOCIChecksumAlgorithmEnvVarKey, Value: config.ChecksumAlgorithm},
		)
	}

	// Handle parameters override
	if storageSpec.Parameters == nil {
		return envVars, nil
	}
	params := *storageSpec.Parameters

	switch storagePrefix {
	case sourcePrefix:
		if region, ok := params[regionParamKey]; ok && region != "" {
			envVars = append(envVars, v1.EnvVar{
				Name:  constants.AgentSourceOCIRegionEnvVarKey,
				Value: region,
			})
		}
		if oboToken, ok := params[oboTokenParamKey]; ok && oboToken != "" {
			envVars = append(envVars,
				v1.EnvVar{Name: constants.AgentSourceOCIOboTokenEnvVarKey, Value: oboToken},
			)
		}
	case destinationPrefix:
		if region, ok := params[regionParamKey]; ok && region != "" {
			envVars = append(envVars, v1.EnvVar{
				Name:  constants.AgentTargetOCIRegionEnvVarKey,
				Value: region,
			})
		}
		if oboToken, ok := params[oboTokenParamKey]; ok && oboToken != "" {
			envVars = append(envVars,
				v1.EnvVar{Name: constants.AgentTargetOCIOboTokenEnvVarKey, Value: oboToken},
			)
		}
	}

	return envVars, nil
}

func buildHfEnvVars(storageSpec *v1beta1.StorageSpec) ([]v1.EnvVar, error) {
	var envVars []v1.EnvVar

	envVars = append(envVars,
		v1.EnvVar{Name: constants.AgentSourceStorageURIEnvVarKey, Value: *storageSpec.StorageUri},
		v1.EnvVar{Name: constants.AgentSourceOCIEnabledEnvVarKey, Value: "false"},
		v1.EnvVar{Name: constants.AgentSourcePVCEnabledEnvVarKey, Value: "false"},
	)

	// Handle parameters override
	if storageSpec.Parameters == nil {
		return envVars, nil
	}
	params := *storageSpec.Parameters

	if hfToken, ok := params["hf_token"]; ok {
		envVars = append(envVars, v1.EnvVar{
			Name:  constants.AgentSourceHFTokenEnvVarKey,
			Value: hfToken,
		})
	}

	return envVars, nil
}

func ValidateStorageUris(source *v1beta1.StorageSpec, destination *v1beta1.StorageSpec) error {
	if source == nil || destination == nil {
		return fmt.Errorf("storageSpec cannot be nil")
	}
	if source.StorageUri == nil || destination.StorageUri == nil {
		return fmt.Errorf("storageUri cannot be nil")
	}

	err := storage.ValidateStorageURI(*source.StorageUri)
	if err != nil {
		return err
	}

	err = storage.ValidateStorageURI(*destination.StorageUri)
	if err != nil {
		return err
	}

	sourceStorageType, err := storage.GetStorageType(*source.StorageUri)
	if err != nil {
		return fmt.Errorf("invalid source storage URI: %v", err)
	}
	if !supportedSourceStorageTypes[sourceStorageType] {
		return fmt.Errorf("source storageType %v is not supported for replication", sourceStorageType)
	}

	destStorageType, err := storage.GetStorageType(*destination.StorageUri)
	if err != nil {
		return fmt.Errorf("invalid source storage URI: %v", err)
	}
	if !supportedDestinationStorageTypes[destStorageType] {
		return fmt.Errorf("destination storageType %v is not supported for replication", destStorageType)
	}
	return nil
}

func FormatClientErrorAndStatus(message string) (string, string) {
	if message == "" {
		return message, ""
	}

	msgLower := strings.ToLower(message)

	unauthorizedPatterns := []string{"http 401", "401", "unauthorized"}
	forbiddenPatterns := []string{"http 403", "403", "forbidden", "gated", "is gated", "requires authentication"}
	notFoundPatterns := []string{"http 404", "404", "not found", "file not found"}

	if utils.ContainsAny(msgLower, unauthorizedPatterns) {
		return "Unauthorized: Authentication failed. Please check your HuggingFace authentication token.", constants.AuthFailed
	}

	if utils.ContainsAny(msgLower, forbiddenPatterns) {
		return "Forbidden: Access denied. Please check your HuggingFace authentication token and repository permissions.", constants.PermissionDenied
	}

	if utils.ContainsAny(msgLower, notFoundPatterns) {
		return "Not found: The requested model or file could not be found in the repository. Please verify the HuggingFace model or file name.", constants.NotFound
	}

	return message, ""
}
