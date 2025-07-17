package logger

import (
	"crypto/rand"
	"encoding/hex"
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
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
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

// GenerateRequestID creates a unique ID for request tracking
func GenerateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "req-error-generating"
	}
	return "req-" + hex.EncodeToString(bytes)
}

// WithRequestID creates a new entry with a request ID
func WithRequestID() *logrus.Entry {
	return Get().WithField("request_id", GenerateRequestID())
}

// WithContext creates a new entry with context ID
func WithContext(contextID string) *logrus.Entry {
	return Get().WithField("context_id", contextID)
}

// WithModule creates a new entry with module name
func WithModule(moduleName string) *logrus.Entry {
	return Get().WithField("module", moduleName)
}

// WithRequestIDAndModule creates a new entry with both request ID and module name
func WithRequestIDAndModule(moduleName string) *logrus.Entry {
	return Get().WithFields(logrus.Fields{
		"request_id": GenerateRequestID(),
		"module":     moduleName,
	})
}

// WithContextAndModule creates a new entry with both context ID and module name
func WithContextAndModule(contextID, moduleName string) *logrus.Entry {
	return Get().WithFields(logrus.Fields{
		"context_id": contextID,
		"module":     moduleName,
	})
}

// Helper function for formatting caller information
func callerPrettyfier(f *runtime.Frame) (string, string) {
	filename := path.Base(f.File)
	return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
}
