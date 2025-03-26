package main

import (
	"net/http"
	"wleowleo-scraper/config"
	"wleowleo-scraper/handler"
	"wleowleo-scraper/logger"
	"wleowleo-scraper/message"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize logrus
	log := logger.NewLogger(cfg)

	// Initialize rabbitmq consumer
	producer := message.NewProducer(cfg, log)
	if err := producer.Initialize(); err != nil {
		log.WithError(err).Fatal("Error initializing producer")
	}

	// Initialize Gin router
	router := gin.Default()

	// Set up CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
	})

	// Register routes
	handler := handler.NewHandler(cfg, log, producer)
	router.POST("/api/start", handler.Start())

	// Start the server
	log.Printf("Server starting on port %s\n", cfg.WebPort)
	if err := http.ListenAndServe(":"+cfg.WebPort, router); err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}
