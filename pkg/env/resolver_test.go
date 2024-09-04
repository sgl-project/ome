package env

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

type fakeVarResolver map[vars.Var]string

func (fvr fakeVarResolver) Resolve(v vars.Var) (string, error) {
	vv, ok := fvr[v]
	if !ok {
		return "", fmt.Errorf("fake not found: %v", v)
	}

	return vv, nil
}

func (fvr fakeVarResolver) CanResolve() []vars.Var {
	var result []vars.Var
	for k := range fvr {
		result = append(result, k)
	}

	return result
}

func TestResolveEnvironment(t *testing.T) {
	config := &ResolverConfig{
		CanonicalRegionNames: map[string]string{
			"sea": "r1",
			"r2":  "us-phoenix-1",
			"phx": "us-phoenix-1",
			"pia": "us-gov-chicago-1",
		},
		RealmConfigs: map[string]*RealmConfig{
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
			"bastion-ob-",
		},
		TouchEnforcedInRealms: DefaultTouchEnforcedInRealms(),
	}

	t.Run("resolve environment for cavium in phx", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region: "phx",
			vars.Ad:     "ad1",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, "us-phoenix-1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "prod", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, "ad1", resolvedEnv.resolved[vars.Ad])
		assert.Equal(t, false, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedForRealm())
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedOnOverlayBastions())
		assert.Equal(t, "", resolvedEnv.resolved[vars.GovExtension])
	})

	t.Run("resolve environment for non-cavium in r1", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region: "r1",
			vars.Ad:     "ad2",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, false, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedForRealm())
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedOnOverlayBastions())
		assert.Equal(t, "r1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "region1", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, "ad2", resolvedEnv.resolved[vars.Ad])
		assert.Equal(t, "", resolvedEnv.resolved[vars.GovExtension])
	})

	t.Run("missing realm/region/ad doesn't fail", func(t *testing.T) {
		fake := fakeVarResolver{}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)

		fmt.Println(resolvedEnv)
	})

	t.Run("resolve environment for cavium in gov region", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region: "pia",
			vars.Ad:     "ad2",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, "us-gov-chicago-1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "oc3", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, true, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedOnOverlayBastions()) // not an overlay bastion
		assert.Equal(t, true, resolvedEnv.IsTouchEnforcedForRealm())           // exposed to ldap-updater
		assert.Equal(t, ".10x", resolvedEnv.resolved[vars.GovExtension])
	})

	t.Run("resolve environment for overlay in r1", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region:                "r1",
			vars.Ad:                    "ad1",
			vars.InstanceCompartmentId: "ocid1.tenancy.r1..aaaaaawida",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)

		assert.Equal(t, "r1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "region1", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, false, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, "", resolvedEnv.resolved[vars.GovExtension])
		assert.Equal(t, "ocid1.tenancy.r1..aaaaaawida", resolvedEnv.resolved[vars.InstanceCompartmentId])
	})

	t.Run("resolve environment for gov overlay bastion in OC3", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region:                "us-gov-chicago-1",
			vars.Ad:                    "ad1",
			vars.InstanceCompartmentId: "ocid1.tenancy.oc3..aaaaaawida",
			vars.Hostclass:             "bastion-ob-oc3",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, "us-gov-chicago-1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "oc3", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, true, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, ".10x", resolvedEnv.resolved[vars.GovExtension])
		assert.Equal(t, "ocid1.tenancy.oc3..aaaaaawida", resolvedEnv.resolved[vars.InstanceCompartmentId])
		assert.Equal(t, "bastion-ob-oc3", resolvedEnv.resolved[vars.Hostclass])
		assert.Equal(t, true, resolvedEnv.isOverlayBastion)
		assert.Equal(t, true, resolvedEnv.IsTouchEnforcedForRealm())
		assert.Equal(t, true, resolvedEnv.IsTouchEnforcedOnOverlayBastions())
	})

	t.Run("resolve environment for overlay bastion in R1", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region:                "r1",
			vars.Ad:                    "ad1",
			vars.InstanceCompartmentId: "ocid1.tenancy.r1..aaaaaawida",
			vars.Hostclass:             "bastion-non-gov",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, "r1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "region1", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, false, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, "", resolvedEnv.resolved[vars.GovExtension])
		assert.Equal(t, "ocid1.tenancy.r1..aaaaaawida", resolvedEnv.resolved[vars.InstanceCompartmentId])
		assert.Equal(t, "bastion-non-gov", resolvedEnv.resolved[vars.Hostclass])
		assert.Equal(t, false, resolvedEnv.isOverlayBastion)
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedForRealm())
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedOnOverlayBastions())
	})

	t.Run("isOverlayBastion resolves to false when HostClass not resolved", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region:    "r1",
			vars.Ad:        "ad1",
			vars.Hostclass: "bastion-something",
		}
		resolvedEnv, err := mustMakeResolver(config, fake).Resolve()

		assert.NoError(t, err)
		assert.Equal(t, "r1", resolvedEnv.resolved[vars.Region])
		assert.Equal(t, "region1", resolvedEnv.resolved[vars.Realm])
		assert.Equal(t, false, resolvedEnv.isGov)
		assert.Equal(t, false, resolvedEnv.isONSR)
		assert.Equal(t, "", resolvedEnv.resolved[vars.GovExtension])
		assert.Equal(t, "bastion-something", resolvedEnv.resolved[vars.Hostclass])
		assert.Equal(t, false, resolvedEnv.isOverlayBastion)
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedForRealm())
		assert.Equal(t, false, resolvedEnv.IsTouchEnforcedOnOverlayBastions())
	})
}

