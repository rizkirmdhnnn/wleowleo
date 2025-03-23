package scraper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ExportLinks exports the scraped links to a JSON file
func (s *Scraper) ExportLinks(links *[]PageLink, outputDir string) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, "links_"+timestamp+".json")

	// Convert links to JSON
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return "", err
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", err
	}

	return filename, nil
}
