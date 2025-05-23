package vault

import "fmt"

type SecretConfig struct {
	CompartmentId *string `mapstructure:"compartment_id"`
	SecretId      *string `mapstructure:"secret_id"`
	SecretName    *string `mapstructure:"secret_name"`
	VaultId       *string `mapstructure:"vault_id"`
	KeyId         *string `mapstructure:"key_id"`

	SecretVersionConfig *SecretVersionConfig `mapstructure:"secret_version_config"`
}

func (sc *SecretConfig) ValidateNameAndVaultId() error {
	if sc.SecretName == nil || sc.VaultId == nil || len(*sc.SecretName) == 0 || len(*sc.VaultId) == 0 {
		return fmt.Errorf("SecretName and VaultId must be provided")
	}
	return nil
}

func (sc *SecretConfig) ValidateSecretId() error {
	if sc.SecretId == nil || len(*sc.SecretId) == 0 {
		return fmt.Errorf("SecretId must be provided")
	}
	return nil
}

type SecretVersionStage string

const (
	CurrentStage    SecretVersionStage = "CURRENT"
	PendingStage    SecretVersionStage = "PENDING"
	LatestStage     SecretVersionStage = "LATEST"
	PreviousStage   SecretVersionStage = "PREVIOUS"
	DeprecatedStage SecretVersionStage = "DEPRECATED"
)

// SecretVersionConfig Use New() to construct it
type SecretVersionConfig struct {
	Stage               *SecretVersionStage `mapstructure:"secret_version_stage"`
	SecretVersionNumber *int64              `mapstructure:"secret_version_number"`
	SecretVersionName   *string             `mapstructure:"secret_version_name"`
}
