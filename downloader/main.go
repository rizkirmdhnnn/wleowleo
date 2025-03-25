package main

import (
	"wleowleo-downloader/config"
	"wleowleo-downloader/logger"
	"wleowleo-downloader/message"
	"wleowleo-downloader/scraper"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize logrus
	log := logger.NewLogger(cfg)

	// Initialize scraper
	downloader := scraper.New(cfg, log)

	// Initialize rabbitmq consumer
	consumer := message.NewConsumer(cfg, downloader, log)
	if err := consumer.Initialize(); err != nil {
		log.Error("Error initializing consumer:", err)
	}

	// Listen for messages
	log.Info("Listening for messages...")
	err := consumer.Listen()
	if err != nil {
		log.Error("Error listening for messages:", err)
	}
}
