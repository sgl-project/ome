package casper

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideCasperDataStore(v *viper.Viper, e *env.Environment, logger logging.Interface, viperKeyNames []string) (*CasperDataStore, error) {
	config, err := NewConfig(WithViper(v, viperKeyNames), WithEnv(e), WithAnotherLog(logger))
	if err != nil {
		return nil, fmt.Errorf("error reading download agent config: %w", err)
	}
	return NewCasperDataStore(config, e)
}

var Module = fx.Provide(
	ProvideCasperDataStore,
)
