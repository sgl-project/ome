package vars

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vibe"
)

const jsonContent = "{" +
	"  \"ad_number_name\": \"ad1\"," +
	"  \"airport\": \"XXP\"," +
	"  \"availability_domain\": \"ad1\"," +
	"  \"bootstrapFootprint\": \"base.0-0\"," +
	"  \"id_code\": \"xp-1\"," +
	"  \"identity_realm\": \"rb1\"," +
	"  \"name\": \"sol-phoebe-1-ad-1\"," +
	"  \"realm\": \"oc1\"," +
	"  \"region\": \"us-phoenix-1\"," +
	"  \"region_state\": \"Building\"," +
	"  \"service_enclave_dns_suffix\": \"svc.ad1.sol-phoebe-1\"" +
	"}"

func TestVibeResolver_Resolve(t *testing.T) {

	config := vibe.Config{
		MetadataFilePath: vibe.DefaultMetadataFilePath,
	}
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, vibe.DefaultMetadataFilePath, []byte(jsonContent), 0)
	require.NoError(t, err)

	resolver, err := NewVibeResolver(config, fs)
	require.NoError(t, err)

	t.Run("realm", func(t *testing.T) {
		realm, err := resolver.Resolve(VibeRealm)
		require.NoError(t, err)
		require.Equal(t, "oc1", realm)
	})
	t.Run("region", func(t *testing.T) {
		region, err := resolver.Resolve(VibeRegion)
		require.NoError(t, err)
		require.Equal(t, "us-phoenix-1", region)
	})
	t.Run("airport code", func(t *testing.T) {
		code, err := resolver.Resolve(VibeAirportCode)
		require.NoError(t, err)
		require.Equal(t, "XXP", code)
	})
	t.Run("service enclave suffix", func(t *testing.T) {
		suffix, err := resolver.Resolve(VibeSvcEnclaveSuffix)
		require.NoError(t, err)
		require.Equal(t, "svc.ad1.sol-phoebe-1", suffix)
	})
	t.Run("identity realm", func(t *testing.T) {
		idRealm, err := resolver.Resolve(VibeIdentityRealm)
		require.NoError(t, err)
		require.Equal(t, "rb1", idRealm)
	})
	t.Run("ldap realm", func(t *testing.T) {
		ldapRealm, err := resolver.Resolve(VibeLdapifiedRealm)
		require.NoError(t, err)
		require.Equal(t, "prod", ldapRealm)
	})
	t.Run("ldap region", func(t *testing.T) {
		ldapRegion, err := resolver.Resolve(VibeLdapifiedRegion)
		require.NoError(t, err)
		require.Equal(t, "r2", ldapRegion)
	})
	t.Run("unsupported var", func(t *testing.T) {
		_, err := resolver.Resolve(RealmTLD)
		require.Error(t, err)
	})
}
