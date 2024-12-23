package env_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/imds"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
)

func newValidResolverConfig() *env.ResolverConfig {
	c := &env.ResolverConfig{
		CanonicalRegionNames: map[string]string{
			"sea": "r1",
			"r2":  "us-phoenix-1",
			"phx": "us-phoenix-1",
			"pia": "us-gov-chicago-1",
		},
		RealmConfigs: map[string]*env.RealmConfig{
			"region1": {
				Regions: []string{"r1"},
			},
			"prod": {
				Regions: []string{"us-phoenix-1"},
			},
			"oc3": {
				Regions: []string{"us-gov-chicago-1"},
				IsGov:   true,
			},
		},
		OverlayBastionHostclassRealmPrefixes: []string{
			"bastion-ob-",
			"overlay-bastion-",
		},
		OverlayBastionHostclasses: []string{
			"bastion-internal-host",
			"bastion-internal-host-test",
		},
		ResolveVarsWith: []vars.ResolverKind{vars.Local},
	}
	return c
}

func TestConfig_Validate(t *testing.T) {
	c := newValidResolverConfig()

	t.Run("happy-case", func(t *testing.T) {
		assert.NoError(t, c.Validate())
	})

	t.Run("nil", func(t *testing.T) {
		var c *env.ResolverConfig
		assert.Error(t, c.Validate(), "should fail validation if nil")
	})

	t.Run("realm-config-is-nil", func(t *testing.T) {
		c := *c // shallow copy
		c.RealmConfigs["x"] = nil
		assert.Error(t, c.Validate(), "should fail validation")
	})
	t.Run("realm-config-is-empty", func(t *testing.T) {
		c := *c // shallow copy, but RealmConfigs are a reference
		c.RealmConfigs["x"] = &env.RealmConfig{
			Regions: nil,
		}
		defer func() {
			delete(c.RealmConfigs, "x")
		}()
		assert.Error(t, c.Validate(), "should fail validation")
	})
	t.Run("imds enabled", func(t *testing.T) {
		c := *c // shallow copy
		c.ResolveVarsWith = []vars.ResolverKind{vars.IMDS}
		c.IMDS.BaseEndpoint = "" // this should cause the failure
		assert.Error(t, c.Validate(), "should fail validation")
	})
	t.Run("imds disabled", func(t *testing.T) {
		c := *c // shallow copy
		c.ResolveVarsWith = []vars.ResolverKind{vars.Local}
		c.IMDS.BaseEndpoint = "" // this should _not_ cause the failure
		assert.NoError(t, c.Validate(), "should fail validation")
	})
}

func TestConfig_OverlayBastionHostclasses(t *testing.T) {
	c := newValidResolverConfig()

	testCases := []struct {
		name   string
		getter func(*env.ResolverConfig) []string
		setter func(*env.ResolverConfig, []string)
	}{
		{
			name:   "overlay_bastion_hostclass_prefixes",
			getter: func(c *env.ResolverConfig) []string { return c.OverlayBastionHostclassRealmPrefixes },
			setter: func(c *env.ResolverConfig, v []string) { c.OverlayBastionHostclassRealmPrefixes = v },
		},
		{
			name:   "overlay_bastion_hostclasses",
			getter: func(c *env.ResolverConfig) []string { return c.OverlayBastionHostclasses },
			setter: func(c *env.ResolverConfig, v []string) { c.OverlayBastionHostclasses = v },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("empty value not fine", func(t *testing.T) {
				c := *c // shallow copy
				tc.setter(&c, append(tc.getter(&c), ""))
				assert.Error(t, c.Validate(), "should fail validation")
			})
			t.Run("upper case is not fine", func(t *testing.T) {
				c := *c // shallow copy
				tc.setter(&c, append(tc.getter(&c), "YoLo"))
				assert.Error(t, c.Validate(), "should fail validation")
			})
			t.Run("lowercase is fine", func(t *testing.T) {
				c := *c // shallow copy
				tc.setter(&c, []string{"yolo"})
				assert.NoError(t, c.Validate(), "should not fail validation")
			})
			t.Run("empty list is fine though", func(t *testing.T) {
				c := *c // shallow copy
				tc.setter(&c, []string{})
				assert.NoError(t, c.Validate(), "should not fail validation")
			})
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	c := &env.ResolverConfig{}
	err := env.WithResolverDefaults()(c)
	require.NoError(t, err)

	require.Equal(t, []vars.ResolverKind{vars.Local}, c.ResolveVarsWith)
	require.Equal(t, imds.DefaultConfig(), c.IMDS)
}
