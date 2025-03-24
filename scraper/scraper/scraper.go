package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"
	"wleowleo-scraper/config"
	"wleowleo-scraper/message"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
)

type PageLink struct {
	Title string
	Link  string
	M3U8  string
}

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

func (s *Scraper) ScrapePage(ctx context.Context, fromPage, toPage int) (*[]PageLink, error) {
	var pageLinks []string
	var pageTitles []string
	var links []PageLink

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
			links = append(links, PageLink{title, link, ""})
		}
	}

	chromedp.Cancel(ctx)
	s.Log.Info("Scraped ", len(links), " links")
	return &links, nil
}

func (s *Scraper) ScrapeVideo(allocCtx context.Context, linkpage *[]PageLink) error {
	currentURL := ""
	title := ""
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	foundLink := make(chan bool, 1)

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

				// Update m3u8 link
				for i, link := range *linkpage {
					if link.Link == currentURL {
						(*linkpage)[i].M3U8 = e.Request.URL
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

	for _, link := range *linkpage {
		currentURL = link.Link
		title = link.Title
		s.Log.WithFields(logrus.Fields{
			"title": link.Title,
			"link":  link.Link,
		}).Debug("Scraping video link")

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

		select {
		case <-foundLink:
		case <-time.After(time.Duration(10) * time.Second):
			s.Log.Warning("Timeout for ", currentURL, ", continuing to next URL")
		}
	}

	return nil
}
