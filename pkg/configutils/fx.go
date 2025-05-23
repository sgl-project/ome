package configutils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// ProvideViperFromFile provides an fx module for creating viper instance
// from the specified config file.
func ProvideViperFromFile(envPrefix string, pflags *pflag.FlagSet, configFilePath string) fx.Option {
	return fx.Provide(func() (*viper.Viper, error) {
		v := viper.New()

		v.SetEnvPrefix(envPrefix)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		if configFilePath == "" {
			return nil, errors.New("no config file provided")
		}

		if pflags != nil {
			if err := v.BindPFlag("debug", pflags.Lookup("debug")); err != nil {
				return nil, fmt.Errorf("can't bind debug flag: %w", err)
			}
		}

		if err := ResolveAndMergeFile(v, configFilePath); err != nil {
			return nil, fmt.Errorf("cannot read config file: %w", err)
		}

		return v, nil
	})
}
