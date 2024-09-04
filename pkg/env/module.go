package env

import (
	"errors"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

var Module fx.Option = fx.Provide(
	provideEnvironment,
	provideEnvironmentInterface,
)

func provideEnvironment(v *viper.Viper, logger logging.Interface, fs afero.Fs) (*Environment, error) {
	return FromResolver(
		WithResolverDefaults(),
		WithResolverFromViper(v, ""),
		WithResolverLogger(logger),
		WithResolverFs(fs),
	)
}

func provideEnvironmentInterface(e *Environment) (Interface, error) {
	if e == nil {
		return nil, errors.New("environment is nil")
	}

	return e, nil
}
