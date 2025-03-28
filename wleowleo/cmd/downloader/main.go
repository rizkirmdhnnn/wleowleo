package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/logger"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/downloader/service"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Get the Scraper configuration
	scraperCfg := cfg.GetDownloaderConfig()
	rabbitCfg := cfg.GetRabbitMQConfig()

	// Initialize logger
	log := logger.New(cfg)

	// Print the Scraper configuration
	log.Infof("Downloader configuration: %+v", scraperCfg)
	log.Infof("RabbitMQ configuration: %+v", cfg.RabbitMq)

	// Initialize RabbitMQ connection
	messageClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ: %s", err)
	}
	defer messageClient.Close()

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the Downloader service
	downloaderService := service.NewDownloaderService(scraperCfg, rabbitCfg, log, messageClient)

	// Start the service
	if err := downloaderService.Start(); err != nil {
		log.Fatalf("Failed to start Downloader service: %s", err)
	}

	log.Info("Downloader service started successfully")

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a termination signal
	sig := <-sigCh
	log.Infof("Received signal %s, shutting down...", sig)

	// Trigger graceful shutdown
	cancel()
}
