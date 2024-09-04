package internal_test

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

func ExampleNew() {
	environment := env.New(
		env.WithResolvedVars(env.Vars{
			vars.Realm:  "custom-realm",
			vars.Region: "custom-region",
		}),
		env.WithIsGov(true),
		env.WithIsONSR(false),
		env.WithIsOverlayBastion(false),
		env.WithIsTouchEnforcedForRealm(false),
	)

	fmt.Println(environment)
}

func ExampleFromResolver() {
	environment, err := env.FromResolver(
		env.WithResolverDefaults(),
		env.WithResolverLogger(logging.NewTestLogger()),
		env.WithResolverFs(afero.NewMemMapFs()),
	)
	panicIf(err)

	// use it
	fmt.Println(environment)
}

func TestAPI_FromResolver_Viper(t *testing.T) {
	v := viper.New()
	require.NoError(t, configutils.ResolveAndMergeFile(v, "testdata/conf.yaml"))

	e, err := env.FromResolver(
		env.WithResolverDefaults(),
		env.WithResolverFromViper(v, ""),
	)
	require.NoError(t, err)

	assert.Equal(t, "oc2/us-langley-1/ad3", env.TryResolve(e, "${realm}/${region}/${ad}"))
	assert.Equal(t, ".10x", env.TryResolve(e, "${govExtension}"))
	assert.Equal(t, "oraclegovcloud.com", env.TryResolve(e, "${realmTLD}"))
	assert.Equal(t, true, e.IsGov(), "isGov should be true")
	assert.Equal(t, false, e.IsONSR(), "isONSR should be false")
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
