package casper

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// ProvideCasperDataStore initializes a single CasperDataStore using viper configuration,
// environment context, and a logger. It is intended to be used as an fx provider.
//
// Returns:
//   - a pointer to CasperDataStore if initialization is successful
//   - an error if configuration loading or initialization fails
func ProvideCasperDataStore(v *viper.Viper, logger logging.Interface) (*CasperDataStore, error) {
	config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
	if err != nil {
		return nil, fmt.Errorf("error reading download agent config: %w", err)
	}
	return NewCasperDataStore(config)
}

// CasperDataStoreModule is an fx module that provides a singleton CasperDataStore.
// It wires ProvideCasperDataStore into the fx dependency graph.
var CasperDataStoreModule = fx.Provide(
	ProvideCasperDataStore,
)

// appParams defines the fx input struct for dependency injection.
// It demonstrates two advanced fx features:
//   - Named logger injection using `name:"another_log"`
//   - Config list injection using fx.ValueGroup (`group:"casperConfigs"`)
type appParams struct {
	fx.In

	// AnotherLogger is an injected named logger (useful if multiple loggers exist).
	AnotherLogger logging.Interface `name:"another_log"`

	// Configs is a list of Casper configuration instances injected via value group.
	Configs []*Config `group:"casperConfigs"`
}

// ProvideListOfCasperDataStoreWithAppParams initializes a list of CasperDataStore instances
// from a group of configuration values (fx.ValueGroup).
//
// This is useful when multiple CasperDataStores need to be constructed and operated in parallel.
//
// Parameters:
//   - e: application-wide environment context
//   - params: injected struct containing config list and logger
//
// Returns:
//   - a list of initialized CasperDataStore pointers
//   - an error if any store fails to initialize (but partial list is returned)
func ProvideListOfCasperDataStoreWithAppParams(params appParams) ([]*CasperDataStore, error) {
	casperDataStoreList := make([]*CasperDataStore, 0)
	for _, config := range params.Configs {
		// Skip nil configs to avoid panics
		if config == nil {
			continue
		}
		casperDataStore, err := NewCasperDataStore(config)
		if err != nil {
			return casperDataStoreList, fmt.Errorf(
				"error initializing CasperDataStore using CasperConfig %+v: %+v", config, err,
			)
		}
		casperDataStoreList = append(casperDataStoreList, casperDataStore)
	}
	return casperDataStoreList, nil
}
