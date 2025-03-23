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
	// Initialize logger
	log := logger.New()

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize configuration
	cfg := config.LoadConfig()

	// Setup ChromeDP
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(cfg.UserAgent),
		//  chromedp.Flag("headless", false),
	)

	// Create browser context
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
		log.Info.Println("Starting video download in parallel (limited to 5 concurrent downloads)...")
		var wg sync.WaitGroup

		// Create a semaphore with buffer size 5 to limit concurrent downloads
		semaphore := make(chan struct{}, 5)

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
					log.Warning.Println("No video link found for this page")
					return
				}
				scrpr.DownloadVideo(url)
			}(link.M3U8)
		}

		// Wait for all downloads to complete
		wg.Wait()
		log.Info.Println("All video downloads completed")
	}

	// Print results
	fmt.Println("Scraped Links:")
	for _, link := range *links {
		fmt.Printf("Title: %s\nPage: %s\nVideo: %s\n\n", link.Title, link.Link, link.M3U8)
	}

	log.Info.Println("All pages processed and video links saved.")
}
