package handler

import (
	"context"
	"fmt"
	"net/http"
	"wleowleo-scraper/config"
	"wleowleo-scraper/message"
	"wleowleo-scraper/model"
	"wleowleo-scraper/scraper"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type StartRequest struct {
	FromPages          int `json:"from_pages" binding:"required,min=1"`
	ToPages            int `json:"to_pages" binding:"required,min=1,gtfield=FromPages"`
	ConcurrentDownload int `json:"concurrent_download" binding:"required,min=1"`
}

type Handler struct {
	cfg      *config.Config
	log      *logrus.Logger
	producer *message.Producer
}

func NewHandler(cfg *config.Config, log *logrus.Logger, producer *message.Producer) *Handler {
	return &Handler{
		cfg:      cfg,
		log:      log,
		producer: producer,
	}
}

func (h *Handler) Start() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req StartRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			h.log.WithError(err).Error("Failed to bind request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		h.updateConfig(req)
		go h.startScraping()

		c.JSON(http.StatusOK, gin.H{
			"message": "Scraping started successfully",
			"data": gin.H{
				"from_pages":          req.FromPages,
				"to_pages":            req.ToPages,
				"concurrent_download": req.ConcurrentDownload,
			},
		})
	}
}

func (h *Handler) updateConfig(req StartRequest) {
	h.cfg.FromPages = req.FromPages
	h.cfg.ToPages = req.ToPages
	h.cfg.ConcurrentDownload = req.ConcurrentDownload
}

func (h *Handler) startScraping() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	allocCtx, allocCancel := h.createChromeContext(ctx)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(h.log.Printf))
	defer browserCancel()

	if err := h.runScraping(browserCtx, allocCtx); err != nil {
		h.log.WithError(err).Error("Scraping failed")
	}
}

func (h *Handler) createChromeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(h.cfg.UserAgent),
		chromedp.WindowSize(1920, 1080),
		// chromedp.NoSandbox,
	)
	return chromedp.NewExecAllocator(ctx, opts...)
}

func (h *Handler) runScraping(browserCtx, allocCtx context.Context) error {
	scrpr := scraper.New(h.cfg, h.log, h.producer)

	h.log.Info("Starting page scraping...")
	links, err := scrpr.ScrapePage(browserCtx, h.cfg.FromPages, h.cfg.ToPages)
	if err != nil {
		return fmt.Errorf("failed to scrape pages: %w", err)
	}

	h.log.Info("Starting video link extraction...")
	if err := scrpr.ScrapeVideo(allocCtx, links); err != nil {
		return fmt.Errorf("failed to scrape video links: %w", err)
	}

	h.log.Info("All pages processed and video links saved successfully")
	h.logScrapedLinks(links)
	return nil
}

func (h *Handler) logScrapedLinks(links *model.DataPage) {
	for _, link := range links.Urls {
		h.log.WithFields(logrus.Fields{
			"title": link.Title,
			"page":  link.Url,
			"video": link.Url,
		}).Info("Scraped link")
	}
}