func TestResolve_RealmTLD(t *testing.T) {
	testCases := []struct {
		realm                      Realm
		realmTLD, internalRealmTLD string
	}{
		{
			realm:            Region1,
			realmTLD:         "oracleiaas.com",
			internalRealmTLD: "oracleiaas.com",
		},
		{
			realm:            OC0,
			realmTLD:         "oraclecloud0.com",
			internalRealmTLD: "oraclerealm0.com",
		},
		{
			realm:            OC1,
			realmTLD:         "oraclecloud.com",
			internalRealmTLD: "oracleiaas.com",
		},
		{
			realm:            OC2,
			realmTLD:         "oraclegovcloud.com",
			internalRealmTLD: "oraclegoviaas.com",
		},
		{
			realm:            OC3,
			realmTLD:         "oraclegovcloud.com",
			internalRealmTLD: "oraclegoviaas.com",
		},
		{
			realm:            OC4,
			realmTLD:         "oraclegovcloud.uk",
			internalRealmTLD: "oraclegoviaas.uk",
		},
		{
			realm:            OC5,
			realmTLD:         "oraclecloud5.com",
			internalRealmTLD: "oraclerealm5.com",
		},
		{
			realm:            OC6,
			realmTLD:         "oraclecloud.ic.gov",
			internalRealmTLD: "oraclerealm.ic.gov",
		},
		{
			realm:            OC7,
			realmTLD:         "oc.ic.gov",
			internalRealmTLD: "oci.ic.gov",
		},
		{
			realm:            OC8,
			realmTLD:         "oraclecloud8.com",
			internalRealmTLD: "oraclerealm8.com",
		},
		{
			realm:            OC9,
			realmTLD:         "oraclecloud9.com",
			internalRealmTLD: "oraclerealm9.com",
		},
		{
			realm:            OC10,
			realmTLD:         "oraclecloud10.com",
			internalRealmTLD: "oraclerealm10.com",
		},
		{
			realm:            OC11,
			realmTLD:         "oraclecloud.smil.mil",
			internalRealmTLD: "oraclerealm.smil.mil",
		},
		{
			realm:            OC12,
			realmTLD:         "oracledodcloud.ic.gov",
			internalRealmTLD: "oracledodrealm.ic.gov",
		},
		{
			realm:            OC14,
			realmTLD:         "oraclecloud14.com",
			internalRealmTLD: "oraclerealm14.com",
		},
		{
			realm:            OC16,
			realmTLD:         "oraclecloud16.com",
			internalRealmTLD: "oraclerealm16.com",
		},
		{
			realm:            OC17,
			realmTLD:         "oraclecloud17.com",
			internalRealmTLD: "oraclerealm17.com",
		},
		{
			realm:            OC18,
			realmTLD:         "oraclecloud18.com",
			internalRealmTLD: "oraclerealm18.com",
		},
		{
			realm:            OC19,
			realmTLD:         "oraclecloud.eu",
			internalRealmTLD: "oraclerealm.eu",
		},
		{
			realm:            OC20,
			realmTLD:         "oraclecloud20.com",
			internalRealmTLD: "oraclerealm20.com",
		},
		{
			realm:            "foobar",
			realmTLD:         "<realmTLD not resolved>",
			internalRealmTLD: "<internalRealmTLD not resolved>",
		},
	}

	for _, tc := range testCases {
		testFn := func(realm string) func(t *testing.T) {
			return func(t *testing.T) {
				fake := fakeVarResolver{
					vars.Region: "foobar", // to remove warnings from test output
					vars.Ad:     "ad1",    // to remove warnings from test output
					vars.Realm:  realm,
				}

				resolvedEnv, err := mustMakeResolver(&ResolverConfig{}, fake).Resolve()
				require.NoError(t, err, "should not have failed")

				realmTLD, err := resolvedEnv.Resolve(vars.RealmTLD.String())
				require.NoError(t, err)
				require.Equal(t, tc.realmTLD, realmTLD)

				// test fallback option when var.InternalRealmTLD resolver is not provided
				internalRealmTLD, err := resolvedEnv.Resolve(vars.InternalRealmTLD.String())
				require.NoError(t, err)
				require.Equal(t, tc.internalRealmTLD, internalRealmTLD)
			}
		}

		realmLowercase := tc.realm.Name()
		for _, realmKey := range []string{
			realmLowercase,
			strings.ToUpper(realmLowercase),
		} {
			t.Run(realmKey, testFn(realmKey))
		}
	}

	t.Run("env_override", func(t *testing.T) {
		fake := fakeVarResolver{
			vars.Region:           "invalid", // to remove warnings from test output
			vars.Ad:               "invalid", // to remove warnings from test output
			vars.Realm:            "invalid", // to remove warnings from test output
			vars.RealmTLD:         "foobar.tld",
			vars.InternalRealmTLD: "foobar.itld",
		}

		resolvedEnv, err := mustMakeResolver(&ResolverConfig{}, fake).Resolve()
		require.NoError(t, err, "should not have failed")

		realmTLD, err := resolvedEnv.Resolve(vars.RealmTLD.String())
		require.NoError(t, err)
		require.Equal(t, "foobar.tld", realmTLD)

		internalRealmTLD, err := resolvedEnv.Resolve(vars.InternalRealmTLD.String())
		require.NoError(t, err)
		require.Equal(t, "foobar.itld", internalRealmTLD)
	})
}

