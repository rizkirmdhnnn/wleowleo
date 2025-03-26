package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rizkirmdhn/wleowleo/web/websocket"
)

// Stats represents the statistics of the scraping and downloading process
type Stats struct {
	TotalPageScraped int `json:"total_page_scraped"`
	TotalScrapedLink int `json:"total_scraped_link"`
	VideoDownloaded  int `json:"video_downloaded"`
}

// ConfigRequest represents the configuration request from the client
type ConfigRequest struct {
	BaseURL                 string `json:"base_url"`
	FromPages               int    `json:"from_pages"`
	ToPages                 int    `json:"to_pages"`
	LimitConcurrentDownload int    `json:"limit_concurrent_download"`
}

// IndexHandler handles the index page request
func IndexHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "WleoWleo Dashboard",
			"config": gin.H{
				"base_url":                os.Getenv("BASE_URL"),
				"FromPages":               os.Getenv("FROM_PAGES"),
				"ToPages":                 os.Getenv("TO_PAGES"),
				"LimitConcurrentDownload": os.Getenv("LIMIT_CONCURRENT_DOWNLOAD"),
			},
		})
	}
}

// GetStatsHandler handles the stats request
func GetStatsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, you would get these values from the actual services
		// For now, we'll use dummy values
		stats := Stats{
			TotalPageScraped: 0,
			TotalScrapedLink: 0,
			VideoDownloaded:  0,
		}

		c.JSON(http.StatusOK, stats)
	}
}

// StartScrapingHandler handles the start scraping request
func StartScrapingHandler(hub *websocket.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, you would start the scraping process here
		// For now, we'll just send a message to all connected clients
		message := map[string]string{
			"type":    "status",
			"message": "Scraping started",
		}

		messageJSON, err := json.Marshal(message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal message"})
			return
		}

		hub.Broadcast(messageJSON)

		c.JSON(http.StatusOK, gin.H{"status": "Scraping started"})
	}
}

// UpdateConfigHandler handles the update configuration request
func UpdateConfigHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var configReq ConfigRequest
		if err := c.ShouldBindJSON(&configReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Update the .env file with the new configuration
		envFile := ".env"
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read .env file"})
			return
		}

		// Update the configuration in the .env file
		// This is a simple implementation, in a real application you would use a proper
		// library to handle .env files
		os.Setenv("BASE_URL", configReq.BaseURL)
		os.Setenv("FROM_PAGES", strconv.Itoa(configReq.FromPages))
		os.Setenv("TO_PAGES", strconv.Itoa(configReq.ToPages))
		os.Setenv("LIMIT_CONCURRENT_DOWNLOAD", strconv.Itoa(configReq.LimitConcurrentDownload))

		// Write the updated configuration to the .env file
		if err := os.WriteFile(envFile, envContent, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write .env file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "Configuration updated"})
	}
}
