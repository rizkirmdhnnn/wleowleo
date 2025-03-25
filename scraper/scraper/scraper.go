package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"
	"wleowleo-scraper/config"
	"wleowleo-scraper/message"
	"wleowleo-scraper/model"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
)

type Scraper struct {
	Config  *config.Config
	Log     *logrus.Logger
	Message *message.Producer
}

func New(cfg *config.Config, log *logrus.Logger, msg *message.Producer) *Scraper {
	return &Scraper{
		Config:  cfg,
		Log:     log,
		Message: msg,
	}
}

func (s *Scraper) ScrapePage(ctx context.Context, fromPage, toPage int) (*model.DataPage, error) {
	var pageLinks []string
	var pageTitles []string
	var links []model.PageLink

	for i := fromPage; i <= toPage; i++ {
		url := fmt.Sprintf("%s/page-%d", s.Config.BaseURL, i)
		s.Log.Info("Scraping page: ", url)

		if err := chromedp.Run(ctx,
			network.Enable(),
			network.SetBlockedURLs([]string{"*.png", "*.jpg", "*.jpeg", "*.gif"}),
			emulation.SetScriptExecutionDisabled(true),
			chromedp.Navigate(url),
			chromedp.Evaluate(`[...document.querySelectorAll('a[href*="watch/"]')].map(a => a.href)`, &pageLinks),
			chromedp.Evaluate(`[...document.querySelectorAll('a[href*="watch/"]')].map(a => a.title)`, &pageTitles),
		); err != nil {
			s.Log.Error("Error scraping page: ", url)
			return nil, err
		}

		for j, link := range pageLinks {
			if strings.HasPrefix(link, "/") {
				link = s.Config.BaseURL + link
			}
			title := "Unknown"
			if j < len(pageTitles) {
				title = pageTitles[j]
			}
			links = append(links, model.PageLink{title, link, ""})
		}
	}

	chromedp.Cancel(ctx)
	s.Log.Info("Scraped ", len(links), " links")
	return &model.DataPage{
		Total: len(links),
		Links: links,
	}, nil
}

