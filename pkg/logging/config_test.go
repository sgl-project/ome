package logging

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestNewConfig_Viper(t *testing.T) {
	v := viper.New()
	v.SetConfigType("YAML")
	require.NoError(t, v.ReadConfig(strings.NewReader(`---
logging:
  debug: true
  level: WARN
  maxage: 10
  maxsize: 42
  maxbackups: 100
  compress: true
  localtime: true
  encodetimeasrfc3339nano: true
  disableConsoleOutput: true
  filename: /var/log/example-application/application.log
`)))

	c, err := NewConfig(WithViper(v))
	require.NoError(t, err)

	d := cmp.Diff(c, &Config{
		Debug:                   true,
		Level:                   LevelWarn,
		EncodeTimeAsRFC3339Nano: true,
		DisableConsoleOutput:    true,
		Logger: lumberjack.Logger{
			Filename:   "/var/log/example-application/application.log",
			MaxSize:    42,
			MaxAge:     10,
			MaxBackups: 100,
			LocalTime:  true,
			Compress:   true,
		},
	}, cmpopts.IgnoreUnexported(lumberjack.Logger{}))
	require.Empty(t, d)
}
