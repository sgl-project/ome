package xet

import (
	"context"
	"fmt"
	"time"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// HubParams represents the parameters that can be injected into the Hub client
type HubParams struct {
	fx.In

	// Logger for hub operations
	Logger logging.Interface `name:"another_log"`
}

var Module = fx.Options(
	fx.Provide(
		func(v *viper.Viper, params HubParams) (*Client, error) {
			config, err := NewConfig(
				WithViper(v),
				WithAppParams(params),
				WithLogger(params.Logger),
				WithDefaults(),
			)
			if err != nil {
				return nil, fmt.Errorf("error creating hub config: %+v", err)
			}

			client, err := NewClient(config)
			if err != nil {
				return nil, fmt.Errorf("error creating xet client: %+v", err)
			}

			if config.EnableProgressReporting {
				// Enable progress reporting for the xet client
				if err := client.EnableConsoleProgress("direct", 250*time.Millisecond); err != nil {
					params.Logger.Warnf("warning: unable to enable progress reporting: %v", err)
				}
			}

			return client, nil
		},
	),
	fx.Invoke(func(lc fx.Lifecycle, client *Client, logger logging.Interface) {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				if err := client.Close(); err != nil {
					logger.Warnf("warning: error closing xet client: %v", err)
				}
				return nil
			},
		})
	}),
)