func (s *Scraper) ScrapeVideo(allocCtx context.Context, data *model.DataPage) error {
	var currentURL string
	var title string
	var totalScraped int

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	foundLink := make(chan bool, 1)
	// Maximum number of retries for each link
	maxRetries := 3
	// Delay between retries in seconds
	retryDelay := 2

	// Listen for network events
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if strings.Contains(e.Request.URL, ".m3u8") {
				s.Log.WithFields(logrus.Fields{
					"url":   e.Request.URL,
					"title": title,
				}).Info("Successfully found video link")

				// Produce message
				if err := s.Message.Produce(message.NewMessage(title, currentURL, e.Request.URL)); err != nil {
					s.Log.Error("Error producing message: ", err)
				}

				// Increment total scraped
				totalScraped += 1

				// Update m3u8 link
				for i := range data.Links {
					if data.Links[i].Link == currentURL {
						data.Links[i].M3U8 = e.Request.URL
						break
					}
				}

				// Send signal
				select {
				case foundLink <- true:
				default:
				}
			}
		}
	})

	// Create a slice to store links that failed to get m3u8
	var failedLinks []model.PageLink

	// First pass: try to scrape all links
	for _, link := range data.Links {
		currentURL = link.Link
		title = link.Title
		s.Log.WithFields(logrus.Fields{
			"title": link.Title,
			"link":  link.Link,
		}).Debug("Scraping video link")

		// Clear the channel before each attempt
		select {
		case <-foundLink:
		default:
		}

		go func(url string) {
			err := chromedp.Run(ctx,
				network.Enable(),
				network.SetBlockedURLs([]string{"*.png", "*.jpg", "*.jpeg", "*.gif"}),
				chromedp.Navigate(url),
			)
			if err != nil {
				s.Log.WithFields(logrus.Fields{
					"error": err,
					"title": link.Title,
					"link":  link.Link,
				}).Error("Scraping video link")
				foundLink <- true
				return
			}
		}(currentURL)

		// Wait for either the link to be found or timeout
		var linkFound bool
		select {
		case <-foundLink:
			linkFound = true
		case <-time.After(time.Duration(10) * time.Second):
			s.Log.Warning("Timeout for ", currentURL, ", will retry later")
			linkFound = false
		}

		// If link was not found, add to failed links for retry
		if !linkFound {
			failedLinks = append(failedLinks, link)
		}
	}

	// Retry failed links
	if len(failedLinks) > 0 {
		s.Log.WithFields(logrus.Fields{
			"count": len(failedLinks),
		}).Info("Retrying failed links")

		for retry := 0; retry < maxRetries; retry++ {
			if len(failedLinks) == 0 {
				break // All links have been successfully processed
			}

			s.Log.WithFields(logrus.Fields{
				"attempt": retry + 1,
				"count":   len(failedLinks),
			}).Info("Retry attempt")

			// Wait before retrying
			time.Sleep(time.Duration(retryDelay) * time.Second)

			// Create a new slice for links that still fail
			var stillFailedLinks []model.PageLink

			// Try each failed link again
			for _, link := range failedLinks {
				currentURL = link.Link
				title = link.Title
				s.Log.WithFields(logrus.Fields{
					"title":   link.Title,
					"link":    link.Link,
					"attempt": retry + 1,
				}).Debug("Retrying video link")

				// Clear the channel before each attempt
				select {
				case <-foundLink:
				default:
				}

				go func(url string) {
					err := chromedp.Run(ctx,
						network.Enable(),
						network.SetBlockedURLs([]string{"*.png", "*.jpg", "*.jpeg", "*.gif"}),
						chromedp.Navigate(url),
					)
					if err != nil {
						s.Log.WithFields(logrus.Fields{
							"error":   err,
							"title":   link.Title,
							"link":    link.Link,
							"attempt": retry + 1,
						}).Error("Retrying video link failed")
						foundLink <- true
						return
					}
				}(currentURL)

				// Wait for either the link to be found or timeout
				var linkFound bool
				select {
				case <-foundLink:
					linkFound = true
				case <-time.After(time.Duration(10) * time.Second):
					s.Log.WithFields(logrus.Fields{
						"title":   link.Title,
						"attempt": retry + 1,
					}).Warning("Retry timeout")
					linkFound = false
				}

				// If link still not found, add to still failed links
				if !linkFound {
					stillFailedLinks = append(stillFailedLinks, link)
				} else {
					// Check if the link was actually updated with m3u8
					var linkUpdated bool
					for _, updatedLink := range data.Links {
						if updatedLink.Link == link.Link && updatedLink.M3U8 != "" {
							linkUpdated = true
							break
						}
					}

					if !linkUpdated {
						s.Log.WithFields(logrus.Fields{
							"title":   link.Title,
							"attempt": retry + 1,
						}).Warning("Link found but m3u8 not updated, adding to retry list")
						stillFailedLinks = append(stillFailedLinks, link)
					}
				}
			}

			// Update the failed links for next retry
			failedLinks = stillFailedLinks

			// Log the progress
			s.Log.WithFields(logrus.Fields{
				"attempt":      retry + 1,
				"still_failed": len(failedLinks),
			}).Info("Retry attempt completed")
		}

		// Log final results
		if len(failedLinks) > 0 {
			s.Log.WithFields(logrus.Fields{
				"count": len(failedLinks),
			}).Warning("Some links could not be processed after all retries")

			// Update the original links that still failed with empty m3u8
			for _, failedLink := range failedLinks {
				for i := range data.Links {
					if data.Links[i].Link == failedLink.Link {
						// Keep M3U8 empty to indicate failure
						s.Log.WithFields(logrus.Fields{
							"title": data.Links[i].Title,
						}).Warning("Failed to get m3u8 link after all retries")
					}
				}
			}
		} else {
			s.Log.Info("All links successfully processed after retries")
		}
	}

	// Log final results
	s.Log.WithFields(logrus.Fields{
		"total_link":    len(data.Links),
		"total_scraped": totalScraped,
	}).Info("Scraped video links")

	return nil
}
