package main

import (
	"context"
	"fmt"

	"wleowleo-scraper/config"
	"wleowleo-scraper/logger"
	"wleowleo-scraper/message"
	"wleowleo-scraper/scraper"

	"github.com/chromedp/chromedp"
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

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Setup ChromeDP
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(cfg.UserAgent),
		// chromedp.Flag("headless", false),
	)

	// Create browser context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Initialize scraper
	scrpr := scraper.New(cfg, log, producer)

	// Scrape pages
	// log.Info.Println("Starting page scraping...")
	log.Info("Starting page scraping...")
	links, err := scrpr.ScrapePage(browserCtx, cfg.FromPages, cfg.ToPages)
	if err != nil {
		log.WithError(err).Fatal("Error scraping pages")
	}

	// Scrape video links
	// log.Info.Println("Starting video link extraction...")
	log.Info("Starting video link extraction...")
	if err := scrpr.ScrapeVideo(allocCtx, links); err != nil {
		log.WithError(err).Fatal("Error scraping video links")
	}

	// Export results to file
	// result, err := scrpr.ExportLinks(links, "output")
	// if err != nil {
	// 	log.Error("Error exporting links:", err)
	// } else {
	// 	log.Info("Video links saved to", result)
	// }

	// Download videos if enabled
	if cfg.AutoDownload {
		for _, link := range *links {
			if link.M3U8 == "" {
				log.Warning("Skipping download for empty URL")
				continue
			}

			// Send message to RabbitMQ
			if err := producer.Produce(message.NewMessage(link.Title, link.Link, link.M3U8)); err != nil {
				log.WithError(err).Error("Error producing message")
			}
		}
	}

	// Print results
	log.Info("All pages processed and video links saved.")
	for _, link := range *links {
		fmt.Printf("Title: %s\nPage: %s\nVideo: %s\n\n", link.Title, link.Link, link.M3U8)
	}
}
