package logging

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module will load the configuration from Viper and creates a new logger
// using "logging" viper configuration key.
var Module fx.Option = fx.Provide(
	provideZapLogger(ConfigKey),
	provideInterface,
)

// ModuleNamed represents a module for *zap.Logger & logging.Interface that's loaded
// and annotated using a given configKey.
//
// See https://uber-go.github.io/fx/get-started/another-handler.html and
// https://uber-go.github.io/fx/annotate.html#annotating-a-function.
func ModuleNamed(configKey string) fx.Option {
	if configKey == ConfigKey {
		panic("use Module instead of ModuleNamed for root logging")
	}

	// used to annotate params/results of this module
	// and differentiate it from other modules.
	nameTag := fmt.Sprintf(`name:"%s"`, configKey)

	return fx.Provide(
		fx.Annotate(provideZapLogger(configKey),
			fx.ResultTags(nameTag), // ensure this result instance is annotated
		),

		fx.Annotate(provideInterface,
			fx.ParamTags(nameTag),  // ensure we get named logger instead of the global non-annotated one
			fx.ResultTags(nameTag), // ensure this result is also annotated
		),
	)
}

func provideZapLogger(configKey string) func(v *viper.Viper) (*zap.Logger, error) {
	return func(v *viper.Viper) (*zap.Logger, error) {
		desc := ""
		if configKey != ConfigKey {
			desc = fmt.Sprintf(" '%s'", configKey)
		}

		config, err := NewConfig(WithViperKey(v, configKey))
		if err != nil {
			return nil, fmt.Errorf("error reading logging configuration%s: %w", desc, err)
		}
		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid logging configuration%s: %w", desc, err)
		}

		return NewLogger(config)
	}
}

// Temporary - provide this interface until all clients are migrated to zap directly.
func provideInterface(l *zap.Logger) Interface { return ForZap(l) }
