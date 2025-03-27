package logger

import (
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/sirupsen/logrus"
)

func New(cfg *config.Config) *logrus.Logger {
	log := logrus.New()

	log.SetLevel(logrus.Level(cfg.App.LogLevel))
	log.SetFormatter(&logrus.TextFormatter{
		// DisableColors: true,
		FullTimestamp: true,
	})

	return log
}
