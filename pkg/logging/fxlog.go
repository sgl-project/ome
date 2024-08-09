package logging

import (
	"strings"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// UseLoggingInterface makes fx itself log its events to
// the instance of logging.Interface inside the container
// being built.
// Requires logging.Interface to be provided within the resulting fx.App.
var UseLoggingInterface fx.Option = fx.WithLogger(
	func(logger Interface) fxevent.Logger {
		return &fxLoggerAdapter{Interface: logger}
	},
)

type fxLoggerAdapter struct{ Interface }

// LogEvent logs an FX app event to the underlying logging.Interface.
func (f fxLoggerAdapter) LogEvent(event fxevent.Event) {
	log := f.Interface.WithField("fx", "event")

	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		log.WithField("callee", e.FunctionName).
			WithField("caller", e.CallerName).
			Info("OnStart hook executing")
	case *fxevent.OnStartExecuted:
		infoOrErr("OnStart hook", e.Err,
			log.WithField("callee", e.FunctionName).
				WithField("caller", e.CallerName).
				WithField("method", e.Method).
				WithField("runtime", e.Runtime.String()))
	case *fxevent.OnStopExecuting:
		log.WithField("callee", e.FunctionName).
			WithField("caller", e.CallerName).
			Info("OnStop hook executing")
	case *fxevent.OnStopExecuted:
		infoOrErr("OnStop hook", e.Err,
			log.WithField("callee", e.FunctionName).
				WithField("caller", e.CallerName).
				WithField("runtime", e.Runtime.String()))
	case *fxevent.Supplied:
		log.WithField("type", e.TypeName).
			WithError(e.Err).
			Info("Supplied")
	case *fxevent.Provided:
		for _, rtype := range e.OutputTypeNames {
			log.WithField("constructor", e.ConstructorName).
				WithField("type", rtype).
				Info("Provided")
		}
		if e.Err != nil {
			log.WithError(e.Err).Error("error encountered while applying options")
		}
	case *fxevent.Invoking:
		log.WithField("function", e.FunctionName).
			Info("Invoking")
	case *fxevent.Invoked:
		infoOrErr("Invoke", e.Err,
			log.WithField("stack", e.Trace).
				WithField("function", e.FunctionName))
	case *fxevent.Stopping:
		log.WithField("signal", strings.ToUpper(e.Signal.String())).
			Info("Stopping: received signal")
	case *fxevent.Stopped:
		infoOrErr("App stop", e.Err, log)
	case *fxevent.RollingBack:
		infoOrErr("Start failed, trying to rolling back...", e.StartErr, log)
	case *fxevent.RolledBack:
		infoOrErr("Rolling back", e.Err, log)
	case *fxevent.Started:
		infoOrErr("App start", e.Err, log)
	case *fxevent.LoggerInitialized:
		infoOrErr("Custom logger initialization", e.Err,
			log.WithField("function", e.ConstructorName))
	default:
		log.WithField("event", event).Warn("Unknown fx event")
	}
}

func infoOrErr(msg string, err error, log Interface) {
	if err == nil {
		log.Info(msg + " succeeded")
		return
	}

	log.WithError(err).
		Error(msg + " failed")
}
