package scraper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"wleowleo-downloader/config"

	"github.com/sirupsen/logrus"
)

type Scraper struct {
	Config *config.Config
	Log    *logrus.Logger
}

type PageLink struct {
	Title string `json:"title"`
	Link  string `json:"url"`
	M3U8  string `json:"m3u8"`
}

// downloadJob represents a single download task
type downloadJob struct {
	index    int
	url      string
	fileName string
}

// New creates a new Scraper
func New(cfg *config.Config, log *logrus.Logger) *Scraper {
	return &Scraper{
		Config: cfg,
		Log:    log,
	}
}

// Download video from m3u8 link
func (s *Scraper) DownloadVideo(link PageLink) {
	s.Log.WithFields(logrus.Fields{
		"title": link.Title,
		"m3u8":  link.M3U8,
	}).Info("Downloading video")
	err := s.processM3U8File(link.M3U8)
	if err != nil {
		s.Log.Error("Error downloading video:", err)
		return
	}
	s.Log.WithFields(logrus.Fields{
		"title": link.Title,
		"m3u8":  link.M3U8,
	}).Info("Download completed")

}

// Convert .ts files to .mp4
func (s *Scraper) convertTStoMP4(foldername string, tsFiles []string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll("output", 0755); err != nil {
		s.Log.Error("Error creating output directory:", err)
		return fmt.Errorf("error creating output directory: %v", err)
	}

	listFileName := filepath.Join("temp", foldername, fmt.Sprintf("%s.txt", foldername))
	listFile, err := os.Create(listFileName)
	if err != nil {
		s.Log.Error("Error creating filelist:", err)
		return fmt.Errorf("error creating filelist: %v", err)
	}
	defer listFile.Close()

	// Write .ts file paths to list file
	for _, tsFile := range tsFiles {
		// Convert to absolute path and escape single quotes
		absPath, err := filepath.Abs(tsFile)
		if err != nil {
			s.Log.Error("Error getting absolute path:", err)
			return fmt.Errorf("error getting absolute path: %v", err)
		}
		// Escape single quotes in the path and ensure proper formatting
		escapedPath := strings.ReplaceAll(absPath, "'", "'\\''")
		_, err = listFile.WriteString(fmt.Sprintf("file '%s'\n", escapedPath))
		if err != nil {
			s.Log.Error("Error writing to filelist:", err)
			return fmt.Errorf("error writing to filelist: %v", err)
		}
	}

	// Output file name
	outputFile := filepath.Join("output", fmt.Sprintf("%s.mp4", foldername))

	// Run ffmpeg command to concatenate .ts files
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFileName, "-c:v", "copy", "-c:a", "copy", "-y", outputFile)
	err = cmd.Run()
	if err != nil {
		s.Log.Error("Error converting to mp4:", err)
		return fmt.Errorf("ffmpeg error: %v", err)
	}
	return nil
}

// Download file from URL
func (s *Scraper) downloadFile(url string, fileName string) error {
	resp, err := http.Get(url)
	if err != nil {
		s.Log.Error("Error downloading file:", err)
		return fmt.Errorf("error downloading file: %v", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(fileName)
	if err != nil {
		s.Log.Error("Error creating file:", err)
		return fmt.Errorf("error creating file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		s.Log.Error("Error writing to file:", err)
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

// Process M3U8 file
func (s *Scraper) processM3U8File(m3u8URL string) error {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll("temp", 0755); err != nil {
		s.Log.Error("Error creating temp directory:", err)
		return fmt.Errorf("error creating temp directory: %v", err)
	}

	// Find M3U8 file name
	format := regexp.MustCompile(`([^/]+)\.m3u8$`)
	link := format.FindStringSubmatch(m3u8URL)

	// Check if match is found
	if len(link) < 2 {
		s.Log.Error("Error finding m3u8 file name")
		return fmt.Errorf("error finding m3u8 file name")
	}

	basefolder := filepath.Join("temp", link[1])
	success := false

	// Setup cleanup
	defer func() {
		if !success {
			if err := os.RemoveAll(basefolder); err != nil {
				s.Log.Warn("Failed to cleanup temporary files:", err)
			}
		}
	}()

	// Create base folder (hanya sekali)
	if err := os.MkdirAll(basefolder, 0755); err != nil {
		s.Log.Error("Error creating temp directory:", err)
		return fmt.Errorf("error creating temp directory: %v", err)
	}

	// Download M3U8 file
	fileName := filepath.Join(basefolder, fmt.Sprintf("%s.m3u8", link[1]))
	if err := s.downloadFile(m3u8URL, fileName); err != nil {
		s.Log.Error("Error downloading m3u8 file:", err)
		return err
	}

	// Read M3U8 file
	data, err := os.ReadFile(fileName)
	if err != nil {
		s.Log.Error("Error reading m3u8 file:", err)
		return fmt.Errorf("error reading m3u8 file: %v", err)
	}

	// Find all .jpg URLs in the M3U8 file
	re := regexp.MustCompile(`https?://[^\s]+\.jpg`)
	matches := re.FindAllString(string(data), -1)

	var tsFiles []string
	tsFiles = make([]string, len(matches))

	// Use a wait group to wait for all downloads to complete
	var wg sync.WaitGroup
	// Use a mutex to protect access to the error variable
	var mu sync.Mutex
	var downloadErr error

	// Number of worker goroutines
	numWorkers := 10
	if len(matches) < numWorkers {
		numWorkers = len(matches)
	}

	// Create a channel to distribute work
	jobs := make(chan downloadJob, len(matches))

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Skip if an error has already occurred
				mu.Lock()
				if downloadErr != nil {
					mu.Unlock()
					continue
				}
				mu.Unlock()

				// Download the file
				err := s.downloadFile(job.url, job.fileName)
				if err != nil {
					mu.Lock()
					if downloadErr == nil {
						s.Log.Error("Error downloading segment", job.index, ":", err)
						downloadErr = fmt.Errorf("error downloading segment %d: %v", job.index, err)
					}
					mu.Unlock()
					continue
				}

				// Store the file name in the correct position
				tsFiles[job.index] = job.fileName
			}
		}()
	}

	// Send jobs to the workers
	for i, match := range matches {
		tsFileName := filepath.Join(basefolder, fmt.Sprintf("segment-%d.ts", i))
		jobs <- downloadJob{
			index:    i,
			url:      match,
			fileName: tsFileName,
		}
	}
	close(jobs)

	// Wait for all downloads to complete
	wg.Wait()

	// Check if any errors occurred during download
	if downloadErr != nil {
		s.Log.Error("Error downloading segments:", downloadErr)
		return downloadErr
	}

	// Convert .ts files to MP4
	err = s.convertTStoMP4(link[1], tsFiles)
	if err != nil {
		s.Log.Error("Error converting to mp4:", err)
		return err
	}

	success = true

	return err
}
