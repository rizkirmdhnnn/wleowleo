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
	"github.com/rizkirmdhn/wleowleo/internal/scraper/service"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Get the Scraper configuration
	scraperCfg := cfg.GetScraperConfig()
	rabbitCfg := cfg.GetRabbitMQConfig()

	// Initialize logger
	log := logger.New(cfg)

	// Print the Scraper configuration
	log.WithFields(logrus.Fields{
		"component": "scraper_main",
		"config":    fmt.Sprintf("%+v", scraperCfg),
	}).Debug("Scraper configuration loaded")

	log.WithFields(logrus.Fields{
		"component": "scraper_main",
		"config":    fmt.Sprintf("%+v", cfg.RabbitMq),
	}).Debug("RabbitMQ configuration loaded")

	// Create a new RabbitMQ client
	messagingClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": "scraper_main",
			"error":     err,
		}).Fatal("Failed to create RabbitMQ client")
	}
	defer messagingClient.Close()

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new Scraper service
	scraperService := service.NewScraperService(scraperCfg, rabbitCfg, log, messagingClient)

	// Start the service
	if err := scraperService.Start(); err != nil {
		log.WithFields(logrus.Fields{
			"component": "scraper_main",
			"error":     err,
		}).Fatal("Failed to start scraper service")
	}

	log.WithField("component", "scraper_main").Info("Scraper service started successfully")

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a termination signal
	sig := <-sigCh
	log.WithFields(logrus.Fields{
		"component": "scraper_main",
		"signal":    sig,
	}).Info("Received signal, shutting down")

	// Trigger graceful shutdown
	cancel()

	// Call Stop() if your service has such method
	scraperService.Stop()

	log.WithField("component", "scraper_main").Info("Scraper service stopped")
}
