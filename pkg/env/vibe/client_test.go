package vibe

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const jsonContent = "{" +
	"  \"ad_number_name\": \"ad1\"," +
	"  \"airport\": \"XXP\"," +
	"  \"availability_domain\": \"ad1\"," +
	"  \"bootstrapFootprint\": \"base.0-0\"," +
	"  \"id_code\": \"xp-1\"," +
	"  \"identity_realm\": \"rb1\"," +
	"  \"name\": \"sol-phoebe-1-ad-1\"," +
	"  \"realm\": \"region1\"," +
	"  \"region\": \"us-seattle-1\"," +
	"  \"region_state\": \"Building\"," +
	"  \"service_enclave_dns_suffix\": \"svc.ad1.sol-phoebe-1\"" +
	"}"

func TestNewMetadataClient(t *testing.T) {
	config := Config{
		MetadataFilePath: DefaultMetadataFilePath,
	}

	t.Run("nil filesystem", func(t *testing.T) {
		_, err := NewMetadataClient(config, nil)
		require.ErrorContainsf(t, err, "nil filesystem", "unexpected error message")
	})

	t.Run("metadata file missing", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := afero.WriteFile(fs, "locale.json", []byte(""), 0)
		require.NoError(t, err)

		_, err = NewMetadataClient(config, fs)
		require.ErrorContains(t, err, "file does not exist")
	})

	t.Run("invalid or empty json content", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := afero.WriteFile(fs, DefaultMetadataFilePath, []byte(""), 0)
		require.NoError(t, err)

		_, err = NewMetadataClient(config, fs)
		require.Error(t, err)
	})

	t.Run("valid json", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := afero.WriteFile(fs, DefaultMetadataFilePath, []byte("{}"), 0)
		require.NoError(t, err)

		_, err = NewMetadataClient(config, fs)
		require.NoError(t, err)
	})

}

func TestClient_Provider(t *testing.T) {
	config := Config{
		MetadataFilePath: DefaultMetadataFilePath,
	}
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, DefaultMetadataFilePath, []byte(jsonContent), 0)
	require.NoError(t, err)

	client, err := NewMetadataClient(config, fs)
	require.NoError(t, err)

	t.Run("region", func(t *testing.T) {
		require.Equal(t, "us-seattle-1", client.GetRegion())
	})
	t.Run("realm", func(t *testing.T) {
		require.Equal(t, "region1", client.GetRealm())
	})
	t.Run("airport code", func(t *testing.T) {
		require.Equal(t, "XXP", client.GetAirportCode())
	})
	t.Run("service enclave suffix", func(t *testing.T) {
		require.Equal(t, "svc.ad1.sol-phoebe-1", client.GetSvcEnclaveSuffix())
	})
	t.Run("identity realm", func(t *testing.T) {
		require.Equal(t, "rb1", client.GetIdentityRealm())
	})
	t.Run("ldap realm", func(t *testing.T) {
		require.Equal(t, "r1", client.GetLdapifiedRealm())
	})
	t.Run("ldap region", func(t *testing.T) {
		require.Equal(t, "r1", client.GetLdapifiedRegion())
	})
	t.Run("test default ldap realm region", func(t *testing.T) {
		modClient, err := NewMetadataClient(config, fs)
		require.NoError(t, err)
		modClient.metadata.Region = "sol-phoebe-1"
		require.Equal(t, "sol-phoebe-1", modClient.GetLdapifiedRegion())

		modClient.metadata.Realm = "rb1"
		require.Equal(t, "rb1", modClient.GetLdapifiedRealm())
	})
}
