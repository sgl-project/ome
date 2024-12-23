package vars

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
)

func TestLocal_Resolve_HappyCase(t *testing.T) {
	fs := afero.NewMemMapFs()

	// result should be trimmed
	_ = afero.WriteFile(fs, "/etc/region", []byte("foobarRegion\n\t"), 0)
	_ = afero.WriteFile(fs, "/etc/identity-realm", []byte("foobarRealm\n\t"), 0)
	_ = afero.WriteFile(fs, "/etc/availability-domain", []byte("foobarAd\n\t"), 0)
	_ = afero.WriteFile(fs, "/etc/hostclass", []byte("foobarHostclass\n\t"), 0)
	_ = afero.WriteFile(fs, "/etc/resource-compartment-id", []byte("ocidv1:tenancy:oc1:region:145875596:aaa"+
		"fooBar\n\t"), 0)
	_ = afero.WriteFile(fs, "/etc/exacs-cluster-name", []byte("my_exacs_cluster_name"), 0)
	_ = afero.WriteFile(fs, "/etc/exacs-cluster-ocid", []byte("ocid1.oc1.1.fasdfas12312asdf"), 0)

	r, err := NewLocalResolver(LocalResolverConfig{}, fs)
	require.NoError(t, err, "should not have failed")

	t.Run("region", func(t *testing.T) {
		region, err := r.Resolve(Region)
		assert.NoError(t, err)
		assert.Equal(t, "foobarRegion", region)
	})
	t.Run("realm", func(t *testing.T) {
		realm, err := r.Resolve(Realm)
		assert.NoError(t, err)
		assert.Equal(t, "foobarRealm", realm)
	})
	t.Run("ad", func(t *testing.T) {
		ad, err := r.Resolve(Ad)
		assert.NoError(t, err)
		assert.Equal(t, "foobarAd", ad)
	})
	t.Run("hostclass", func(t *testing.T) {
		hc, err := r.Resolve(Hostclass)
		assert.NoError(t, err)
		assert.Equal(t, "foobarhostclass", hc)
	})
	t.Run("compartment id", func(t *testing.T) {
		compId, err := r.Resolve(ResourceCompartmentId)
		assert.NoError(t, err)
		assert.Equal(t, "ocidv1-tenancy-oc1-region-145875596-aaafooBar", compId)
	})

	t.Run("additional vars", func(t *testing.T) {
		r, err := NewLocalResolver(LocalResolverConfig{
			AdditionalVars: []LocalAdditionalVar{
				{Name: "exacsClusterName", FilePath: "/etc/exacs-cluster-name"},
				{Name: "exacsClusterOcid", FilePath: "/etc/exacs-cluster-ocid", IsOcid: true},
			},
		}, fs)

		require.NoError(t, err, "should not have failed")

		exacsClusterName, err := r.Resolve(MustNewVar("exacsClusterName", false))
		require.NoError(t, err)
		assert.Equal(t, "my_exacs_cluster_name", exacsClusterName)

		exacsClusterOcid, err := r.Resolve(MustNewVar("exacsClusterOcid", true))
		require.NoError(t, err)
		assert.Equal(t, "ocid1-oc1-1-fasdfas12312asdf", exacsClusterOcid)
	})
}

func TestLocal_Resolve_BadCases(t *testing.T) {
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "/etc/region", []byte(""), 0)
	r, err := NewLocalResolver(LocalResolverConfig{}, fs)
	require.NoError(t, err, "should not have failed")

	t.Run("empty file", func(t *testing.T) {
		region, err := r.Resolve(Region)
		assert.Error(t, err)
		assert.Equal(t, "", region)
	})

	t.Run("non-existent file", func(t *testing.T) {
		hc, err := r.Resolve(Hostclass)
		assert.Error(t, err)
		assert.Equal(t, "", hc)
	})

	t.Run("some unknown var", func(t *testing.T) {
		result, err := r.Resolve(InstanceCompartmentId)
		assert.Error(t, err)
		assert.Equal(t, "", result)
	})
}

func TestLocal_Ctor_BadVars(t *testing.T) {
	config := LocalResolverConfig{
		AdditionalVars: []LocalAdditionalVar{
			{Name: "invalid var", FilePath: "/etc/hello"},
		},
	}

	_, err := NewLocalResolver(config, afero.NewMemMapFs())
	require.Error(t, err)
}

func TestLocal_AdditionalVars(t *testing.T) {
	t.Run("cant_redefine_local_builtins", func(t *testing.T) {
		conf := LocalResolverConfig{
			AdditionalVars: []LocalAdditionalVar{
				{
					Name:     Ad.Name(),
					FilePath: "a1",
				},
			},
		}

		_, err := conf.constructAddlVars()
		assert.Error(t, err)
	})

	t.Run("duplicate_not_allowed", func(t *testing.T) {
		conf := LocalResolverConfig{
			AdditionalVars: []LocalAdditionalVar{
				{
					Name:     "a",
					FilePath: "a1",
				},
				{
					Name:     "a",
					FilePath: "a2",
				},
			},
		}

		_, err := conf.constructAddlVars()
		assert.Error(t, err)
	})

	t.Run("from_viper", func(t *testing.T) {
		v := viper.New()
		require.NoError(t, configutils.ResolveAndMergeFile(v, "testdata/local_additional_vars.yaml"))

		conf := LocalResolverConfig{}
		require.NoError(t, v.UnmarshalKey("local", &conf))
		assert.Equal(t, []LocalAdditionalVar{
			{
				Name:     "exacsClusterName",
				IsOcid:   false,
				FilePath: "/etc/exacs-cluster-name",
			},
		}, conf.AdditionalVars)

		vars, err := conf.constructAddlVars()
		assert.NoError(t, err)
		assert.Equal(t, map[Var]string{
			MustNewVar("exacsClusterName", false): "/etc/exacs-cluster-name",
		}, vars)
	})

}
