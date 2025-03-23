package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"wleowleo/config"
	"wleowleo/logger"
	"wleowleo/scraper"

	"github.com/chromedp/chromedp"
)

func main() {
	// Initialize logger
	log := logger.New()

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info.Println("Received shutdown signal. Cleaning up...")
		cancel()
	}()

	// Initialize configuration
	cfg := config.LoadConfig()

	// Setup ChromeDP
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(cfg.UserAgent),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Initialize scraper
	scrpr := scraper.New(cfg)

	// Scrape pages
	log.Info.Println("Starting page scraping...")
	links, err := scrpr.ScrapePage(browserCtx, cfg.Pages)
	if err != nil {
		log.Error.Fatal("Error scraping pages:", err)
	}

	// Scrape video links
	log.Info.Println("Starting video link extraction...")
	scrpr.ScrapeVideo(allocCtx, links)

	// Export results to file
	result, err := scrpr.ExportLinks(links, "output")
	if err != nil {
		log.Error.Printf("Error exporting links: %v", err)
	} else {
		log.Info.Printf("Links exported to %s directory", result)
	}

	// Download videos if enabled
	if cfg.AutoDownload {
		log.Info.Println("Starting video download...")
		for _, link := range *links {
			scrpr.DownloadVideo(link.M3U8)
		}
	}

	// Print results
	fmt.Println("Scraped Links:")
	for _, link := range *links {
		fmt.Printf("Title: %s\nPage: %s\nVideo: %s\n\n", link.Title, link.Link, link.M3U8)
	}

	log.Info.Println("All pages processed and video links saved.")
}
