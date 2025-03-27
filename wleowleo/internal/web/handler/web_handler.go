package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/web/websocket"
	"github.com/rizkirmdhn/wleowleo/pkg/models"
	"github.com/sirupsen/logrus"
)

// Constants for the message routing
const (
	CommandsRoutingKey = "commands.scraper"
)

type Handler struct {
	webCfg      *config.WebPanelConfig
	scrapCfg    *config.ScraperConfig
	downloadCfg *config.DownloaderConfig
	log         *logrus.Logger
	msgClient   messaging.Client
	wsHub       *websocket.Hub
}

func NewHander(webCfg *config.WebPanelConfig, scrapCfg *config.ScraperConfig, dl *config.DownloaderConfig, log *logrus.Logger, msgClient messaging.Client) *Handler {
	// Create WebSocket hub
	wsHub := websocket.NewHub(log)
	go wsHub.Run()

	handler := &Handler{
		webCfg:      webCfg,
		scrapCfg:    scrapCfg,
		downloadCfg: dl,
		log:         log,
		msgClient:   msgClient,
		wsHub:       wsHub,
	}

	// Setup RabbitMQ consumer
	handler.setupRabbitMQConsumer()

	return handler
}

// IndexHandler handles the index page request
func (h *Handler) IndexHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "WleoWleo Dashboard",
			"config": gin.H{
				"base_url":                h.webCfg.Host,
				"FromPages":               h.scrapCfg.StartPage,
				"ToPages":                 h.scrapCfg.EndPage,
				"LimitConcurrentDownload": h.downloadCfg.Concurrency,
			},
		})
	}
}

// WebSocketHandler returns the WebSocket connection handler
func (h *Handler) WebSocketHandler() gin.HandlerFunc {
	return websocket.WebSocketHandler(h.wsHub, h.log)
}

// setupRabbitMQConsumer sets up RabbitMQ consumers for logs and other messages
func (h *Handler) setupRabbitMQConsumer() {
	// Get queue configuration
	queueName := h.msgClient.(*messaging.RabbitMQClient).GetConfig().Queue.ScraperLogQueue

	// Setup consumer for scraper logs
	err := h.msgClient.Consume(queueName, func(message []byte) error {
		// Parse message
		var scrapLog models.ScrapLog
		if err := json.Unmarshal(message, &scrapLog); err != nil {
			h.log.WithError(err).Error("Failed to unmarshal scraper log message")
			return err
		}

		// Format message for WebSocket clients
		wsMessage, err := json.Marshal(map[string]any{
			"type":   "scraper_log",
			"status": scrapLog.Status,
			"data":   scrapLog.Data,
			"error":  scrapLog.Error,
			"stats":  scrapLog.Stats,
		})

		if err != nil {
			h.log.WithError(err).Error("Failed to marshal WebSocket message")
			return err
		}

		// Broadcast message to all WebSocket clients
		h.wsHub.Broadcast(wsMessage)
		h.log.WithField("message", string(wsMessage)).Info("Broadcasting message to WebSocket clients")

		return nil
	})

	if err != nil {
		h.log.WithError(err).Error("Failed to setup RabbitMQ consumer")
	}
}

// RegisterRoutes registers all the routes for the web handler
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Web views
	r.GET("/", h.IndexHandler())
	r.GET("/ws", h.WebSocketHandler())

	// API endpoints
	api := r.Group("/api")
	{
		api.POST("/start", h.StartScraperHandler())
		api.POST("/stop", h.StopScraperHandler())
		api.GET("/stats", h.GetStatsHandler())
		// api.POST("/config", h.UpdateConfigHandler())
	}
}

// StartScraperHandler handles requests to start the scraper
func (h *Handler) StartScraperHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req struct {
			FromPages          int `json:"from_pages"`
			ToPages            int `json:"to_pages"`
			ConcurrentDownload int `json:"concurrent_download"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			return
		}

		// Validate input
		if req.FromPages <= 0 {
			req.FromPages = h.scrapCfg.StartPage
		}
		if req.ToPages < req.FromPages {
			req.ToPages = req.FromPages
		}
		if req.ConcurrentDownload <= 0 {
			req.ConcurrentDownload = h.downloadCfg.Concurrency
		}

		// Create command
		command := models.ScrapingCommand{
			Action: models.StartScrapingAction,
			Data: models.Data{
				StartPage: req.FromPages,
				EndPage:   req.ToPages,
			},
		}

		// Publish command to RabbitMQ
		if err := h.publishCommand(command); err != nil {
			h.log.WithError(err).Error("Failed to publish start command")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to start scraper",
			})
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"message": "Scraper started successfully",
			"config": gin.H{
				"from_pages":          req.FromPages,
				"to_pages":            req.ToPages,
				"concurrent_download": req.ConcurrentDownload,
			},
		})

		// Broadcast a message to all WebSocket clients
		h.broadcastStatus("Scraping process started", "info")
	}
}

// StopScraperHandler handles requests to stop the scraper
func (h *Handler) StopScraperHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create stop command
		command := models.ScrapingCommand{
			Action: models.StopScrapingAction,
		}

		// Publish command to RabbitMQ
		if err := h.publishCommand(command); err != nil {
			h.log.WithError(err).Error("Failed to publish stop command")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to stop scraper",
			})
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"message": "Stop command sent to scraper",
		})

		// Broadcast a message to all WebSocket clients
		h.broadcastStatus("Scraping process is being stopped", "info")
	}
}

// GetStatsHandler returns the current scraping statistics
func (h *Handler) GetStatsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, you would retrieve stats from a repository or service
		// For now, we'll return some default values
		c.JSON(http.StatusOK, gin.H{
			"total_page_scraped": 0,
			"total_scraped_link": 0,
			"video_downloaded":   0,
		})
	}
}

// UpdateConfigHandler updates the configuration
func (h *Handler) UpdateConfigHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BaseURL                 string `json:"base_url"`
			FromPages               int    `json:"from_pages"`
			ToPages                 int    `json:"to_pages"`
			LimitConcurrentDownload int    `json:"limit_concurrent_download"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			return
		}

		// In a real implementation, you would update the configuration
		// For now, we'll just return the received config
		c.JSON(http.StatusOK, gin.H{
			"message": "Configuration updated successfully",
			"config":  req,
		})
	}
}

// publishCommand publishes a command to the RabbitMQ command queue
func (h *Handler) publishCommand(command models.ScrapingCommand) error {
	return h.msgClient.PublishJSON(
		h.msgClient.(*messaging.RabbitMQClient).GetConfig().Exchange,
		CommandsRoutingKey,
		command,
	)
}

// broadcastStatus broadcasts a status message to all WebSocket clients
func (h *Handler) broadcastStatus(message string, status string) {
	wsMessage, err := json.Marshal(map[string]interface{}{
		"type":    "status",
		"message": message,
		"status":  status,
	})

	if err != nil {
		h.log.WithError(err).Error("Failed to marshal WebSocket status message")
		return
	}

	h.wsHub.Broadcast(wsMessage)
}
