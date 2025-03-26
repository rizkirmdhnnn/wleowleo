package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Web server configuration
	WebPort string

	// RabbitMQ configuration
	RabbitMQHost     string
	RabbitMQPort     int
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQQueue    string

	// Scraper configuration
	BaseURL   string
	UserAgent string
	FromPages int
	ToPages   int

	// Downloader configuration
	LimitConcurrentDownload int

	// Log level
	LogLevel int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// Get web server configuration
	webPort := os.Getenv("WEB_PORT")
	if webPort == "" {
		webPort = "8080"
	}

	// Get RabbitMQ configuration
	rabbitMQPort, err := strconv.Atoi(os.Getenv("RABBITMQ_PORT"))
	if err != nil {
		rabbitMQPort = 5672 // Default RabbitMQ port
	}

	// Get scraper configuration
	baseURL := os.Getenv("BASE_URL")
	userAgent := os.Getenv("USERAGENT")

	fromPages, err := strconv.Atoi(os.Getenv("FROM_PAGES"))
	if err != nil {
		fromPages = 1 // Default from page
	}

	toPages, err := strconv.Atoi(os.Getenv("TO_PAGES"))
	if err != nil {
		toPages = 5 // Default to page
	}

	// Get downloader configuration
	limitConcurrentDownload, err := strconv.Atoi(os.Getenv("LIMIT_CONCURRENT_DOWNLOAD"))
	if err != nil {
		limitConcurrentDownload = 5 // Default concurrent download limit
	}

	// Get log level
	logLevel, err := strconv.Atoi(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = 4 // Default to Info level
	}

	return &Config{
		WebPort:                 webPort,
		RabbitMQHost:            os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort:            rabbitMQPort,
		RabbitMQUser:            os.Getenv("RABBITMQ_USER"),
		RabbitMQPassword:        os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQQueue:           os.Getenv("RABBITMQ_QUEUE"),
		BaseURL:                 baseURL,
		UserAgent:               userAgent,
		FromPages:               fromPages,
		ToPages:                 toPages,
		LimitConcurrentDownload: limitConcurrentDownload,
		LogLevel:                logLevel,
	}
}
