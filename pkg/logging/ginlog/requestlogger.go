package ginlog

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
)

const (
	RequestIDKey     = logging.RequestIDKey
	RequestIDHeader  = logging.RequestIDHeader
	RequestLoggerKey = logging.RequestLoggerKey
)

// RequestLoggerConfig is a config for RequestLogger.
type RequestLoggerConfig struct {
	// LevelByPath sets a custom logging level by path.
	// e.g. "/api/v1/user/keys.pub" -> "debug".
	// Any other paths will be using default Info logging level
	LevelByPath map[string]string `mapstructure:"level_by_path"`
}

// Opts returns a set of opts to apply to RequestLogger constructor using this config.
func (rec RequestLoggerConfig) Opts() []RequestLoggerOption {
	result := make([]RequestLoggerOption, 0, 1)
	if len(rec.LevelByPath) != 0 {
		result = append(result, WithRequestLoggerLevelByPath(parseLevelByPath(rec)))
	}

	return result
}

func parseLevelByPath(rec RequestLoggerConfig) map[string]zapcore.Level {
	levelByPath := make(map[string]zapcore.Level, len(rec.LevelByPath))
	for path, lvlString := range rec.LevelByPath {
		var lvl = zapcore.InfoLevel

		if err := lvl.UnmarshalText([]byte(lvlString)); err != nil {
			// we can't log anything here, nor we should panic/return error
			// so just drop back to default logging level silently
			lvl = zapcore.InfoLevel
		}

		levelByPath[path] = lvl
	}

	return levelByPath
}

// GetRequestLogger returns a logger for the current request context. By
// default, this only includes the request ID.
func GetRequestLogger(ctx *gin.Context) *zap.Logger {
	return ctx.MustGet(RequestLoggerKey).(*zap.Logger)
}

// GetSugaredRequestLogger is a shortcut for `GetRequestLogger(c).Sugar()`.
func GetSugaredRequestLogger(ctx *gin.Context) *zap.SugaredLogger {
	return GetRequestLogger(ctx).Sugar()
}

type requestLogger struct {
	logger      *zap.Logger
	levelByPath map[string]zapcore.Level
}

func (rl *requestLogger) HandlerFunc(ctx *gin.Context) {
	start := logging.TimeNowFunc()

	// extract these in case other middleware modify them
	path := ctx.Request.URL.Path
	query := ctx.Request.URL.RawQuery

	// get a unique ID for this request
	requestID := GetOrCreateRequestID(ctx)

	// set up a context-specific logger
	requestLogger := rl.logger.With(zap.String(RequestIDKey, requestID))
	ctx.Set(RequestLoggerKey, requestLogger)

	// process the request
	ctx.Next()

	// calculate request duration
	end := logging.TimeNowFunc()
	latency := end.Sub(start)

	// write logs
	if len(ctx.Errors) > 0 {
		for _, err := range ctx.Errors.Errors() {
			requestLogger.Error(err)
		}
		return
	}

	lvl, ok := rl.levelByPath[path]
	if !ok {
		// default to Info level if path wasn't found
		lvl = zapcore.InfoLevel
	}

	// this is doing exactly what requestLogger.(Info/Debug/Fatal/...) methods
	// would do, but the logging level could be changed.
	if ce := requestLogger.Check(lvl, path); ce != nil {
		ce.Write(
			zap.String("method", ctx.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", ctx.ClientIP()),
			zap.String("user-agent", ctx.Request.UserAgent()),
			zap.Int("status", ctx.Writer.Status()),
			zap.String("time", end.Format(logging.TimeFormat)),
			zap.Duration("latency", latency),
		)
	}
}

// RequestLoggerOption is a functional option pattern
// applied to configuring RequestLogger.
type RequestLoggerOption func(*requestLogger)

// WithRequestLoggerLevelByPath sets a custom logging level depending on path.
func WithRequestLoggerLevelByPath(levelByPath map[string]zapcore.Level) RequestLoggerOption {
	return func(rl *requestLogger) {
		rl.levelByPath = levelByPath
	}
}

// RequestLogger returns a Gin middleware for logging using Zap.
func RequestLogger(logger *zap.Logger, opts ...RequestLoggerOption) gin.HandlerFunc {
	rl := &requestLogger{logger: logger}
	for _, opt := range opts {
		opt(rl)
	}

	return rl.HandlerFunc
}
