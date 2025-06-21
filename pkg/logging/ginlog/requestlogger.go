package ginlog

import (
	"fmt"
	"regexp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/sgl-project/ome/pkg/logging"
)

const (
	RequestIDKey     = logging.RequestIDKey
	RequestIDHeader  = logging.RequestIDHeader
	RequestLoggerKey = logging.RequestLoggerKey
)

// RequestLoggerConfig is a config for RequestLogger.
type RequestLoggerConfig struct {
	// ExcludeQueryParameters controls whether query params are stripped off when logging.
	ExcludeQueryParameters bool `mapstructure:"exclude_query_parameters"`

	// LevelByPath sets a custom logging level by path.
	// e.g. "/api/v1/user/keys.pub" -> "debug".
	// Any other paths will be using default Info logging level
	LevelByPath map[string]string `mapstructure:"level_by_path"`

	// LevelByRegexPath sets a custom logging level by regex path.
	// e.g. "\\/api\\/v1\\/user/\\[a-zA-Z0-9_-]+\\/info" -> "debug".
	// Any other paths will be using default Info logging level
	LevelByRegexPath map[string]string `mapstructure:"level_by_regex_path"`
}

// Validate RequestLoggerConfig.
func (rec RequestLoggerConfig) Validate() error {
	// if LevelByRegexPath is not empty we must verify that they are able to compile.
	if len(rec.LevelByRegexPath) != 0 {
		for _, pattern := range rec.LevelByRegexPath {
			_, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("error compiling pattern %q: %w", pattern, err)
			}
		}
	}
	return nil
}

// Opts returns a set of opts to apply to RequestLogger constructor using this config.
func (rec RequestLoggerConfig) Opts() []RequestLoggerOption {
	result := make([]RequestLoggerOption, 0, 3)
	if len(rec.LevelByPath) != 0 {
		result = append(result, WithRequestLoggerLevelByPath(parseLevelByPath(rec)))
	}

	if len(rec.LevelByRegexPath) != 0 {
		result = append(result, WithRequestLoggerLevelByRegexPath(parseLevelByRegexPath(rec)))
	}

	result = append(result, WithRequestLoggerExcludeQueryParameters(rec.ExcludeQueryParameters))
	return result
}

// parseLevelByPath parses config LevelByRegexPath and returns a map of compiled string path -> zapcore.Level.
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

// parseLevelByRegexPath parses config LevelByRegexPath and returns a map of compiled regexp.Regexp path -> zapcore.Level.
func parseLevelByRegexPath(rec RequestLoggerConfig) map[*regexp.Regexp]zapcore.Level {
	levelByPath := make(map[*regexp.Regexp]zapcore.Level, len(rec.LevelByRegexPath))
	for path, lvlString := range rec.LevelByRegexPath {
		var lvl = zapcore.InfoLevel
		if err := lvl.UnmarshalText([]byte(lvlString)); err != nil {
			// we can't log anything here, nor we should panic/return error
			// so just drop back to default logging level silently
			lvl = zapcore.InfoLevel
		}

		// none of the patterns should fail to compile as Validate will fail when there is an invalid pattern.
		re := regexp.MustCompile(path)

		levelByPath[re] = lvl
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
	logger                 *zap.Logger
	levelByPath            map[string]zapcore.Level
	levelByRegexPath       map[*regexp.Regexp]zapcore.Level
	excludeQueryParameters bool
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

	lvl := rl.getLogLevel(ctx, path)

	// this is doing exactly what requestLogger.(Info/Debug/Fatal/...) methods
	// would do, but the logging level could be changed.
	if ce := requestLogger.Check(lvl, path); ce != nil {
		fields := []zap.Field{
			zap.String("method", ctx.Request.Method),
			zap.String("path", path),
			zap.String("ip", ctx.ClientIP()),
			zap.String("user-agent", ctx.Request.UserAgent()),
			zap.Int("status", ctx.Writer.Status()),
			zap.String("time", end.Format(logging.TimeFormat)),
			zap.Duration("latency", latency),
		}

		if !rl.excludeQueryParameters {
			fields = append(fields, zap.String("query", query))
		}

		ce.Write(fields...)
	}
}

// getLogLevel returns zapcore.Level based on how input path matched defined requestLogger paths.
func (rl *requestLogger) getLogLevel(_ *gin.Context, path string) zapcore.Level {
	// try to match with non-regex paths first.
	if lvl, ok := rl.levelByPath[path]; ok {
		return lvl
	}

	// match with regex paths if not matched it defaults to zapcore.InfoLevel.
	lvl := matchPathWithRegex(rl.levelByRegexPath, path)

	return lvl
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

// WithRequestLoggerLevelByRegexPath sets a custom logging level depending on regex path.
func WithRequestLoggerLevelByRegexPath(levelByRegexPath map[*regexp.Regexp]zapcore.Level) RequestLoggerOption {
	return func(rl *requestLogger) {
		rl.levelByRegexPath = levelByRegexPath
	}
}

// WithRequestLoggerExcludeQueryParameters controls whether to exclude
// query parameters from logging.
//
// By default, the query parameters are always logged.
func WithRequestLoggerExcludeQueryParameters(value bool) RequestLoggerOption {
	return func(rl *requestLogger) {
		rl.excludeQueryParameters = value
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

// matchPathWithRegex searches through compiled regexPaths and checks if path matches any of them.
func matchPathWithRegex(regexPaths map[*regexp.Regexp]zapcore.Level, path string) zapcore.Level {
	for re, lvl := range regexPaths {
		if re.MatchString(path) {
			return lvl
		}
	}

	return zapcore.InfoLevel
}
