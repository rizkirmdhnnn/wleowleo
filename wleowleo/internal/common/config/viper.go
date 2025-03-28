package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
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
	URL              string        `json:"url"`
	Exchange         ExchangeNames `json:"exchange"`
	Queue            QueueNames    `json:"queue"`
	ReconnectRetries int           `json:"reconnectRetries"`
	ReconnectTimeout int           `json:"reconnectTimeout"`
}

type ScraperConfig struct {
	Host      string `json:"host"`
	StartPage int    `json:"startPage"`
	EndPage   int    `json:"endPage"`
	UserAgent string `json:"userAgent"`
}

type DownloaderConfig struct {
	Concurrency int    `json:"concurrency"`
	TempDir     string `json:"tempDir"`
	DownloadDir string `json:"downloadDir"`
}

type WebPanelConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type ExchangeNames struct {
	Task string `json:"task"`
	Log  string `json:"log"`
}

type QueueNames struct {
	CommandQueue    string `json:"commandQueue"`
	DownloaderQueue string `json:"downloaderQueue"`
	LogQueue        string `json:"logQueue"`
}

// Load config from config.json
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config") // Nama file tanpa ekstensi
	v.SetConfigType("json")   // Ubah ke JSON
	v.AddConfigPath(".")      // Cari file di direktori saat ini
	v.AutomaticEnv()

	// Coba baca file konfigurasi
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	fmt.Printf("Viper All Settings: %+v\n", v.AllSettings())

	// Unmarshal JSON ke struct Config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	fmt.Printf("Config: %+v\n", config)

	// Override dari environment variable jika ada
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
