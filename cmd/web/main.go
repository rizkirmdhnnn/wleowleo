package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/logger"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/web/handler"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Get the configuration
	// webCfg := cfg.GetWebPanelConfig()
	// scrpCfg := cfg.GetScraperConfig()
	// dlCfg := cfg.GetDownloaderConfig()

	// Initialize logger
	log := logger.New(cfg)

	// Print the Web panel configuration
	log.WithFields(logrus.Fields{
		"component": "web_main",
		"config":    fmt.Sprintf("%+v", cfg.WebPanel),
	}).Debug("Web panel configuration loaded")

	// Initialize message consumer
	msgClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": "web_main",
			"error":     err,
		}).Fatal("Failed to create RabbitMQ client")
	}
	defer msgClient.Close()

	// Check environment
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize the gin router
	r := gin.Default()

	// Set up static files
	r.Static("/static", "internal/web/static")

	// Set up templates
	r.LoadHTMLGlob("internal/web/templates/*")

	// Setup Handlers
	h := handler.NewHander(cfg, log, msgClient)

	// Register routes
	h.RegisterRoutes(r)

	// Start the web server
	log.WithFields(logrus.Fields{
		"component": "web_main",
		"port":      cfg.WebPanel.Port,
	}).Info("Server starting")
	if err := http.ListenAndServe(":"+strconv.Itoa(cfg.WebPanel.Port), r); err != nil {
		log.WithFields(logrus.Fields{
			"component": "web_main",
			"error":     err,
		}).Fatal("Failed to start server")
	}
}