func TestResolve_RegionSE(t *testing.T) {
	testCases := []struct {
		region, regionSE string
	}{
		{region: "us-seattle-1", regionSE: "r1"},
		{region: "r1", regionSE: "r1"},
		{region: "sea", regionSE: "r1"},

		{region: "us-phoenix-1", regionSE: "r2"},
		{region: "r2", regionSE: "r2"},
		{region: "phx", regionSE: "r2"},

		{region: "us-tacoma-1", regionSE: "us-tacoma-1"},
	}

	for _, tc := range testCases {
		t.Run(tc.region, func(t *testing.T) {
			fake := fakeVarResolver{
				vars.Region: tc.region, // to remove warnings from test output
				vars.Ad:     "ad1",     // to remove warnings from test output
			}

			resolvedEnv, err := mustMakeResolver(&ResolverConfig{}, fake).Resolve()
			require.NoError(t, err, "should not have failed")

			regionSE, err := resolvedEnv.Resolve(vars.RegionSE.String())
			require.NoError(t, err)
			require.Equal(t, tc.regionSE, regionSE)
		})
	}
}

func TestResolve_OverlayBastions(t *testing.T) {
	config := &ResolverConfig{
		OverlayBastionHostclassRealmPrefixes: DefaultOverlayBastionHostclassRealmPrefixes(),
		OverlayBastionHostclasses:            DefaultOverlayBastionHostclasses(),
	}

	realm := "REALM"
	testCases := []struct {
		expected  bool
		hostclass string
	}{
		// Legacy OB3
		{expected: true, hostclass: "overlay-bastion-" + strings.ToLower(realm)},
		{expected: true, hostclass: "overlay-bastion-" + realm},
		{expected: true, hostclass: "OVERLAY-BASTION-" + realm},
		{expected: true, hostclass: "bastion-ob-" + realm},

		// New internal bastions with tunnel only capability
		{expected: true, hostclass: "bastion-internal-host"},
		{expected: true, hostclass: "BASTION-INTERNAL-HOST"},
		{expected: true, hostclass: "bastion-internal-host-test"},
		{expected: true, hostclass: "ztb-internal-host-test"},
		{expected: true, hostclass: "ztb-internal-host"},

		// Failures
		{expected: false, hostclass: "bastion-internal-host-" + realm},
		{expected: false, hostclass: "bastion-internal-host" + realm},
		{expected: false, hostclass: "bastion-internal-host-test" + realm},
		{expected: false, hostclass: "bastion-internal-host-test-" + realm},
		{expected: false, hostclass: "cerberus"},
		{expected: false, hostclass: "cerberus-" + realm},
		{expected: false, hostclass: "permissions-worker" + realm},
		{expected: false, hostclass: "prefixed-bastion-internal-host"},
		{expected: false, hostclass: "prefixed-overlay-bastion-host-" + realm},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf(tc.hostclass)
		t.Run(name, func(t *testing.T) {
			fake := fakeVarResolver{
				vars.Region:    "r1", // whatever value works to omit warnings
				vars.Ad:        "ad1",
				vars.Realm:     realm,
				vars.Hostclass: tc.hostclass,
			}

			resolvedEnv, err := mustMakeResolver(config, fake).Resolve()
			require.NoError(t, err, "should not have failed")
			assert.Equal(t, tc.expected, resolvedEnv.IsOverlayBastion())
		})
	}
}

