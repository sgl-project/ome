package env

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringSliceBy_Resolve(t *testing.T) {
	t.Run("stringSlice", func(t *testing.T) {
		m := StringSlice{"${region}.${realm}", "test-branch"}
		tcs := []struct {
			env    Interface
			result []string
		}{
			{
				env:    newFakeEnv("oc0", "us-seattle-1"),
				result: []string{"us-seattle-1.oc0", "test-branch"},
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

	t.Run("region", func(t *testing.T) {
		m := StringSliceByRegion{
			"r1":           []string{"r1-branch"},
			"us-seattle-1": []string{"sea-branch", "seattle-branch"},
			"default":      []string{"${region}"},
		}

		tcs := []struct {
			env    Interface
			result []string
		}{
			{
				env:    newFakeEnv("oc0", "us-seattle-1"),
				result: []string{"sea-branch", "seattle-branch"},
			},
			{
				env:    newFakeEnv("oc0", "r1"),
				result: []string{"r1-branch"},
			},
			{
				env:    newFakeEnv("oc0", "yo"),
				result: []string{"yo"},
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
		m := StringSliceByRealm{
			"region1": []string{"r1-branch", "sea-branch"},
			"oc1":     []string{"oc1-branch.${region}", "oc1-branch.${region}.other"},
			"default": []string{"${region}.${realm}", "${region}.${realm}.other"},
		}

		tcs := []struct {
			env    Interface
			result []string
		}{
			{
				env:    newFakeEnv("region1", "us-seattle-1"),
				result: []string{"r1-branch", "sea-branch"},
			},
			{
				env:    newFakeEnv("oc1", "phx"),
				result: []string{"oc1-branch.phx", "oc1-branch.phx.other"},
			},
			{
				env:    newFakeEnv("oc1", "us-phoenix-1"),
				result: []string{"oc1-branch.us-phoenix-1", "oc1-branch.us-phoenix-1.other"},
			},
			{
				env:    newFakeEnv("oc8", "ap-chiyoda-1"),
				result: []string{"ap-chiyoda-1.oc8", "ap-chiyoda-1.oc8.other"},
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

func TestStringSliceBy_ResolveIfExists(t *testing.T) {
	t.Run("region", func(t *testing.T) {
		m := StringSliceByRegion{
			"r1":           []string{"r1-branch", "sea-branch"},
			"us-seattle-1": []string{"sea-branch", "${region}-branch"},
			"default":      []string{"${region}", "${region}.other"},
		}

		tcs := []struct {
			env    Interface
			result []string
		}{
			{
				env:    newFakeEnv("oc0", "us-seattle-1"),
				result: []string{"sea-branch", "us-seattle-1-branch"},
			},
			{
				env:    newFakeEnv("oc0", "r1"),
				result: []string{"r1-branch", "sea-branch"},
			},
			{
				env:    newFakeEnv("oc0", "yo"),
				result: nil, // if exists doesn't default to "default" case
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
		m := StringSliceByRealm{
			"region1": []string{"r1-branch", "r1-branch.${region}"},
			"oc1":     []string{"oc1-branch.${region}", "oc1-branch.${region}.other"},
			"default": []string{"${region}.${realm}", "${region}.${realm}.other"},
		}

		tcs := []struct {
			env    Interface
			result []string
		}{
			{
				env:    newFakeEnv("region1", "us-seattle-1"),
				result: []string{"r1-branch", "r1-branch.us-seattle-1"},
			},
			{
				env:    newFakeEnv("oc1", "phx"),
				result: []string{"oc1-branch.phx", "oc1-branch.phx.other"},
			},
			{
				env:    newFakeEnv("oc1", "us-phoenix-1"),
				result: []string{"oc1-branch.us-phoenix-1", "oc1-branch.us-phoenix-1.other"},
			},
			{
				env:    newFakeEnv("oc8", "ap-chiyoda-1"),
				result: nil, // if exists doesn't default to "default" case
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
