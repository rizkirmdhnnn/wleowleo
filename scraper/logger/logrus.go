package logger

import (
	"wleowleo-scraper/config"

	"github.com/sirupsen/logrus"
)

// NewLogger creates a new logger
func NewLogger(cfg *config.Config) *logrus.Logger {
	log := logrus.New()

	// log.SetLevel(logrus.Level()
	log.SetFormatter(&logrus.TextFormatter{
		// DisableColors: true,
		FullTimestamp: true,
	})

	return log
}
