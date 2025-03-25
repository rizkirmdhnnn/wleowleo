package model

// PageLink represents a link to a page
type PageLink struct {
	Title string
	Link  string
	M3U8  string
}

// DataPage represents a page of data
type DataPage struct {
	Total int
	Links []PageLink
}

// Message represents a message
type Message struct {
	Message string
	Status  string
}
