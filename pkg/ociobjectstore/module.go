package ociobjectstore

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// ProvideOCIOSDataStore initializes a single OCIOSDataStore using viper configuration,
// environment context, and a logger. It is intended to be used as an fx provider.
//
// Returns:
//   - a pointer to OCIOSDataStore if initialization is successful
//   - an error if configuration loading or initialization fails
func ProvideOCIOSDataStore(v *viper.Viper, logger logging.Interface) (*OCIOSDataStore, error) {
	config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
	if err != nil {
		return nil, fmt.Errorf("error reading download agent config: %w", err)
	}
	return NewOCIOSDataStore(config)
}

// OCIOSDataStoreModule is an fx module that provides a singleton OCIOSDataStore.
// It wires ProvideOCIOSDataStore into the fx dependency graph.
var OCIOSDataStoreModule = fx.Provide(
	ProvideOCIOSDataStore,
)

// appParams defines the fx input struct for dependency injection.
// It demonstrates two advanced fx features:
//   - Named logger injection using `name:"another_log"`
//   - Config list injection using fx.ValueGroup (`group:"casperConfigs"`)
type appParams struct {
	fx.In

	// AnotherLogger is an injected named logger (useful if multiple loggers exist).
	AnotherLogger logging.Interface `name:"another_log"`

	// Configs is a list of Object Storage configuration instances injected via value group.
	Configs []*Config `group:"casperConfigs"`
}

// ProvideListOfOCIOSDataStoreWithAppParams initializes a list of OCIOSDataStore instances
// from a group of configuration values (fx.ValueGroup).
//
// This is useful when multiple CasperDataStores need to be constructed and operated in parallel.
//
// Parameters:
//   - e: application-wide environment context
//   - params: injected struct containing config list and logger
//
// Returns:
//   - a list of initialized OCIOSDataStore pointers
//   - an error if any store fails to initialize (but partial list is returned)
func ProvideListOfOCIOSDataStoreWithAppParams(params appParams) ([]*OCIOSDataStore, error) {
	osDataStoreList := make([]*OCIOSDataStore, 0)
	for _, config := range params.Configs {
		// Skip nil configs to avoid panics
		if config == nil {
			continue
		}
		dataStore, err := NewOCIOSDataStore(config)
		if err != nil {
			return osDataStoreList, fmt.Errorf(
				"error initializing OCIOSDataStore using CasperConfig %+v: %+v", config, err,
			)
		}
		osDataStoreList = append(osDataStoreList, dataStore)
	}
	return osDataStoreList, nil
}
