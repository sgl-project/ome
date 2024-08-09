package key_management

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideKmsConfig(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*KmsConfig, error) {
	kmsConfig, err := NewKmsConfig(
		WithViper(v, viperKeyNames),
		WithEnv(e),
		WithAnotherLog(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}
	return kmsConfig, nil
}

func ProvideKmsCrypto(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*KmsCrypto, error) {
	kmsConfig, err := ProvideKmsConfig(v, e, logger, viperKeyNames)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}

	kmsCrypto, err := NewKmsCrypto(kmsConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsCrypto: %+v", err)
	}
	return kmsCrypto, nil
}

func ProvideKmsManagement(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*KmsManagement, error) {
	kmsConfig, err := ProvideKmsConfig(v, e, logger, viperKeyNames)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}

	kmsManagement, err := NewKmsManagement(kmsConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsManagement: %+v", err)
	}
	return kmsManagement, nil
}

var KmsModule = fx.Provide(
	ProvideKmsCrypto,
	ProvideKmsManagement,
)
