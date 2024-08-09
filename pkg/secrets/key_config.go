package secrets

import "github.com/oracle/oci-go-sdk/v65/keymanagement"

type DefinedTag struct {
	Namespace string `mapstructure:"defined_tag_namespace"`
	Key       string `mapstructure:"defined_tag_key"`
	Value     string `mapstructure:"defined_tag_value"`
}

type KeyConfig struct {
	//Required
	CompartmentId string `mapstructure:"compartment_id"`

	//Optional
	Name             string                                     `mapstructure:"key_name"`
	Algorithm        keymanagement.ListKeysAlgorithmEnum        `mapstructure:"key_algorithm"`
	Length           int                                        `mapstructure:"key_length"`
	ProtectMode      keymanagement.ListKeysProtectionModeEnum   `mapstructure:"key_protection_mode"`
	LifecycleStyle   keymanagement.KeySummaryLifecycleStateEnum `mapstructure:"key_lifecycle"`
	EnableDefinedTag bool                                       `mapstructure:"enable_key_defined_tag"`
	DefinedTag       DefinedTag                                 `mapstructure:"key_defined_tag"`
}

type KeyConfigOption func(sc *KeyConfig)

func WithName(name string) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.Name = name
	}
}

func WithAlgorithm(algorithm keymanagement.ListKeysAlgorithmEnum) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.Algorithm = algorithm
	}
}

func WithLength(length int) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.Length = length
	}
}

func WithProtectMode(protectMode keymanagement.ListKeysProtectionModeEnum) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.ProtectMode = protectMode
	}
}

func WithLifecycleStyle(lifecycleStyle keymanagement.KeySummaryLifecycleStateEnum) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.LifecycleStyle = lifecycleStyle
	}
}

func WithDefinedTag(definedTag DefinedTag) KeyConfigOption {
	return func(kc *KeyConfig) {
		kc.DefinedTag = definedTag
	}
}

func NewKeyConfig(compartment string, keyConfigOpts ...KeyConfigOption) *KeyConfig {
	keyConfig := &KeyConfig{
		CompartmentId: compartment,
	}

	for _, opt := range keyConfigOpts {
		opt(keyConfig)
	}
	return keyConfig
}
