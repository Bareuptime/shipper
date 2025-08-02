package logger

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	// Global logger instance
	globalLogger *logrus.Logger
)

// Initialize sets up the global logger with the specified configuration
func Initialize() *logrus.Logger {
	if globalLogger != nil {
		return globalLogger
	}

	logger := logrus.New()

	// Configure logger based on environment
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Configure log format
	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	if logFormat == "text" {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    true,
			ForceColors:      true,
			CallerPrettyfier: callerPrettyfier,
		})
	} else {
		// Default to JSON format
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat:  "2006-01-02T15:04:05.000Z07:00",
			CallerPrettyfier: callerPrettyfier,
		})
	}

	// Enable reporting of the caller
	logger.SetReportCaller(true)

	// Ensure logs go to stdout
	logger.SetOutput(os.Stdout)

	globalLogger = logger
	return logger
}

// Get returns the global logger instance, initializing it if necessary
func Get() *logrus.Logger {
	if globalLogger == nil {
		return Initialize()
	}
	return globalLogger
}

// WithModule creates a new entry with module name
func WithModule(moduleName string) *logrus.Entry {
	return Get().WithField("module", moduleName)
}

// Helper function for formatting caller information
func callerPrettyfier(f *runtime.Frame) (string, string) {
	filename := path.Base(f.File)
	return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
}
