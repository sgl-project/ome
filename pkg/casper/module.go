package casper

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideCasperDataStore(v *viper.Viper, e *env.Environment, logger logging.Interface) (*CasperDataStore, error) {
	config, err := NewConfig(WithViper(v), WithEnv(e), WithAnotherLog(logger))
	if err != nil {
		return nil, fmt.Errorf("error reading download agent config: %w", err)
	}
	return NewCasperDataStore(config, e)
}

var CasperDataStoreModule = fx.Provide(
	ProvideCasperDataStore,
)

type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*Config `group:"casperConfigs"`
}

func ProvideListOfCasperDataStoreWithAppParams(e *env.Environment, params appParams) ([]*CasperDataStore, error) {
	casperDataStoreList := make([]*CasperDataStore, 0)
	for _, config := range params.Configs {
		// Skip when config is nil
		if config == nil {
			continue
		}
		casperDataStore, err := NewCasperDataStore(config, e)
		if err != nil {
			return casperDataStoreList, fmt.Errorf("error initializing CasperDataStore using CasperConfig %+v: %+v", config, err)
		}
		casperDataStoreList = append(casperDataStoreList, casperDataStore)
	}
	return casperDataStoreList, nil
}
