package newrelic

import (
	"shipper-deployment/internal/config"
	"shipper-deployment/internal/logger"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

var (
	// Global New Relic application instance
	App *newrelic.Application
)

// Initialize sets up New Relic monitoring
func Initialize(cfg *config.Config) (*newrelic.Application, error) {
	nrLogger := logger.WithModule("newrelic")

	if !cfg.NewRelicEnabled {
		nrLogger.Info("New Relic monitoring is disabled")
		return newrelic.NewApplication(newrelic.ConfigEnabled(false))
	}

	if cfg.NewRelicLicense == "" {
		nrLogger.Warn("New Relic license key is not provided, monitoring will be disabled")
		return newrelic.NewApplication(newrelic.ConfigEnabled(false))
	}

	nrLogger.Info("Initializing New Relic monitoring")

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.NewRelicAppName),
		newrelic.ConfigLicense(cfg.NewRelicLicense),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigLogger(newRelicLogger{logger: nrLogger}),
	)

	if err != nil {
		nrLogger.WithError(err).Error("Failed to initialize New Relic")
		return nil, err
	}

	// Set the global App instance
	App = app

	nrLogger.WithFields(logrus.Fields{
		"app_name": cfg.NewRelicAppName,
		"enabled":  cfg.NewRelicEnabled,
	}).Info("New Relic initialized successfully")

	return app, nil
}

// newRelicLogger implements the newrelic.Logger interface using logrus
type newRelicLogger struct {
	logger *logrus.Entry
}

func (l newRelicLogger) Error(msg string, context map[string]interface{}) {
	l.logger.WithFields(logrus.Fields(context)).Error(msg)
}

func (l newRelicLogger) Warn(msg string, context map[string]interface{}) {
	l.logger.WithFields(logrus.Fields(context)).Warn(msg)
}

func (l newRelicLogger) Info(msg string, context map[string]interface{}) {
	l.logger.WithFields(logrus.Fields(context)).Info(msg)
}

func (l newRelicLogger) Debug(msg string, context map[string]interface{}) {
	l.logger.WithFields(logrus.Fields(context)).Debug(msg)
}

func (l newRelicLogger) DebugEnabled() bool {
	return l.logger.Logger.IsLevelEnabled(logrus.DebugLevel)
}

// GetApp returns the global New Relic application instance
func GetApp() *newrelic.Application {
	return App
}

// IsEnabled returns true if New Relic is enabled and initialized
func IsEnabled() bool {
	return App != nil && App.WaitForConnection(0) == nil
}
