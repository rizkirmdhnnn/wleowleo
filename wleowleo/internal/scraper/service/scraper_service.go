package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/pkg/models"
	"github.com/sirupsen/logrus"
)

// Constants for the scraper service
const (
	CommandsRoutingKey   = "commands.scraper"
	VideoLinksRoutingKey = "video.links"
	ScraperLogRoutingKey = "scraper.log"

	DefaultMaxRetries = 3
	DefaultRetryDelay = 2 * time.Second
	DefaultTimeout    = 10 * time.Second
)

// ScraperService is the struct that holds the scraper service
type ScraperService struct {
	scrapCfg   *config.ScraperConfig
	rabbitCfg  *config.RabbitMQConfig
	log        *logrus.Logger
	httpClient *http.Client
	message    messaging.Client
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// NewScraperService creates a new ScraperService
func NewScraperService(scrapCfg *config.ScraperConfig, rabbitCfg *config.RabbitMQConfig, logger *logrus.Logger, msg messaging.Client) *ScraperService {
	return &ScraperService{
		scrapCfg:   scrapCfg,
		rabbitCfg:  rabbitCfg,
		log:        logger,
		httpClient: &http.Client{},
		message:    msg,
	}
}

// Start starts the ScraperService
func (s *ScraperService) Start() error {
	// Create a context that can be canceled for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	// Setup messaging infrastructure
	if err := s.setupMessaging(); err != nil {
		return fmt.Errorf("failed to set up messaging: %w", err)
	}

	// Consume messages
	return s.message.Consume(s.rabbitCfg.Queue.ScraperCommandQueue, func(msg []byte) error {
		return s.handleCommand(ctx, msg)
	})
}

// Stop gracefully stops the ScraperService
func (s *ScraperService) Stop() {
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil // Ensure we don't try to cancel the same context twice
	}
	// Wait for all goroutines to complete
	s.wg.Wait()
	s.log.Info("Scraper service stopped gracefully")
}

// setupMessaging sets up the messaging infrastructure
func (s *ScraperService) setupMessaging() error {
	// Declare and bind all needed queues
	queues := []struct {
		name       string
		routingKey string
	}{
		{s.rabbitCfg.Queue.ScraperCommandQueue, CommandsRoutingKey},
		{s.rabbitCfg.Queue.VideoLinksQueue, VideoLinksRoutingKey},
		{s.rabbitCfg.Queue.ScraperLogQueue, ScraperLogRoutingKey},
	}

	for _, q := range queues {
		if err := s.message.DeclareQueue(q.name); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		if err := s.message.BindQueue(q.name, s.rabbitCfg.Exchange, q.routingKey); err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", q.name, err)
		}
	}

	return nil
}

// handleCommand processes incoming commands
func (s *ScraperService) handleCommand(ctx context.Context, msg []byte) error {
	// Unmarshal the message
	var command models.ScrapingCommand
	if err := json.Unmarshal(msg, &command); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	// Log the command
	s.log.WithField("command", command).Info("Received command")

	// Check the command type and act accordingly
	switch command.Action {
	case models.StartScrapingAction:
		if ctx.Err() != nil {
			s.log.Info("Stopping scraping process before starting")
		}

		// Create a new context for the scraping process
		ctx, cancel := context.WithCancel(context.Background())
		s.cancelFunc = cancel

		// Get the parameters with sensible defaults
		startPage := getIntWithDefault(command.Data.StartPage, 1)
		endPage := getIntWithDefault(command.Data.EndPage, 5)

		// Track this goroutine with WaitGroup
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.StartScraping(ctx, startPage, endPage)
		}()

		return nil

	case models.StopScrapingAction:
		// Implement the stop scraping logic
		if s.cancelFunc != nil {
			s.log.Info("Stopping all scraping processes...")
			s.cancelFunc()
			s.cancelFunc = nil
			s.log.Info("Stop command sent, waiting for all processes to stop")
		} else {
			s.log.Info("No scraping processes are currently running")
		}
		return nil

	default:
		return fmt.Errorf("unknown command: %s", command.Action)
	}
}

