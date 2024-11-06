package partner_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretinvault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_in_vault"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	CasperDataStoreList []*casper.CasperDataStore
	KmsCryptoList       []*keymanagement.CryptoClient
	KmsManagementList   []*keymanagement.KmsKeyManager
	SecretRetrievalList []*secretretrieval.SecretRetriever
	SecretInVault       *secretinvault.SecretInVault //Ony has one instance of this type, no need to use list (value group) to inject
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params appParams) (*DownloadAgent, error) {
		config, err := NewConfig(
			WithAppParams(params),
			WithViper(v),
			WithEnv(e),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating partner download agent config: %+v", err)
		}
		return NewDownloadAgent(config)
	})
