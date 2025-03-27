package models

// Action
const (
	StartScrapingAction = "start"
	StopScrapingAction  = "stop"
)

// StartScrapingCommand is the command to start scraping
type ScrapingCommand struct {
	Action string `json:"action"`
	Data   Data   `json:"data,omitempty"`
}

type Data struct {
	StartPage int `json:"startPage"`
	EndPage   int `json:"endPage"`
}
