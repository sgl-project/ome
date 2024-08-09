package env

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
)

func TestResolve(t *testing.T) {
	e := New(WithResolvedVars(Vars{
		vars.Realm:                 "myRealm",
		vars.Ad:                    "myAd",
		vars.Region:                "myRegion", // note: there is no vars.RegionSE - it's computed on-the-fly
		vars.GovExtension:          ".10x",
		vars.Hostclass:             "myHostclass",
		vars.InstanceCompartmentId: "ocid1-dsafdsa-adsfsadfdsaf-dsfafdsa",
	}))

	t.Run("resolve methods from resolved map", func(t *testing.T) {
		realm, ok := e.Realm()
		assert.True(t, ok)
		assert.Equal(t, "myRealm", realm)

		region, ok := e.Region()
		assert.True(t, ok)
		assert.Equal(t, "myRegion", region)

		ad, ok := e.Ad()
		assert.True(t, ok)
		assert.Equal(t, "myAd", ad)
	})

	t.Run("expand vars within string", func(t *testing.T) {
		result, err := e.Resolve("${realm}/${region}/${regionSE}/${ad}/${govExtension}/${hostclass}/${instanceCompartmentID}")
		assert.NoError(t, err)
		assert.Equal(t, "myRealm/myRegion/myRegion/myAd/.10x/myHostclass/ocid1-dsafdsa-adsfsadfdsaf-dsfafdsa", result)
	})

	t.Run("expand nested vars should fail", func(t *testing.T) {
		_, err := e.Resolve("${realm${region}}")
		require.Error(t, err)
		fmt.Println(err)
		_, err = e.Resolve("${region${realm}}")
		require.Error(t, err)
		fmt.Println(err)

		_, err = e.Resolve("${${region}realm}")
		require.Error(t, err)
		fmt.Println(err)
		_, err = e.Resolve("${${realm}region}")
		require.Error(t, err)
		fmt.Println(err)
	})

	t.Run("doesn't expand var when no value is provided", func(t *testing.T) {
		e := New(WithResolvedVars(Vars{
			vars.Realm:  "myRealm",
			vars.Ad:     "myAd",
			vars.Region: "myRegion",
		}))

		result, err := e.Resolve("${instanceCompartmentID}")
		assert.Error(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("expands var when empty value is provided", func(t *testing.T) {
		e := New(WithResolvedVars(Vars{vars.GovExtension: ""}))

		result, err := e.Resolve("access${govExtension}.tar.gz")
		assert.NoError(t, err)
		assert.Equal(t, "access.tar.gz", result)
	})

	t.Run("touch-enforced-overlay-bastions", func(t *testing.T) {
		// One might argue it's not worth for a simple boolean AND operation
		// to be tested, but I decided to put it here anyways to establish
		// the contract with the bastion team as concretely as possible.
		testCases := []struct {
			touchEnabled, overlayBastion bool
			expected                     bool
		}{
			{touchEnabled: true, overlayBastion: true, expected: true},
			{touchEnabled: false, overlayBastion: true, expected: false},
			{touchEnabled: true, overlayBastion: false, expected: false},
			{touchEnabled: false, overlayBastion: false, expected: false},
		}

		for _, tc := range testCases {
			name := fmt.Sprintf("%v_%v", tc.touchEnabled, tc.overlayBastion)
			t.Run(name, func(t *testing.T) {
				e := New(
					WithIsTouchEnforcedForRealm(tc.touchEnabled),
					WithIsOverlayBastion(tc.overlayBastion),
				)

				assert.Equal(t, tc.expected, e.IsTouchEnforcedOnOverlayBastions())
			})
		}
	})
	t.Run("touch-enforced-independent-overlay-bastions", func(t *testing.T) {
		// One might argue it's not worth for a simple boolean AND operation
		// to be tested, but I decided to put it here anyways to establish
		// the contract with the bastion team as concretely as possible.
		testCases := []struct {
			touchEnabled, overlayBastion bool
			expected                     bool
		}{
			{touchEnabled: true, overlayBastion: true, expected: true},
			{touchEnabled: false, overlayBastion: true, expected: false},
			{touchEnabled: true, overlayBastion: false, expected: true},
			{touchEnabled: false, overlayBastion: false, expected: false},
		}

		for _, tc := range testCases {
			name := fmt.Sprintf("%v_%v", tc.touchEnabled, tc.overlayBastion)
			t.Run(name, func(t *testing.T) {
				e := New(
					WithIsTouchEnforcedForRealm(tc.touchEnabled),
					WithIsOverlayBastion(tc.overlayBastion),
				)

				assert.Equal(t, tc.expected, e.IsTouchEnforcedForRealm())
			})
		}
	})
}
