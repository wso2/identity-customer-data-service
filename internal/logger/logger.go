package logger

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"log"
	"os"
)

var (
	Log            logr.Logger
	isDebugEnabled bool
)

// Init initializes the logger with configured log level
func Init(debugEnabled bool) {
	stdrLogger := stdr.New(log.New(os.Stderr, "", log.LstdFlags)).WithName("customer-data-service")
	Log = stdrLogger

	if debugEnabled {
		isDebugEnabled = true
		Log = Log.V(1)
	} else {
		isDebugEnabled = false
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	Log.Info(msg, keysAndValues...)
}

func Error(err error, msg string, keysAndValues ...interface{}) {
	Log.Error(err, msg, keysAndValues...)
}

// Debug only logs if isDebugEnabled
func Debug(msg string, keysAndValues ...interface{}) {
	if isDebugEnabled {
		Log.V(1).Info(msg, keysAndValues...)
	}
}
