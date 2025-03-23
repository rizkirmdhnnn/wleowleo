package config

import (
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BaseURL         string
	UserAgent       string
	Pages           int
	AutoDownload    bool
	LimitConcurrent int

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
	autoDownload := os.Getenv("AUTO_DOWNLOAD")
	limitConcurrent := os.Getenv("LIMIT_CONCURRENT")

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
	if autoDownload == "" {
		log.Panic("AUTO_DOWNLOAD is required")
	}
	if limitConcurrent == "" {
		log.Panic("LIMIT_CONCURRENT is required")
	}

	// Check if ffmpeg is installed
	if autoDownload == "true" {
		execFFmpeg := exec.Command("ffmpeg", "-version")
		err := execFFmpeg.Run()
		if err != nil {
			log.Panic("Auto download requires ffmpeg to be installed")
		}
	}

	// Convert PAGES to integer
	pageInt, err := strconv.Atoi(pages)
	if err != nil {
		log.Panic("Error converting PAGES to integer:", err)
	}

	// Convert LIMIT_CONCURRENT to integer
	limitConcurrentInt, err := strconv.Atoi(limitConcurrent)
	if err != nil {
		log.Panic("Error converting LIMIT_CONCURRENT to integer:", err)
	}

	rabbitMQPort, err := strconv.Atoi(os.Getenv("RABBITMQ_PORT"))
	if err != nil {
		log.Panic("RABBITMQ_PORT is required")
	}

	return &Config{
		BaseURL:         baseURL,
		UserAgent:       userAgent,
		Pages:           pageInt,
		AutoDownload:    autoDownload == "true",
		LimitConcurrent: limitConcurrentInt,

		RabbitMQHost:     os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort:     rabbitMQPort,
		RabbitMQUser:     os.Getenv("RABBITMQ_USER"),
		RabbitMQPassword: os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQQueue:    os.Getenv("RABBITMQ_QUEUE"),
	}
}
