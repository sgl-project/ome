package secret_retrieval

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideSecretRetrievalConfig(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*SecretRetrievalConfig, error) {
	secretRetrievalConfig, err := NewSecretRetrievalConfig(
		WithViper(v, viperKeyNames),
		WithEnv(e),
		WithAnotherLog(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetrievalConfig: %+v", err)
	}
	return secretRetrievalConfig, nil
}

func ProvideSecretRetrieval(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*SecretRetrieval, error) {
	secretRetrievalConfig, err := ProvideSecretRetrievalConfig(v, e, logger, viperKeyNames)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetrievalConfig: %+v", err)
	}

	secretRetrieval, err := NewSecretRetrieval(secretRetrievalConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetrieval: %+v", err)
	}
	return secretRetrieval, nil
}

var SecretRetrievalModule = fx.Provide(
	ProvideSecretRetrieval,
)
