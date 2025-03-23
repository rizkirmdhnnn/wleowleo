package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	LimitConcurrentDownload int

	// RabbitMQ
	RabbitMQHost     string
	RabbitMQPort     int
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQQueue    string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// Log error but continue - we might be using environment variables directly
		log.Printf("Warning: Error loading .env file: %v\n", err)
	}

	limitConcurrentDownload, err := strconv.Atoi(os.Getenv("LIMIT_CONCURRENT_DOWNLOAD"))
	if err != nil {
		log.Panic("LIMIT_CONCURRENT_DOWNLOAD is required")
	}

	rabbitMQPort, err := strconv.Atoi(os.Getenv("RABBITMQ_PORT"))
	if err != nil {
		log.Panic("RABBITMQ_PORT is required")
	}

	cfg := &Config{
		LimitConcurrentDownload: limitConcurrentDownload,
		RabbitMQHost:            os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort:            rabbitMQPort,
		RabbitMQUser:            os.Getenv("RABBITMQ_USER"),
		RabbitMQPassword:        os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQQueue:           os.Getenv("RABBITMQ_QUEUE"),
	}
	return cfg
}
