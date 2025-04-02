package models

// Stats represents the statistics of the scraping and downloading process
type Stats struct {
	TotalPageScraped int `json:"total_page_scraped"`
	TotalScrapedLink int `json:"total_scraped_link"`
	VideoDownloaded  int `json:"video_downloaded"`
}

// Video represents a video information
type Video struct {
	Title    string        `json:"title"`
	URL      string        `json:"url"`
	M3U8     *string       `json:"m3u8,omitempty"`
	Progress *ProgressInfo `json:"progress,omitempty"`
}

// ProgressInfo represents the progress of the video downloading process
type ProgressInfo struct {
	TotalSegments int `json:"total_segments"`
	Downloaded    int `json:"downloaded"`
}

// ScrapLog represents a log message from the scraper
type ScrapLog struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   *Page  `json:"data,omitempty"`
	Stats  *Stats `json:"stats,omitempty"`
}

// VideoLog represents a log message from the video downloader
type VideoLog struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   Video  `json:"data"`
}

// ProcessResult represents the result of processing a URL
type ProcessResult struct {
	Page    Page
	Success bool
}
