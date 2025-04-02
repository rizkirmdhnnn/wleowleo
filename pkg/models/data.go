package models

// Page represents a web page
type Page struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	M3u8  string `json:"m3u8"`
}

// DataPage represents a page with links
type DataPage struct {
	Total int    `json:"total"`
	Urls  []Page `json:"urls"`
}
