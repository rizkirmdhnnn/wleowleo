package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/logger"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/internal/web/handler"
)

func main() {
	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Get the configuration
	webCfg := cfg.GetWebPanelConfig()
	scrpCfg := cfg.GetScraperConfig()
	dlCfg := cfg.GetDownloaderConfig()

	// Initialize logger
	log := logger.New(cfg)

	// Print the Web panel configuration
	log.Infof("Web panel configuration: %+v", webCfg)

	// Initialize message consumer
	msgClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMq)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ client: %v", err)
	}
	defer msgClient.Close()

	// Initialize the gin router
	r := gin.Default()

	// Set up static files
	r.Static("/static", "internal/web/static")

	// Set up templates
	r.LoadHTMLGlob("internal/web/templates/*")

	// Setup Handlers
	h := handler.NewHander(webCfg, scrpCfg, dlCfg, log, msgClient)

	// Register routes
	h.RegisterRoutes(r)

	// Start the web server
	addr := fmt.Sprintf("%s:%d", webCfg.Host, webCfg.Port)
	log.Infof("Starting web server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}