func TestConfig_IsTouchEnforced(t *testing.T) {
	config := &ResolverConfig{
		TouchEnforcedInRealms: []string{"oc2", "oc3"},
	}

	testCases := []struct {
		realm    string
		expected bool
	}{
		{realm: "region1", expected: false},
		{realm: "oc1", expected: false},
		{realm: "oc2", expected: true},
		{realm: "oc3", expected: true},
		{realm: "oc4", expected: false},
		{realm: "oc5", expected: false},
		{realm: "oc6", expected: false},
		{realm: "oc7", expected: false}, // intentionally set to false to exercise the config
	}
	for _, tc := range testCases {
		t.Run(tc.realm, func(t *testing.T) {
			fake := fakeVarResolver{
				vars.Region: "r1", // whatever value works to omit warnings
				vars.Ad:     "ad1",
				vars.Realm:  tc.realm,
			}

			resolvedEnv, err := mustMakeResolver(config, fake).Resolve()
			require.NoError(t, err, "should not have failed")
			assert.Equal(t, tc.expected, resolvedEnv.isTouchEnabledForRealm)
		})
	}
}

func TestResolve_Ad_Pop(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{
		{input: "ad1", output: "ad1"},
		{input: "pop1", output: "ad1"},
	}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			fake := fakeVarResolver{
				vars.Ad:     tc.input,
				vars.Region: "whatever", // to remove warnings from test output
				vars.Realm:  "whatever", // to remove warnings from test output
			}

			resolvedEnv, err := mustMakeResolver(&ResolverConfig{}, fake).Resolve()
			require.NoError(t, err, "should not have failed")

			actualAd, err := resolvedEnv.Resolve(vars.Ad.String())
			require.NoError(t, err)
			require.Equal(t, tc.output, actualAd)
		})
	}
}

func mustMakeResolver(config *ResolverConfig, varResolvers ...vars.Resolver) *resolver {
	config.logger = newTestLogger()
	r, err := newResolverUsing(config, varResolvers)
	if err != nil {
		panic(err)
	}

	return r
}

func newTestLogger() logging.Interface {
	return logging.ForLogrus(logrus.NewEntry(logrus.New()))
}

func TestMapToString(t *testing.T) {
	mp := map[vars.Var]string{
		vars.MustNewVar("realm", false):      "realm1",
		vars.MustNewVar("region", false):     "phx",
		vars.MustNewVar("tenancyOcid", true): "ocid1...tenancy.blabla",
	}

	expected := "realm: realm1, region: phx, tenancyOcid: ocid1...tenancy.blabla"
	require.Equal(t, expected, mapToString(mp))
}
