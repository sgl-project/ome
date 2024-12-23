package ginlog

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestGetOrCreateRequestID(t *testing.T) {
	t.Run("if no request ID is present, then one should be created", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err, "should not error creating request")

		c := &gin.Context{Request: r}

		_, ok := c.Get(RequestIDKey)
		assert.False(t, ok, "should not have request ID")

		id := GetOrCreateRequestID(c)
		assert.NotEmpty(t, id, "request ID should not be empty")
	})

	t.Run("if request ID is present in header, then it should be used", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err, "should not error creating request")

		r.Header.Add(RequestIDHeader, "test")
		c := &gin.Context{Request: r}

		id := GetOrCreateRequestID(c)
		assert.Equal(t, "test", id)
	})

	t.Run("if request ID is on context, then it should be used", func(t *testing.T) {
		c := &gin.Context{}
		c.Set(RequestIDKey, "test")

		id := GetOrCreateRequestID(c)
		assert.Equal(t, "test", id)
	})
}

func TestRequestLogger_getLogLevel(t *testing.T) {
	zapDev, err := zap.NewDevelopment()
	require.NoError(t, err)

	t.Run("when no paths are defined", func(t *testing.T) {
		rl := requestLogger{
			logger: zapDev,
		}

		responseRecorder := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(responseRecorder)
		lvl := rl.getLogLevel(requestContext, "/")

		require.Equal(t, zapcore.InfoLevel, lvl)
	})

	t.Run("when levelByPath is defined", func(t *testing.T) {
		rl := requestLogger{
			logger: zapDev,
			levelByPath: map[string]zapcore.Level{
				"/": zapcore.DebugLevel,
			},
		}

		responseRecorder := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(responseRecorder)
		lvl := rl.getLogLevel(requestContext, "/")

		require.Equal(t, zapcore.DebugLevel, lvl)
	})

	t.Run("when levelByPath and  is defined", func(t *testing.T) {
		exp, err := regexp.Compile("^/hostclass/[a-zA-Z0-9_-]+/regexptest")
		require.NoError(t, err)

		rl := requestLogger{
			logger: zapDev,
			levelByPath: map[string]zapcore.Level{
				"/": zapcore.DebugLevel,
			},
			levelByRegexPath: map[*regexp.Regexp]zapcore.Level{
				exp: zapcore.FatalLevel,
			},
		}

		responseRecorder := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(responseRecorder)

		lvl := rl.getLogLevel(requestContext, "/")
		require.Equal(t, zapcore.DebugLevel, lvl)

		lvl = rl.getLogLevel(requestContext, "/hostclass/hc_foo/regexptest")
		require.Equal(t, zapcore.FatalLevel, lvl)
	})
}

func TestParseLevelByRegexPath(t *testing.T) {
	config := RequestLoggerConfig{
		LevelByPath: map[string]string{},
		LevelByRegexPath: map[string]string{
			"^/hostclass/[a-zA-Z0-9_-]+/regexptest": "debug",
		},
	}

	t.Run("test parse level by tegex path", func(t *testing.T) {
		lvlPath := parseLevelByRegexPath(config)
		require.Equal(t, 1, len(lvlPath))

		for re, lvl := range lvlPath {
			assert.True(t, re.MatchString("/hostclass/hc_foo/regexptest"))
			assert.Equal(t, zapcore.DebugLevel, lvl)
		}
	})

	t.Run("test parse level by tegex path", func(t *testing.T) {

		config.LevelByRegexPath = map[string]string{
			"^/hostclass/[a-zA-Z0-9_-]+/regexptest": "lala",
		}
		lvlPath := parseLevelByRegexPath(config)
		require.Equal(t, 1, len(lvlPath))

		for re, lvl := range lvlPath {
			assert.True(t, re.MatchString("/hostclass/hc_foo/regexptest"))
			assert.Equal(t, zapcore.InfoLevel, lvl)
		}
	})
}
