package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestLevel(t *testing.T) {
	t.Run("ParseLevel", func(t *testing.T) {
		cases := map[string]Level{
			"info":  LevelInfo,
			"InFo":  LevelInfo,
			"INFO":  LevelInfo,
			"warn":  LevelWarn,
			"error": LevelError,
			"debug": LevelDebug,
			"":      LevelInfo,
		}

		for in, want := range cases {
			t.Run(in, func(t *testing.T) {
				got, err := ParseLevel(in)
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	})
	t.Run("Validate", func(t *testing.T) {
		cases := map[string]struct{}{
			"info":  {},
			"InFo":  {},
			"INFO":  {},
			"warn":  {},
			"error": {},
			"debug": {},
			"":      {},
		}

		for in := range cases {
			t.Run(in, func(t *testing.T) {
				err := Level(in).Validate()
				require.NoError(t, err)
			})
		}
	})

	t.Run("toZapCoreLevel", func(t *testing.T) {
		cases := map[string]zapcore.Level{
			"info":  zapcore.InfoLevel,
			"InFo":  zapcore.InfoLevel,
			"INFO":  zapcore.InfoLevel,
			"warn":  zapcore.WarnLevel,
			"error": zapcore.ErrorLevel,
			"debug": zapcore.DebugLevel,
			"":      zapcore.InfoLevel,
		}

		for in, want := range cases {
			t.Run(in, func(t *testing.T) {
				got, err := Level(in).toZapCoreLevel()
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		}
	})

	t.Run("Config.toZapCoreLevel", func(t *testing.T) {
		t.Run("debug=true overrides any level value", func(t *testing.T) {
			cases := map[string]struct{}{
				"info":  {},
				"InFo":  {},
				"INFO":  {},
				"warn":  {},
				"error": {},
				"debug": {},
				"":      {},
			}

			for in := range cases {
				t.Run(in, func(t *testing.T) {
					c := &Config{
						Debug: true,
						Level: Level(in),
					}

					got, err := c.toZapCoreLevel()
					require.NoError(t, err)
					require.Equal(t, zapcore.DebugLevel, got)
				})
			}
		})
		t.Run("debug=false doesn't override any level value", func(t *testing.T) {
			cases := map[string]zapcore.Level{
				"info":  zapcore.InfoLevel,
				"InFo":  zapcore.InfoLevel,
				"INFO":  zapcore.InfoLevel,
				"warn":  zapcore.WarnLevel,
				"error": zapcore.ErrorLevel,
				"debug": zapcore.DebugLevel,
				"":      zapcore.InfoLevel,
			}

			for in, want := range cases {
				t.Run(in, func(t *testing.T) {
					c := &Config{
						Level: Level(in),
					}

					got, err := c.toZapCoreLevel()
					require.NoError(t, err)
					require.Equal(t, want, got)
				})
			}
		})
	})
}
