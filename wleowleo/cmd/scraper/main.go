package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/logger"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/scraper/service"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Get the Scraper configuration
	scraperCfg := cfg.GetAppConfig()

	// Initialize logger
	log := logger.New(cfg)

	// Print the Scraper configuration
	log.Infof("Scraper configuration: %+v", scraperCfg)

	// Create a new RabbitMQ client
	messagingClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ client: %v", err)
	}
	defer messagingClient.Close()

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new Scraper service
	scraperService := service.NewScraperService(&cfg.Scraper, log, messagingClient)

	// Start the service
	if err := scraperService.Start(); err != nil {
		log.Fatalf("Failed to start scraper service: %v", err)
	}

	log.Info("Scraper service started successfully")

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a termination signal
	sig := <-sigCh
	log.Infof("Received signal %s, shutting down...", sig)

	// Trigger graceful shutdown
	cancel()

	// // Call Stop() if your service has such method
	// if err := scraperService.Stop(); err != nil {
	// 	log.Errorf("Error stopping scraper service: %v", err)
	// }

	log.Info("Scraper service stopped")
}
