package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/logger"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/downloader/service"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Initialize logger
	log := logger.New(cfg)

	// Print the Downloader configuration
	log.WithFields(logrus.Fields{
		"component": "downloader_main",
		"config":    fmt.Sprintf("%+v", cfg.Downloader),
	}).Debug("Downloader configuration loaded")

	log.WithFields(logrus.Fields{
		"component": "downloader_main",
		"config":    fmt.Sprintf("%+v", cfg.RabbitMq),
	}).Debug("RabbitMQ configuration loaded")

	// Initialize RabbitMQ connection
	messageClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": "downloader_main",
			"error":     err,
		}).Fatal("Failed to initialize RabbitMQ")
	}
	defer messageClient.Close()

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the Downloader service
	downloaderService := service.NewDownloaderService(&cfg.Downloader, &cfg.RabbitMq, log, messageClient)

	// Start the service
	if err := downloaderService.Start(); err != nil {
		log.WithFields(logrus.Fields{
			"component": "downloader_main",
			"error":     err,
		}).Fatal("Failed to start Downloader service")
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a termination signal
	sig := <-sigCh
	log.WithFields(logrus.Fields{
		"component": "downloader_main",
		"signal":    sig,
	}).Info("Received signal, shutting down")

	// Stop the service
	// downloaderService.Stop()

	// Trigger graceful shutdown
	cancel()
}
