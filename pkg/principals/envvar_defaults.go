package principals

import "bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"

const (
	/** common **/

	EnvOciSdkAuthClientRegionUrl = "OCI_SDK_AUTH_CLIENT_REGION_URL"

	/** resource principals **/

	EnvResourcePrincipalVersion      = "OCI_RESOURCE_PRINCIPAL_VERSION"
	EnvResourcePrincipalRPTEndpoint  = "OCI_RESOURCE_PRINCIPAL_RPT_ENDPOINT"
	EnvResourcePrincipalRPTPath      = "OCI_RESOURCE_PRINCIPAL_RPT_PATH"
	EnvResourcePrincipalRPSTEndpoint = "OCI_RESOURCE_PRINCIPAL_RPST_ENDPOINT"
	EnvResourcePrincipalRegion       = "OCI_RESOURCE_PRINCIPAL_REGION"
)

var (
	/** common **/

	DefaultAuthClientRegionURLSubstrate = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"us-phoenix-1": "https://authservice.svc.${ad}.r2",
			"default":      "https://authservice.svc.${ad}.${region}",
		},
	})
	DefaultAuthClientRegionURLOverlay = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"r1":      "https://auth.r1.${realmTLD}",
			"default": "https://auth.${region}.${realmTLD}",
		},
	})
	DefaultRegion = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"default": "${region}",
		},
	})

	/** resource principals **/

	DefaultResourcePrincipalVersion11 = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"default": "1.1",
		},
	})

	DefaultResourcePrincipalRPTEndpoint = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"r1":      "https://database.r1.oracleiaas.com",
			"default": "https://database.${region}.${realmTLD}",
		},
	})

	DefaultResourcePrincipalRPTPath = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"default": "/20180711/resourcePrincipalToken",
		},
	})

	DefaultResourcePrincipalRPSTEndpoint = ensureValid(&EnvVar{
		ValueByRegion: env.StringByRegion{
			"r1":      "https://auth.r1.oracleiaas.com",
			"default": "https://auth.${region}.${realmTLD}",
		},
	})
)

// ensureValid ensures the default EnvVars above are valid.
// It will panic during at `go test ./...` if they're not
func ensureValid(ev *EnvVar) *EnvVar {
	if err := ev.Validate(); err != nil {
		panic("invalid default EnvVar: " + err.Error())
	}

	return ev
}
