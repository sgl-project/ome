package serving_init

import (
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	KmsCrypto     *keymanagement.KmsCrypto
	KmsManagement *keymanagement.KmsManagement
	SecretInVault *secretretrieval.SecretRetrieval
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params appParams) (*ServingInit, error) {
		config, err := NewConfig(
			WithAppParams(params),
			WithViper(v, params.AnotherLogger),
			WithEnv(e),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating serving init config: %+v", err)
		}
		return NewApplication(config)
	})
