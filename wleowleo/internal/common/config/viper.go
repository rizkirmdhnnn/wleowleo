package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the struct that holds the configuration of the application
type Config struct {
	App        AppConfig        `yaml:"app"`
	RabbitMq   RabbitMQConfig   `yaml:"rabbitmq"`
	Scraper    ScraperConfig    `yaml:"scraper"`
	Downloader DownloaderConfig `yaml:"downloader"`
	WebPanel   WebPanelConfig   `yaml:"webpanel"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	LogLevel int    `yaml:"logLevel"`
	Env      string `yaml:"env"`
}

type QueueNames struct {
	ScraperCommandQueue string `yaml:"scraperCommandQueue"`
	VideoLinksQueue     string `yaml:"videoLinksQueue"`
	ScraperLogQueue     string `yaml:"scraperLogQueue"`
}

type RabbitMQConfig struct {
	URL              string     `yaml:"url"`
	Exchange         string     `yaml:"exchange"`
	Queue            QueueNames `yaml:"queue"`
	ReconnectRetries int        `yaml:"reconnectRetries"`
	ReconnectTimeout int        `yaml:"reconnectTimeout"`
}

type ScraperConfig struct {
	Host      string `yaml:"host"`
	StartPage int    `yaml:"startPage"`
	EndPage   int    `yaml:"endPage"`
	UserAgent string `yaml:"userAgent"`
}

type DownloaderConfig struct {
	Concurrency int    `yaml:"concurrency"`
	TempDir     string `yaml:"tempDir"`
	DownloadDir string `yaml:"downloadDir"`
}

type WebPanelConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// NewConfig creates a new Config struct
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	// Add environment variable support
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Try to read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal directly into config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
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
