package main

import (
	"context"
	"fmt"
	"sync"

	"wleowleo/config"
	"wleowleo/logger"
	"wleowleo/scraper"

	"github.com/chromedp/chromedp"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize logrus
	log := logger.NewLogger(cfg)

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
	scrpr := scraper.New(cfg, log)

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
	result, err := scrpr.ExportLinks(links, "output")
	if err != nil {
		log.Error("Error exporting links:", err)
	} else {
		log.Info("Video links saved to", result)
	}

	// Download videos if enabled
	if cfg.AutoDownload {
		// log.Info.Println("Starting video download in parallel (limited to 5 concurrent downloads)...")
		log.Info("Starting video download in parallel (limited to 5 concurrent downloads)...")
		var wg sync.WaitGroup

		// Create a semaphore with buffer size 5 to limit concurrent downloads
		semaphore := make(chan struct{}, cfg.LimitConcurrent)

		for _, link := range *links {
			wg.Add(1)
			go func(url string) {
				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() {
					// Release semaphore when done
					<-semaphore
					wg.Done()
				}()

				if url == "" {
					log.Warning("Skipping download for empty URL")
					return
				}
				scrpr.DownloadVideo(url)
			}(link.M3U8)
		}

		// Wait for all downloads to complete
		wg.Wait()
		// log.Info.Println("All video downloads completed")
		log.Info("All video downloads completed")
	}

	// Print results
	fmt.Println("Scraped Links:")
	for _, link := range *links {
		fmt.Printf("Title: %s\nPage: %s\nVideo: %s\n\n", link.Title, link.Link, link.M3U8)
	}

	// log.Info.Println("All pages processed and video links saved.")
	log.Info("All pages processed and video links saved.")
}
