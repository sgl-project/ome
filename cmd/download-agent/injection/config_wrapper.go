package injection

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"go.uber.org/fx"
)

/*CasperConfigWrapper provides CasperConfig to the fx app defined in casper module (from casper pkg).
 * The initialized configuration in this struct will be added to the "casperConfigs" group, further allowing multiple
 * CasperConfig to be injected and managed collectively.
 * More info regarding fx Value Groups can be found: https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
 */
type CasperConfigWrapper struct {
	fx.Out

	CasperConfig *casper.Config `group:"casperConfigs"`
}

/*KMSConfigWrapper provides KMSConfig to the fx app defined in secret/key_management module (from secrets pkg).
 * The initialized configuration in this struct will be added to the "kmsConfigs" group, further allowing multiple
 * KMSConfig to be injected and managed collectively.
 * More info regarding fx Value Groups can be found: https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
 */
type KMSConfigWrapper struct {
	fx.Out

	KMSConfig *keymanagement.KmsConfig `group:"kmsConfigs"`
}

/*SecretRetrievalConfigWrapper provides SecretRetrievalConfig to the fx app defined in secret/secret_retrieval module (from secrets pkg).
 * The initialized configuration in this struct will be added to the "secretRetrievalConfigs" group, further allowing multiple
 * SecretRetrievalConfig to be injected and managed collectively.
 * More info regarding fx Value Groups can be found: https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
 */
type SecretRetrievalConfigWrapper struct {
	fx.Out

	SecretRetrievalConfig *secretretrieval.SecretRetrievalConfig `group:"secretRetrievalConfigs"`
}
