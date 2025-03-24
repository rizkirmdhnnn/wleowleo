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
	FromPages    int
	ToPages      int
	AutoDownload bool
	LogLevel     int

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
	fromPages := os.Getenv("FROM_PAGES")
	toPages := os.Getenv("TO_PAGES")

	// Check if required environment variables are set
	if baseURL == "" {
		log.Panic("BASE_URL is required")
	}
	if userAgent == "" {
		log.Panic("USER_AGENT is required")
	}
	if fromPages == "" {
		log.Panic("FROM_PAGES is required")
	}
	if toPages == "" {
		log.Panic("TO_PAGES is required")
	}

	// Convert PAGES to integer
	fromPagesInt, err := strconv.Atoi(fromPages)
	if err != nil {
		log.Panic("Error converting PAGES to integer:", err)
	}
	toPagesInt, err := strconv.Atoi(toPages)
	if err != nil {
		log.Panic("Error converting PAGES to integer:", err)
	}

	rabbitMQPort, err := strconv.Atoi(os.Getenv("RABBITMQ_PORT"))
	if err != nil {
		log.Panic("RABBITMQ_PORT is required")
	}

	logLevel, err := strconv.Atoi(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = 4 // default to Info level
	}

	return &Config{
		BaseURL:   baseURL,
		UserAgent: userAgent,
		FromPages: fromPagesInt,
		ToPages:   toPagesInt,
		LogLevel:  logLevel,

		RabbitMQHost:     os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort:     rabbitMQPort,
		RabbitMQUser:     os.Getenv("RABBITMQ_USER"),
		RabbitMQPassword: os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQQueue:    os.Getenv("RABBITMQ_QUEUE"),
	}
}
