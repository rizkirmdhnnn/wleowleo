package main

import (
	"context"
	"fmt"

	"wleowleo/config"
	"wleowleo/logger"
	"wleowleo/scraper"

	"wleowleo/message"

	"github.com/chromedp/chromedp"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize logrus
	log := logger.NewLogger(cfg)

	// Initialize rabbitmq consumer
	consumer := message.NewProducer(cfg, log)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Setup ChromeDP
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(cfg.UserAgent),
	)

	// Create browser context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Initialize scraper
	scrpr := scraper.New(cfg, log, consumer)

	// Scrape pages
	// log.Info.Println("Starting page scraping...")
	log.Info("Starting page scraping...")
	links, err := scrpr.ScrapePage(browserCtx, cfg.Pages)
	if err != nil {
		log.Error("Error scraping pages:", err)
	}

	// Scrape video links
	// log.Info.Println("Starting video link extraction...")
	log.Info("Starting video link extraction...")
	scrpr.ScrapeVideo(allocCtx, links)

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

			// Produce message
			msg := message.NewMessage(link.Title, link.Link, link.M3U8)

			log.Info("Producing message:", msg)
			log.Info(consumer)

			consumer.Produce(msg)
			// err := scrpr.DownloadVideo(link)
			if err != nil {
				log.Error("Error downloading video:", err)
			}
		}
	}

	// Print results
	fmt.Println("Scraped Links:")
	for _, link := range *links {
		fmt.Printf("Title: %s\nPage: %s\nVideo: %s\n\n", link.Title, link.Link, link.M3U8)
	}

	// log.Info.Println("All pages processed and video links saved.")
	log.Info("All pages processed and video links saved.")
}
