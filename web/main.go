package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rizkirmdhn/wleowleo/web/config"
	"github.com/rizkirmdhn/wleowleo/web/handlers"
	"github.com/rizkirmdhn/wleowleo/web/websocket"
)

func main() {
	// Load configuration
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Error loading .env file: %v\n", err)
	}

	// Load configuration
	cfg := config.LoadConfig()

	_ = cfg
	// Set up Gin router
	r := gin.Default()

	// Set up static files
	r.Static("/static", "./static")

	// Set up templates
	r.LoadHTMLGlob("templates/*")

	// Create websocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Set up routes
	r.GET("/", handlers.IndexHandler())
	r.GET("/ws", func(c *gin.Context) {
		websocket.ServeWs(hub, c.Writer, c.Request)
	})

	// API routes
	api := r.Group("/api")
	{
		api.GET("/stats", handlers.GetStatsHandler())
		api.POST("/start", handlers.StartScrapingHandler(hub))
		api.POST("/config", handlers.UpdateConfigHandler())
	}

	// Create necessary directories if they don't exist
	os.MkdirAll(filepath.Join("static", "css"), 0755)
	os.MkdirAll(filepath.Join("static", "js"), 0755)
	os.MkdirAll("templates", 0755)

	// Start server
	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}