// StartScraping starts the scraping process
func (s *ScraperService) StartScraping(ctx context.Context, startPage, endPage int) {
	s.log.WithFields(logrus.Fields{
		"startPage": startPage,
		"endPage":   endPage,
	}).Info("Starting scraping process")

	// Check if context is already canceled
	if ctx.Err() != nil {
		s.log.Info("Scraping canceled before it started")
		return
	}

	// Create Chrome contexts with proper lifecycle management
	allocCtx, allocCancel := s.createChromeContext(ctx)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(s.log.Printf))
	defer browserCancel()

	// Process the pages
	dataPage, err := s.processPage(browserCtx, startPage, endPage)
	if err != nil {
		if ctx.Err() != nil {
			s.log.Info("Scraping stopped during page processing")
		} else {
			s.log.WithError(err).Error("Error processing page")
		}
		return
	}

	// Check again before processing URLs
	if ctx.Err() != nil {
		s.log.Info("Scraping stopped before processing URLs")
		return
	}

	// Process the URLs
	if err := s.processUrls(allocCtx, dataPage); err != nil {
		if ctx.Err() != nil {
			s.log.Info("Scraping stopped during URL processing")
		} else {
			s.log.WithError(err).Error("Error processing URLs")
		}
		return
	}

	// Check if stopped due to stop command
	if ctx.Err() != nil {
		s.log.Info("Scraping terminated by stop command")
	} else {
		s.log.Info("Scraping completed successfully")
	}
}

// createChromeContext creates a new context for the Chrome browser
func (s *ScraperService) createChromeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(s.scrapCfg.UserAgent),
		// chromedp.WindowSize(1920, 1080),
		// chromedp.DisableGPU,
		// Add additional options for reliability in containerized environments
		// chromedp.NoFirstRun,
		// chromedp.NoDefaultBrowserCheck,
		// chromedp.Flag("headless", false),
	)
	return chromedp.NewExecAllocator(ctx, opts...)
}

// processPage scrapes the specified range of pages
func (s *ScraperService) processPage(ctx context.Context, fromPage, toPage int) (*models.DataPage, error) {
	var links []models.Page

	// For each page in the range
	for i := fromPage; i <= toPage; i++ {
		url := fmt.Sprintf("%s/page-%d", s.scrapCfg.Host, i)
		s.log.WithField("url", url).Info("Scraping page")

		var pageLinks []string
		var pageTitles []string

		// Using a timeout for each page scrape
		err := chromedp.Run(ctx,
			network.Enable(),
			network.SetBlockedURLs([]string{"*.png", "*.jpg", "*.jpeg", "*.gif"}),
			emulation.SetScriptExecutionDisabled(true),
			chromedp.Navigate(url),
			chromedp.Evaluate(`[...document.querySelectorAll('a[href*="watch/"]')].map(a => a.href)`, &pageLinks),
			chromedp.Evaluate(`[...document.querySelectorAll('a[href*="watch/"]')].map(a => a.title)`, &pageTitles),
		)

		if err != nil {
			s.log.WithError(err).WithField("url", url).Error("Error scraping page")
			continue // Try the next page instead of failing completely
		}

		// Process the links found on this page
		for j, link := range pageLinks {
			if strings.HasPrefix(link, "/") {
				link = s.scrapCfg.Host + link
			}
			title := "Unknown"
			if j < len(pageTitles) {
				title = pageTitles[j]
			}
			links = append(links, models.Page{
				Title: title,
				URL:   link,
			})
		}

		s.log.WithFields(logrus.Fields{
			"page":  i,
			"links": len(pageLinks),
		}).Info("Page scraped successfully")
	}

	// Cancel the context to free up resources
	chromedp.Cancel(ctx)
	s.log.WithField("total", len(links)).Info("Total links scraped")
	return &models.DataPage{
		Total: len(links),
		Urls:  links,
	}, nil
}

