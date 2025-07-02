package utils

import (
	"fmt"

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
	sourceEnvVars, err := buildEnvVarsFromStorage(spec.Source, sourcePrefix)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, sourceEnvVars...)

	destEnvVars, err := buildEnvVarsFromStorage(spec.Destination, destinationPrefix)
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
			Name:  constants.AgentAuthTypeEnvVarKey,
			Value: config.AuthType,
		},
		{
			Name:  constants.AgentRegionEnvVarKey,
			Value: config.Region,
		},
		{
			Name:  constants.AgentCompartmentIDEnvVarKey,
			Value: config.CompartmentId,
		},
		{
			Name:  constants.AgentDownloadSizeLimitEnvVarKey,
			Value: config.DownloadSizeLimit,
		},
		{
			Name:  constants.AgentEnableSizeLimitCheckEnvVarKey,
			Value: config.EnableSizeLimitCheck,
		},
	}
}

func buildEnvVarsFromStorage(storageSpec *v1beta1.StorageSpec, storagePrefix string) ([]v1.EnvVar, error) {
	storageType, err := storage.GetStorageType(*storageSpec.StorageUri)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case storage.StorageTypeOCI:
		return buildOsEnvVars(storageSpec, storagePrefix)
	case storage.StorageTypeHuggingFace:
		return buildHfEnvVars(storageSpec)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

func buildOsEnvVars(storageSpec *v1beta1.StorageSpec, storagePrefix string) ([]v1.EnvVar, error) {
	uri, err := storage.NewObjectURI(*storageSpec.StorageUri)
	if err != nil {
		return nil, err
	}
	var envVars []v1.EnvVar

	switch storagePrefix {
	case sourcePrefix:
		envVars = append(envVars,
			v1.EnvVar{Name: constants.AgentSourceNamespaceEnvVarKey, Value: uri.Namespace},
			v1.EnvVar{Name: constants.AgentSourceBucketNameEnvVarKey, Value: uri.BucketName},
			v1.EnvVar{Name: constants.AgentSourcePrefixEnvVarKey, Value: uri.Prefix},
		)
	case destinationPrefix:
		envVars = append(envVars,
			v1.EnvVar{Name: constants.AgentTargetNamespaceEnvVarKey, Value: uri.Namespace},
			v1.EnvVar{Name: constants.AgentTargetBucketNameEnvVarKey, Value: uri.BucketName},
			v1.EnvVar{Name: constants.AgentTargetPrefixEnvVarKey, Value: uri.Prefix},
		)
	}

	// Handle parameters override
	if storageSpec.Parameters == nil {
		return envVars, nil
	}
	params := *storageSpec.Parameters

	switch storagePrefix {
	case sourcePrefix:
		if region, ok := params["region"]; ok {
			envVars = append(envVars, v1.EnvVar{
				Name:  constants.AgentSourceRegionEnvVarKey,
				Value: region,
			})
		}
		if authType, ok := params["auth"]; ok {
			// default auth type is consistent with ome agent auth type InstancePrincipal
			envVars = append(envVars, v1.EnvVar{
				Name:  constants.AgentAuthTypeEnvVarKey,
				Value: authType,
			})
		}
		if oboToken, ok := params[constants.OboTokenConfigKey]; ok {
			envVars = append(envVars,
				v1.EnvVar{Name: constants.AgentOboTokenEnvVarKey, Value: oboToken},
				v1.EnvVar{Name: constants.AgentEnableOboTokenEnvVarKey, Value: "true"},
			)
		}
	case destinationPrefix:
		if region, ok := params["region"]; ok {
			envVars = append(envVars, v1.EnvVar{
				Name:  constants.AgentTargetRegionEnvVarKey,
				Value: region,
			})
		}
	}

	return envVars, nil
}

func buildHfEnvVars(storageSpec *v1beta1.StorageSpec) ([]v1.EnvVar, error) {
	uri, err := storage.NewObjectURI(*storageSpec.StorageUri)
	if err != nil {
		return nil, err
	}
	var envVars []v1.EnvVar

	envVars = append(envVars,
		v1.EnvVar{Name: constants.AgentSourceNamespaceEnvVarKey, Value: uri.Namespace},
		v1.EnvVar{Name: constants.AgentSourceBucketNameEnvVarKey, Value: uri.BucketName},
		v1.EnvVar{Name: constants.AgentSourcePrefixEnvVarKey, Value: uri.Prefix},
	)

	// Handle parameters override
	if storageSpec.Parameters == nil {
		return envVars, nil
	}
	params := *storageSpec.Parameters

	if authType, ok := params["auth"]; ok {
		// default auth type is consistent with ome agent auth type InstancePrincipal
		envVars = append(envVars, v1.EnvVar{
			Name:  constants.AgentAuthTypeEnvVarKey,
			Value: authType,
		})
	}
	if hfToken, ok := params["hf_token"]; ok {
		envVars = append(envVars, v1.EnvVar{
			Name:  constants.AgentHFTokenEnvVarKey,
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
