package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"strings"
)

const (
	CasperConfigViperKeyName = "object_store_config"
)

func configProvider(cli *cobra.Command) fx.Option {
	return fx.Provide(func() (*viper.Viper, error) {
		v := viper.GetViper()

		v.SetDefault("app_name", appName)
		v.SetDefault(casper.CasperConfigViperKeyNameKey, CasperConfigViperKeyName)

		v.SetEnvPrefix(appName)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		v.BindEnv("model_download_directory", "MODEL_DOWNLOAD_DIRECTORY")
		v.BindEnv("model_weight_directory", "MODEL_WEIGHT_DIRECTORY")
		v.BindEnv("is_ft_weights_merged", "IS_FT_WEIGHTS_MERGED")
		v.BindEnv("finetuning_strategy", "FINETUNING_STRATEGY")
		v.BindEnv("model_format", "MODEL_FORMAT")

		v.BindEnv("ft_weight_object_store_uri.namespace", "OS_NAMESPACE")
		v.BindEnv("ft_weight_object_store_uri.bucket_name", "OS_BUCKET")
		v.BindEnv("ft_weight_object_store_uri.object_name", "OS_FINETUNED_WEIGHTS_PATH")

		v.BindEnv(fmt.Sprintf("%s.%s", CasperConfigViperKeyName, casper.AuthTypeViperKeyName), "AUTH_TYPE")
		v.BindEnv(fmt.Sprintf("%s.%s", CasperConfigViperKeyName, casper.CompartmentIdViperKeyName), "COMPARTMENT_ID")

		if err := v.BindPFlag("debug", cli.Flags().Lookup("debug")); err != nil {
			panic(err)
		}

		if configFilePath == "" {
			return nil, errors.New("no config file provided")
		}

		if err := configutils.ResolveAndMergeFile(v, configFilePath); err != nil {
			return nil, fmt.Errorf("cannot read serving-ft config file: %+v", err)
		}

		return v, nil
	})
}
