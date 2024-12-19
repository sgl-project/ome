package key_management

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideKmsConfig(v *viper.Viper, e *env.Environment, logger logging.Interface) (*KmsConfig, error) {
	kmsConfig, err := NewKmsConfig(
		WithViper(v),
		WithEnv(e),
		WithAnotherLog(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}
	return kmsConfig, nil
}

func ProvideKmsCrypto(v *viper.Viper, e *env.Environment, logger logging.Interface) (*CryptoClient, error) {
	kmsConfig, err := ProvideKmsConfig(v, e, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}

	kmsCrypto, err := NewCryptoClient(kmsConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing CryptoClient: %+v", err)
	}
	return kmsCrypto, nil
}

func ProvideKmsManagement(v *viper.Viper, e *env.Environment, logger logging.Interface) (*KmsKeyManager, error) {
	kmsConfig, err := ProvideKmsConfig(v, e, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsConfig: %+v", err)
	}

	kmsManagement, err := NewKmsKeyManager(kmsConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing KmsKeyManager: %+v", err)
	}
	return kmsManagement, nil
}

var KmsCryptoModule = fx.Provide(
	ProvideKmsCrypto,
)

var KmsManagementModule = fx.Provide(
	ProvideKmsManagement,
)

/*
 * Below is a way to inject a list of CryptoClient and KmsKeyManager using a list of Configs leveraging fx Value Groups feature
 */
type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*KmsConfig `group:"kmsConfigs"`
}

func ProvideListOfKmsCryptoWithAppParams(e *env.Environment, params appParams) ([]*CryptoClient, error) {
	kmsCryptoList := make([]*CryptoClient, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		kmsCrypto, err := NewCryptoClient(config, e)
		if err != nil {
			return kmsCryptoList, fmt.Errorf("error initializing a list of CryptoClient using KmsConfig: %+v: %+v", config, err)
		}
		kmsCryptoList = append(kmsCryptoList, kmsCrypto)
	}
	return kmsCryptoList, nil
}

func ProvideListOfKmsManagementWithAppParams(e *env.Environment, params appParams) ([]*KmsKeyManager, error) {
	kmsManagementList := make([]*KmsKeyManager, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		kmsManagement, err := NewKmsKeyManager(config, e)
		if err != nil {
			return kmsManagementList, fmt.Errorf("error initializing a list of KmsKeyManager using KmsConfig: %+v: %+v", config, err)
		}
		kmsManagementList = append(kmsManagementList, kmsManagement)
	}
	return kmsManagementList, nil
}
