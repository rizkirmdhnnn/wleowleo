package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

const (
	// Nama Exchange
	ExchangeName = "task_exchange"

	// Routing Keys
	RoutingCommandScraper    = "command.scraper"
	RoutingCommandDownloader = "command.downloader"
	RoutingTaskDownload      = "task.download"
	RoutingLogScraper        = "log.scraper"
	RoutingLogDownloader     = "log.downloader"

	// Queue Names
	// QueueScraper      = "scraper_queue"
	// QueueDownloader   = "downloader_queue"
	// QueueDownloadTask = "download_task_queue"
	// QueueLog          = "log_queue"

	// Exchange Type
	ExchangeTypeTopic = "topic"
)

// Config is the struct that holds the configuration of the application
type Config struct {
	App        AppConfig        `json:"app"`
	RabbitMq   RabbitMQConfig   `json:"rabbitmq"`
	Scraper    ScraperConfig    `json:"scraper"`
	Downloader DownloaderConfig `json:"downloader"`
	WebPanel   WebPanelConfig   `json:"webpanel"`
}

type AppConfig struct {
	Name     string `json:"name"`
	LogLevel int    `json:"logLevel"`
	Env      string `json:"env"`
}

type RabbitMQConfig struct {
	URL              string     `json:"url"`
	Exchange         string     `json:"exchange"`
	Queue            QueueNames `json:"queue"`
	ReconnectRetries int        `json:"reconnectRetries"`
	ReconnectTimeout int        `json:"reconnectTimeout"`
}

type ScraperConfig struct {
	Host      string `json:"host"`
	UserAgent string `json:"userAgent"`
}

type DownloaderConfig struct {
	DefaultWorker int    `json:"defaultWorker"`
	TempDir       string `json:"tempDir"`
	DownloadDir   string `json:"downloadDir"`
}

type WebPanelConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type QueueNames struct {
	Scraper      string `json:"scraper"`
	Downloader   string `json:"downloader"`
	DownloadTask string `json:"downloadTask"`
	Log          string `json:"log"`
}

// Load config from config.json
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config") // File name without extension
	v.SetConfigType("json")   // Set to JSON format
	v.AddConfigPath(".")      // Look for config file in current directory
	v.AutomaticEnv()

	// Try to read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal JSON to Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Override from environment variables if available
	if envURL := os.Getenv("RABBITMQ_URL"); envURL != "" {
		config.RabbitMq.URL = envURL
	}

	return &config, nil
}

// Get config for app
func (c *Config) GetAppConfig() *AppConfig {
	return &c.App
}

// Get config for scraping
func (c *Config) GetScraperConfig() *ScraperConfig {
	return &c.Scraper
}

// Get config for downloader
func (c *Config) GetDownloaderConfig() *DownloaderConfig {
	return &c.Downloader
}

// Get config for web panel
func (c *Config) GetWebPanelConfig() *WebPanelConfig {
	return &c.WebPanel
}

// Get config for RabbitMQ
func (c *Config) GetRabbitMQConfig() *RabbitMQConfig {
	return &c.RabbitMq
}