// processUrls processes the URLs found during scraping to extract m3u8 links
func (s *ScraperService) processUrls(ctx context.Context, dataModel *models.DataPage) error {
	if len(dataModel.Urls) == 0 {
		s.log.Warn("No URLs to process")
		return nil
	}

	// Create a single browser context for all URL processing
	browserCtx, cancel := chromedp.NewContext(ctx, chromedp.WithLogf(s.log.Printf))
	defer cancel()

	// Initialize browser with a blank page
	if err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank")); err != nil {
		return fmt.Errorf("failed to initialize browser: %w", err)
	}

	var totalUrlScraped int

	// Process URLs one by one
	for i, page := range dataModel.Urls {
		// Check if stop command received (context canceled)
		select {
		case <-ctx.Done():
			s.log.Info("Stopping URL processing - stop command received")
			return nil
		default:
			// Continue processing if no cancellation
		}

		s.log.WithFields(logrus.Fields{
			"progress": fmt.Sprintf("%d/%d", i+1, len(dataModel.Urls)),
			"title":    page.Title,
			"url":      page.URL,
		}).Info("Processing URL")

		// Process the URL with retries
		success := s.processURLWithRetries(browserCtx, page, dataModel, &totalUrlScraped)

		if !success {
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Warning("Failed to process URL after all retries")

			// Publish final failure log
			s.publishScraperLog("error", page, fmt.Errorf("failed after all retry attempts"),
				totalUrlScraped, len(dataModel.Urls))
		}

		// Short pause between URLs, but also check for stop command
		select {
		case <-ctx.Done():
			s.log.Info("Stopping processing during pause - stop command received")
			return nil
		case <-time.After(500 * time.Millisecond):
			// Continue after pause
		}
	}

	// Log final results
	s.log.WithFields(logrus.Fields{
		"total_page":    len(dataModel.Urls),
		"total_scraped": totalUrlScraped,
	}).Info("Completed processing video links")

	return nil
}

// processURLWithRetries processes a single URL with multiple retry attempts
func (s *ScraperService) processURLWithRetries(ctx context.Context, page models.Page,
	dataModel *models.DataPage, totalScraped *int) bool {

	maxRetries := DefaultMaxRetries
	retryDelay := DefaultRetryDelay

	// Check if context is already canceled (stop command received)
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Skipping URL processing - stop command received")
		return false
	}

	// First attempt
	result := s.processURL(ctx, page, DefaultTimeout)
	if result.Success {
		// Update stats
		*totalScraped++

		// Update original data model
		for i := range dataModel.Urls {
			if dataModel.Urls[i].URL == result.Page.URL {
				dataModel.Urls[i].M3u8 = result.Page.M3u8
				break
			}
		}

		// Publish success message
		s.publishVideoLink(result.Page)
		s.publishScraperLog("success", result.Page, nil, *totalScraped, len(dataModel.Urls))
		return true
	}

	// If context is canceled after first attempt, don't continue
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Stopping retry - stop command received")
		return false
	}

	// Retry logic
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check if context has been canceled before starting new retry
		if ctx.Err() != nil {
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Stopping retry process - stop command received")
			return false
		}

		s.log.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"title":   page.Title,
			"url":     page.URL,
		}).Info("Retrying URL")

		// Wait before retrying with interruptible select
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop() // Ensure timer is cleaned up
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Stopping during retry delay - stop command received")
			return false
		case <-timer.C:
			// Continue after delay
		}

		// Check again before starting new attempt
		if ctx.Err() != nil {
			return false
		}

		// Increase timeout for retries
		result := s.processURL(ctx, page, DefaultTimeout*(time.Duration(attempt+2)/2))

		if result.Success {
			// Update stats
			*totalScraped++

			// Update original data model
			for i := range dataModel.Urls {
				if dataModel.Urls[i].URL == result.Page.URL {
					dataModel.Urls[i].M3u8 = result.Page.M3u8
					break
				}
			}

			// Publish success message
			s.publishVideoLink(result.Page)
			s.publishScraperLog("success", result.Page, nil, *totalScraped, len(dataModel.Urls))
			return true
		}

		// Check again if context canceled after failed attempt
		if ctx.Err() != nil {
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Stopping after failed attempt - stop command received")
			return false
		}

		// Publish retry failure log
		s.publishScraperLog("retry", page,
			fmt.Errorf("retry attempt %d failed", attempt+1),
			*totalScraped, len(dataModel.Urls))
	}

	return false
}

