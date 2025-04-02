package logger

import (
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/sirupsen/logrus"
)

// ComponentLogger wraps logrus.Logger to provide consistent component logging
type ComponentLogger struct {
	*logrus.Logger
	component string
}

// New creates a new logrus logger with standard configuration
func New(cfg *config.Config) *logrus.Logger {
	log := logrus.New()

	log.SetLevel(logrus.Level(cfg.App.LogLevel))
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors: true,
		// DisableColors: true,
		FullTimestamp: true,
	})

	return log
}

// NewComponentLogger creates a logger with a component field
func NewComponentLogger(log *logrus.Logger, component string) *ComponentLogger {
	return &ComponentLogger{
		Logger:    log,
		component: component,
	}
}

// WithField adds a field to the log entry
func (c *ComponentLogger) WithField(key string, value interface{}) *logrus.Entry {
	return c.Logger.WithFields(logrus.Fields{
		"component": c.component,
		key:         value,
	})
}

// WithFields adds multiple fields to the log entry, always including component
func (c *ComponentLogger) WithFields(fields logrus.Fields) *logrus.Entry {
	// Add component field if not already present
	if _, exists := fields["component"]; !exists {
		fields["component"] = c.component
	}
	return c.Logger.WithFields(fields)
}

// WithError adds an error field to the log entry
func (c *ComponentLogger) WithError(err error) *logrus.Entry {
	return c.Logger.WithFields(logrus.Fields{
		"component": c.component,
		"error":     err,
	})
}
