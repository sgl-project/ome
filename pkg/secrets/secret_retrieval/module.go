package secret_retrieval

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideSecretRetrievalConfig(v *viper.Viper, e *env.Environment, logger logging.Interface) (*SecretRetrievalConfig, error) {
	secretRetrievalConfig, err := NewSecretRetrievalConfig(
		WithViper(v),
		WithEnv(e),
		WithAnotherLog(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetrievalConfig: %+v", err)
	}
	return secretRetrievalConfig, nil
}

func ProvideSecretRetrieval(v *viper.Viper, e *env.Environment, logger logging.Interface) (*SecretRetriever, error) {
	secretRetrievalConfig, err := ProvideSecretRetrievalConfig(v, e, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetrievalConfig: %+v", err)
	}

	secretRetrieval, err := NewSecretRetriever(secretRetrievalConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretRetriever: %+v", err)
	}
	return secretRetrieval, nil
}

var SecretRetrievalModule = fx.Provide(
	ProvideSecretRetrieval,
)

/*
 * Below is a way to inject a list of SecretRetriever using a list of Configs leveraging fx Value Groups feature
 * Regarding how to use it, you can refer to the code snippet under:
 * ome/cmd/hf_download-agent/injection/partner-injection.go, in CasperDataStoreListProvider function
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
	Configs []*SecretRetrievalConfig `group:"secretRetrievalConfigs"`
}

func ProvideListOfSecretRetrievalWithAppParams(e *env.Environment, params appParams) ([]*SecretRetriever, error) {
	secretRetrievalList := make([]*SecretRetriever, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		secretRetrieval, err := NewSecretRetriever(config, e)
		if err != nil {
			return secretRetrievalList, fmt.Errorf("error initializing a list of SecretRetriever using Config: %+v: %+v", config, err)
		}
		secretRetrievalList = append(secretRetrievalList, secretRetrieval)
	}
	return secretRetrievalList, nil
}