// processURL processes a single URL and returns the result
func (s *ScraperService) processURL(ctx context.Context, page models.Page, timeout time.Duration) models.ProcessResult {
	result := models.ProcessResult{
		Page:    page,
		Success: false,
	}

	// Check context cancellation first
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Skipping URL processing - stop command received")
		return result
	}

	// Channel to receive the m3u8 link
	linkChan := make(chan string, 1)

	// Setup a listener for network events to catch m3u8 links
	listenerCtx, cancelListener := context.WithCancel(ctx)
	defer cancelListener()

	chromedp.ListenTarget(listenerCtx, func(ev any) {
		if e, ok := ev.(*network.EventRequestWillBeSent); ok {
			if strings.Contains(e.Request.URL, ".m3u8") {
				select {
				case linkChan <- e.Request.URL: // Send the link
				default: // Don't block if channel is full
				}
			}
		}
	})

	// Navigate to the URL in a separate goroutine
	errChan := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := chromedp.Run(ctx,
			network.Enable(),
			network.SetBlockedURLs([]string{"*.png", "*.jpg", "*.jpeg", "*.gif"}),
			chromedp.Navigate(page.URL),
		)
		select {
		case errChan <- err:
		case <-ctx.Done():
			// Context canceled, no need to send error
		}
	}()

	// Create timer for timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Wait for m3u8 link, error, timeout, or context cancellation
	select {
	case <-ctx.Done():
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Stopping URL processing - stop command received")
		// Clean up navigation goroutine
		cancelListener()
		// Wait for navigation goroutine to complete
		<-done
		// Try navigating to empty page to clean up
		cleanup := context.Background()
		cleanCtx, cleanCancel := chromedp.NewContext(cleanup)
		defer cleanCancel()
		go chromedp.Run(cleanCtx, chromedp.Navigate("about:blank"))
		return result

	case m3u8Link := <-linkChan:
		result.Page.M3u8 = m3u8Link
		result.Success = true
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
			"m3u8":  m3u8Link,
		}).Info("Successfully found video link")

		// Stop timer as it's no longer needed
		timer.Stop()

		// Navigate to blank page to stop any media loading and free resources
		cleanupErr := chromedp.Run(ctx, chromedp.Navigate("about:blank"))
		if cleanupErr != nil && ctx.Err() == nil {
			s.log.WithError(cleanupErr).Warn("Error while navigating to blank page")
		}

	case err := <-errChan:
		if err != nil && ctx.Err() == nil { // Don't log error if context was canceled
			s.log.WithError(err).WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Error("Error navigating to URL")
		}

	case <-timer.C:
		if ctx.Err() == nil { // Don't log timeout if context was canceled
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Warn("Timeout while processing URL")

			// Navigate to blank page to stop any ongoing requests
			cleanupErr := chromedp.Run(ctx, chromedp.Navigate("about:blank"))
			if cleanupErr != nil && ctx.Err() == nil {
				s.log.WithError(cleanupErr).Warn("Error while navigating to blank page")
			}
		}
	}

	return result
}

// publishVideoLink publishes a video link message
func (s *ScraperService) publishVideoLink(page models.Page) {
	s.message.PublishJSON(s.rabbitCfg.Exchange, VideoLinksRoutingKey, models.Video{
		Title: page.Title,
		URL:   page.URL,
		M3U8:  &page.M3u8,
	})
}

// publishScraperLog publishes a scraper log message
func (s *ScraperService) publishScraperLog(status string, page models.Page, err error, totalScraped, totalPages int) {
	log := models.ScrapLog{
		Status: status,
		Data: models.Page{
			Title: page.Title,
			URL:   page.URL,
			M3u8:  page.M3u8,
		},
		Stats: &models.Stats{
			TotalPageScraped: totalPages,
			TotalScrapedLink: totalScraped,
		},
	}

	if err != nil {
		log.Error = err.Error()
	}

	s.message.PublishJSON(s.rabbitCfg.Exchange, ScraperLogRoutingKey, log)
}

// Helper function to get int value with fallback default
func getIntWithDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}
