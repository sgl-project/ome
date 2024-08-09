package env

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
)

func newFakeEnv(realm, region string) Interface {
	return New(WithResolvedVars(Vars{
		vars.Realm:  realm,
		vars.Region: region,
	}))
}

func TestMapBy_Resolve(t *testing.T) {
	t.Run("region", func(t *testing.T) {
		m := StringByRegion{
			"r1":           "r1-branch",
			"us-seattle-1": "sea-branch",
			"default":      "${region}",
		}

		tcs := []struct {
			env    Interface
			result string
		}{
			{
				env:    newFakeEnv("oc0", "us-seattle-1"),
				result: "sea-branch",
			},
			{
				env:    newFakeEnv("oc0", "r1"),
				result: "r1-branch",
			},
			{
				env:    newFakeEnv("oc0", "yo"),
				result: "yo",
			},
		}

		for _, tc := range tcs {
			t.Run(fmt.Sprintf("%+v", tc.env), func(t *testing.T) {
				result, err := m.Resolve(tc.env)
				require.NoError(t, err)
				require.Equal(t, tc.result, result)
			})
		}
	})
	t.Run("realm", func(t *testing.T) {
		m := StringByRealm{
			"region1": "r1-branch",
			"oc1":     "oc1-branch.${region}",
			"default": "${region}.${realm}",
		}

		tcs := []struct {
			env    Interface
			result string
		}{
			{
				env:    newFakeEnv("region1", "us-seattle-1"),
				result: "r1-branch",
			},
			{
				env:    newFakeEnv("oc1", "phx"),
				result: "oc1-branch.phx",
			},
			{
				env:    newFakeEnv("oc1", "us-phoenix-1"),
				result: "oc1-branch.us-phoenix-1",
			},
			{
				env:    newFakeEnv("oc8", "ap-chiyoda-1"),
				result: "ap-chiyoda-1.oc8",
			},
		}

		for _, tc := range tcs {
			t.Run(fmt.Sprintf("%+v", tc.env), func(t *testing.T) {
				result, err := m.Resolve(tc.env)
				require.NoError(t, err)
				require.Equal(t, tc.result, result)
			})
		}
	})
}

func TestMapBy_ResolveIfExists(t *testing.T) {
	t.Run("region", func(t *testing.T) {
		m := StringByRegion{
			"r1":           "r1-branch",
			"us-seattle-1": "sea-branch",
			"default":      "${region}",
		}

		tcs := []struct {
			env    Interface
			result string
		}{
			{
				env:    newFakeEnv("oc0", "us-seattle-1"),
				result: "sea-branch",
			},
			{
				env:    newFakeEnv("oc0", "r1"),
				result: "r1-branch",
			},
			{
				env:    newFakeEnv("oc0", "yo"),
				result: "", // if exists doesn't default to "default" case
			},
		}

		for _, tc := range tcs {
			t.Run(fmt.Sprintf("%+v", tc.env), func(t *testing.T) {
				result, err := m.ResolveIfExists(tc.env)
				require.NoError(t, err)
				require.Equal(t, tc.result, result)
			})
		}
	})
	t.Run("realm", func(t *testing.T) {
		m := StringByRealm{
			"region1": "r1-branch",
			"oc1":     "oc1-branch.${region}",
			"default": "${region}.${realm}",
		}

		tcs := []struct {
			env    Interface
			result string
		}{
			{
				env:    newFakeEnv("region1", "us-seattle-1"),
				result: "r1-branch",
			},
			{
				env:    newFakeEnv("oc1", "phx"),
				result: "oc1-branch.phx",
			},
			{
				env:    newFakeEnv("oc1", "us-phoenix-1"),
				result: "oc1-branch.us-phoenix-1",
			},
			{
				env:    newFakeEnv("oc8", "ap-chiyoda-1"),
				result: "", // if exists doesn't default to "default" case
			},
		}

		for _, tc := range tcs {
			t.Run(fmt.Sprintf("%+v", tc.env), func(t *testing.T) {
				result, err := m.ResolveIfExists(tc.env)
				require.NoError(t, err)
				require.Equal(t, tc.result, result)
			})
		}
	})
}
