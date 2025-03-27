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
	ScraperCommandQueue  = "scraper_commands"
	VideoLinksQueue      = "scraper_video_queue"
	ScraperLogQueue      = "scraper_log"
	ExchangeName         = "wleowleo"
	CommandsRoutingKey   = "commands.scraper"
	VideoLinksRoutingKey = "video.links"
	ScraperLogRoutingKey = "scraper.log"

	DefaultMaxRetries = 3
	DefaultRetryDelay = 2 * time.Second
	DefaultTimeout    = 10 * time.Second
)

// ScraperService is the struct that holds the scraper service
type ScraperService struct {
	config     *config.ScraperConfig
	log        *logrus.Logger
	httpClient *http.Client
	message    messaging.Client
	// Add context cancellation to support graceful shutdown
	cancelFunc context.CancelFunc
	// Add WaitGroup to wait for all goroutines to finish
	wg sync.WaitGroup
}

// NewScraperService creates a new ScraperService
func NewScraperService(config *config.ScraperConfig, logger *logrus.Logger, msg messaging.Client) *ScraperService {
	return &ScraperService{
		config:     config,
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
	return s.message.Consume(ScraperCommandQueue, func(msg []byte) error {
		return s.handleCommand(ctx, msg)
	})
}

// Stop gracefully stops the ScraperService
func (s *ScraperService) Stop() {
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil // Pastikan kita tidak mencoba membatalkan context yang sama dua kali
	}
	// Tunggu semua goroutine selesai
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
		{ScraperCommandQueue, CommandsRoutingKey},
		{VideoLinksQueue, VideoLinksRoutingKey},
		{ScraperLogQueue, ScraperLogRoutingKey},
	}

	for _, q := range queues {
		if err := s.message.DeclareQueue(q.name); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		if err := s.message.BindQueue(q.name, ExchangeName, q.routingKey); err != nil {
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
			s.log.Info("Menghentikan proses scraping sebelum memulai")
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
			s.log.Info("Menghentikan semua proses scraping...")
			s.cancelFunc()
			s.cancelFunc = nil
			s.log.Info("Perintah stop telah dikirim, menunggu semua proses berhenti")
		} else {
			s.log.Info("Tidak ada proses scraping yang berjalan")
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

	// Periksa apakah context sudah dibatalkan
	if ctx.Err() != nil {
		s.log.Info("Scraping dibatalkan sebelum dimulai")
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
			s.log.Info("Scraping dihentikan selama pemrosesan halaman")
		} else {
			s.log.WithError(err).Error("Error processing page")
		}
		return
	}

	// Periksa lagi sebelum memproses URL
	if ctx.Err() != nil {
		s.log.Info("Scraping dihentikan sebelum memproses URL")
		return
	}

	// Process the URLs
	if err := s.processUrls(allocCtx, dataPage); err != nil {
		if ctx.Err() != nil {
			s.log.Info("Scraping dihentikan selama pemrosesan URL")
		} else {
			s.log.WithError(err).Error("Error processing URLs")
		}
		return
	}

	// Periksa apakah berhenti karena perintah stop
	if ctx.Err() != nil {
		s.log.Info("Scraping dihentikan oleh perintah stop")
	} else {
		s.log.Info("Scraping complete")
	}
}

// createChromeContext creates a new context for the Chrome browser
func (s *ScraperService) createChromeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(s.config.UserAgent),
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
		url := fmt.Sprintf("%s/page-%d", s.config.Host, i)
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
				link = s.config.Host + link
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
		// Periksa apakah ada perintah stop (context dibatalkan)
		select {
		case <-ctx.Done():
			s.log.Info("Menghentikan proses pengolahan URL - perintah stop diterima")
			return nil
		default:
			// Lanjutkan pemrosesan jika tidak ada pembatalan
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

		// Jeda pendek antara URL, tapi juga periksa perintah stop
		select {
		case <-ctx.Done():
			s.log.Info("Menghentikan pengolahan selama jeda - perintah stop diterima")
			return nil
		case <-time.After(500 * time.Millisecond):
			// Lanjutkan setelah jeda
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

	// Periksa jika context sudah dibatalkan (perintah stop diterima)
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Melewatkan pemrosesan URL - perintah stop diterima")
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

	// Jika context sudah dibatalkan setelah percobaan pertama, jangan lanjutkan
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Menghentikan retry - perintah stop diterima")
		return false
	}

	// Logika percobaan ulang (retry)
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Periksa apakah context telah dibatalkan sebelum memulai retry baru
		if ctx.Err() != nil {
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Menghentikan proses retry - perintah stop diterima")
			return false
		}

		s.log.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"title":   page.Title,
			"url":     page.URL,
		}).Info("Retrying URL")

		// Tunggu sebelum mencoba ulang dengan select yang bisa diinterupsi
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop() // Pastikan timer dibersihkan
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Berhenti selama delay retry - perintah stop diterima")
			return false
		case <-timer.C:
			// Lanjutkan setelah jeda
		}

		// Periksa lagi sebelum memulai percobaan baru
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

		// Periksa lagi jika konteks dibatalkan setelah upaya gagal
		if ctx.Err() != nil {
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Info("Berhenti setelah percobaan gagal - perintah stop diterima")
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

	// Periksa pembatalan context terlebih dahulu
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Melewatkan pemrosesan URL - perintah stop diterima")
		return result
	}

	// Channel to receive the m3u8 link
	linkChan := make(chan string, 1)

	// Setup a listener for network events to catch m3u8 links
	listenerCtx, cancelListener := context.WithCancel(ctx)
	defer cancelListener()

	chromedp.ListenTarget(listenerCtx, func(ev interface{}) {
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
			// Context dibatalkan, tidak perlu mengirim error
		}
	}()

	// Buat timer untuk timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Tunggu link m3u8, error, timeout, atau pembatalan context
	select {
	case <-ctx.Done():
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"url":   page.URL,
		}).Info("Menghentikan pemrosesan URL - perintah stop diterima")
		// Bersihkan goroutine untuk navigasi
		cancelListener()
		// Tunggu goroutine navigasi selesai
		<-done
		// Coba navigasi ke halaman kosong untuk membersihkan
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

		// Hentikan timer karena tidak dibutuhkan lagi
		timer.Stop()

		// Navigate to blank page to stop any media loading and free resources
		cleanupErr := chromedp.Run(ctx, chromedp.Navigate("about:blank"))
		if cleanupErr != nil && ctx.Err() == nil {
			s.log.WithError(cleanupErr).Warn("Error while navigating to blank page")
		}

	case err := <-errChan:
		if err != nil && ctx.Err() == nil { // Jangan log error jika context dibatalkan
			s.log.WithError(err).WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Error("Error navigating to URL")
		}

	case <-timer.C:
		if ctx.Err() == nil { // Jangan log timeout jika context dibatalkan
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
	s.message.PublishJSON(ExchangeName, VideoLinksRoutingKey, models.Video{
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

	s.message.PublishJSON(ExchangeName, ScraperLogRoutingKey, log)
}

// Helper function to get int value with fallback default
func getIntWithDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}
