package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BaseURL      string
	UserAgent    string
	Pages        int
	AutoDownload bool

	// RabbitMQ
	RabbitMQHost     string
	RabbitMQPort     int
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQQueue    string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// Log error but continue - we might be using environment variables directly
		log.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// Get environment variables
	baseURL := os.Getenv("BASE_URL")
	userAgent := os.Getenv("USERAGENT")
	pages := os.Getenv("PAGES")

	// Check if required environment variables are set
	if baseURL == "" {
		log.Panic("BASE_URL is required")
	}
	if userAgent == "" {
		log.Panic("USER_AGENT is required")
	}
	if pages == "" {
		log.Panic("PAGES is required")
	}

	// Convert PAGES to integer
	pageInt, err := strconv.Atoi(pages)
	if err != nil {
		log.Panic("Error converting PAGES to integer:", err)
	}

	rabbitMQPort, err := strconv.Atoi(os.Getenv("RABBITMQ_PORT"))
	if err != nil {
		log.Panic("RABBITMQ_PORT is required")
	}

	return &Config{
		BaseURL:   baseURL,
		UserAgent: userAgent,
		Pages:     pageInt,

		RabbitMQHost:     os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort:     rabbitMQPort,
		RabbitMQUser:     os.Getenv("RABBITMQ_USER"),
		RabbitMQPassword: os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQQueue:    os.Getenv("RABBITMQ_QUEUE"),
	}
}
