package model

// PageLink represents a link to a page
type PageLink struct {
	Title string `json:"title"`
	Url   string `json:"url"`
	M3u8  string `json:"m3u8"`
}

// DataPage represents a page of data
type DataPage struct {
	Total int
	Urls  []PageLink
}

// Stats represents the statistics of the scraping and downloading process
type Stats struct {
	TotalPageScraped int `json:"total_page_scraped"`
	TotalScrapedLink int `json:"total_scraped_link"`
	VideoDownloaded  int `json:"video_downloaded"`
}

// ScrapLog represents a log message from the scraper
type ScrapLog struct {
	Type   string   `json:"type"`
	Status string   `json:"status"`
	Error  string   `json:"error,omitempty"`
	Data   PageLink `json:"data"`
	Stats  Stats    `json:"stats,omitempty"`
}
