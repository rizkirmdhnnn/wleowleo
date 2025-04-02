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
	CommandsRoutingKey = "scraper.start"

	ScraperLogRoutingKey    = "scraper.log"
	DownloaderLogRoutingKey = "downloader.log"
)

type Handler struct {
	cfg     *config.Config
	log     *logrus.Logger
	message messaging.Client
	wsHub   *websocket.Hub
}

func NewHander(cfg *config.Config, log *logrus.Logger, msg messaging.Client) *Handler {
	// Create WebSocket hub
	wsHub := websocket.NewHub(log)
	go wsHub.Run()

	handler := &Handler{
		cfg:     cfg,
		log:     log,
		message: msg,
		wsHub:   wsHub,
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
				"base_url":                h.cfg.WebPanel.Host,
				"FromPages":               1,
				"ToPages":                 2,
				"LimitConcurrentDownload": h.cfg.Downloader.DefaultWorker,
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
	// Setup consumer for logs
	err := h.message.Consume(h.cfg.RabbitMq.Queue.Log, func(message []byte, routingKey string) error {
		h.log.Info("Received message from RabbitMQ")
		h.log.Info("Routing key: ", routingKey)
		h.log.Info("Message: ", string(message))

		if routingKey == config.RoutingLogScraper {
			// Parse message
			var scrapLog models.ScrapLog
			if err := json.Unmarshal(message, &scrapLog); err != nil {
				h.log.WithError(err).Error("Failed to unmarshal scraper log message")
				return err
			}

			// Check if only stats are present
			if scrapLog.Data == nil && scrapLog.Error == "" {
				// Format message for WebSocket clients
				wsMessage, err := json.Marshal(map[string]any{
					"type":  "stats",
					"stats": scrapLog.Stats,
				})

				if err != nil {
					h.log.WithError(err).Error("Failed to marshal WebSocket message")
					return err
				}

				// Broadcast message to all WebSocket clients
				h.wsHub.Broadcast(wsMessage)
				h.log.WithField("message", string(wsMessage)).Info("Broadcasting stats to WebSocket clients")
				return nil
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
		}

		if routingKey == config.RoutingLogDownloader {
			// Parse message
			var dlLog models.VideoLog
			if err := json.Unmarshal(message, &dlLog); err != nil {
				h.log.WithError(err).Error("Failed to unmarshal downloader log message")
				return err
			}

			// Get progress info for the message
			var progress, total int
			if dlLog.Data.Progress != nil {
				progress = dlLog.Data.Progress.Downloaded
				total = dlLog.Data.Progress.TotalSegments
			}

			// Format message for WebSocket clients
			wsMessage, err := json.Marshal(map[string]any{
				"type":     "downloader_log",
				"status":   dlLog.Status,
				"message":  dlLog.Data.Title,
				"progress": progress,
				"total":    total,
				"error":    dlLog.Error,
			})

			if err != nil {
				h.log.WithError(err).Error("Failed to marshal WebSocket message")
				return err
			}

			// Broadcast message to all WebSocket clients
			h.wsHub.Broadcast(wsMessage)
			h.log.WithField("message", string(wsMessage)).Info("Broadcasting downloader log to WebSocket clients")
		}

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
		api.GET("/stats", h.GetStatsHandler())
		api.POST("/stop", h.StopScraperHandler())
		// api.POST("/config", h.UpdateConfigHandler())
	}
}

// StopScraperHandler handles requests to stop the scraper
func (h *Handler) StopScraperHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create command
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
			"message": "Scraper stopped successfully",
		})

		// Broadcast a message to all WebSocket clients
		h.broadcastStatus("Scraping process stopped", "info")
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
			req.FromPages = 1
		}
		if req.ToPages < req.FromPages {
			req.ToPages = req.FromPages
		}
		if req.ConcurrentDownload <= 0 {
			req.ConcurrentDownload = h.cfg.Downloader.DefaultWorker
		}

		// Create command
		command := models.ScrapingCommand{
			Action: models.StartScrapingAction,
			Data: models.Data{
				StartPage:  req.FromPages,
				EndPage:    req.ToPages,
				Concurrent: req.ConcurrentDownload,
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
	routingKeys := []string{
		config.RoutingCommandScraper,
		config.RoutingCommandDownloader,
	}

	for _, routingKey := range routingKeys {
		if err := h.message.PublishJSON(
			h.message.(*messaging.RabbitMQClient).GetConfig().Exchange,
			routingKey,
			command,
		); err != nil {
			return err
		}
	}

	if err := h.message.PublishJSON(
		h.message.(*messaging.RabbitMQClient).GetConfig().Exchange,
		CommandsRoutingKey,
		command,
	); err != nil {
		return err
	}
	return nil
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
