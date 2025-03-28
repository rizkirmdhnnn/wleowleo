package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rizkirmdhn/wleowleo/internal/common/config"
	"github.com/rizkirmdhn/wleowleo/internal/common/messaging"
	"github.com/rizkirmdhn/wleowleo/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	ConfigRoutingKey = "downloader.config"
	TaskRoutingKey   = "downloader.task"
	LogRoutingKey    = "downloader.log"
)

// TODO: Implement 2 queue for downloader service and scraper service

// downloadJob represents a single download task
type downloadJob struct {
	index    int
	url      string
	fileName string
}
type DownloaderService struct {
	config    *config.DownloaderConfig
	rabbitCfg *config.RabbitMQConfig
	log       *logrus.Logger
	message   messaging.Client
}

func NewDownloaderService(cfg *config.DownloaderConfig, rabbitCfg *config.RabbitMQConfig, log *logrus.Logger, message messaging.Client) *DownloaderService {
	return &DownloaderService{
		config:    cfg,
		rabbitCfg: rabbitCfg,
		log:       log,
		message:   message,
	}
}

// // Start starts the Downloader service
// func (s *DownloaderService) Start() error {
// 	// Setup message consumer
// 	if err := s.setupMessaging(); err != nil {
// 		return fmt.Errorf("failed to setup messaging: %w", err)
// 	}

// 	return s.message.Consume(s.rabbitCfg.Queue.VideoLinksQueue, func(msg []byte) error {
// 		return s.handleMessage(msg)
// 	})

// }

func (s *DownloaderService) handleMessage(msg []byte) error {
	// Unmarshal the message
	var command models.ScrapingCommand
	if err := json.Unmarshal(msg, &command); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	// Log the command
	s.log.Info("Received command to download video", command.Data.Concurrent)

	return s.prepare(command.Data.Concurrent)
}

// Setup message consumer
func (s *DownloaderService) setupMessaging() error {
	queues := []struct {
		name        string
		exchange    string
		routingKeys []string
	}{
		{
			name:        s.rabbitCfg.Queue.DownloaderQueue,
			exchange:    s.rabbitCfg.Exchange.Task,
			routingKeys: []string{TaskRoutingKey, ConfigRoutingKey},
		},
		{
			name:        s.rabbitCfg.Queue.LogQueue,
			exchange:    s.rabbitCfg.Exchange.Log,
			routingKeys: []string{LogRoutingKey},
		},
	}

	for _, q := range queues {
		// Declare queue
		if err := s.message.DeclareQueue(q.name); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		// Bind queue to each routing key
		for _, key := range q.routingKeys {
			if err := s.message.BindQueue(q.name, q.exchange, key); err != nil {
				return fmt.Errorf("failed to bind queue %s to exchange %s with key %s: %w", q.name, q.exchange, key, err)
			}
		}
	}

	return nil
}

// Download video from m3u8 link
func (s *DownloaderService) DownloadVideo(urls models.Page) error {
	s.log.WithFields(logrus.Fields{"title": urls.Title, "m3u8": urls.M3u8}).Debug("Downloading video")
	err := s.processM3U8File(urls.M3u8, urls.Title)
	if err != nil {
		return err
	}
	s.log.WithFields(logrus.Fields{"title": urls.Title, "m3u8": urls.M3u8}).Debug("Download completed")

	return nil
}

// Process M3U8 file
func (s *DownloaderService) processM3U8File(m3u8URL string, title string) error {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll("temp", 0755); err != nil {
		return fmt.Errorf("error creating temp directory: %v", err)
	}

	// Find M3U8 file name
	format := regexp.MustCompile(`([^/]+)\.m3u8$`)
	link := format.FindStringSubmatch(m3u8URL)

	// Check if match is found
	if len(link) < 2 {
		return fmt.Errorf("error finding m3u8 file name")
	}

	basefolder := filepath.Join("temp", link[1])

	// Create base folder
	if err := os.MkdirAll(basefolder, 0755); err != nil {
		return fmt.Errorf("error creating temp directory: %v", err)
	}

	// Download M3U8 file
	fileName := filepath.Join(basefolder, fmt.Sprintf("%s.m3u8", link[1]))
	s.log.Debug("Downloading M3U8 file")
	if err := s.downloadFile(m3u8URL, fileName); err != nil {
		return fmt.Errorf("failed to download M3U8 file: %v", err)
	}

	// Read M3U8 file
	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("error reading m3u8 file: %v", err)
	}

	// Find all .jpg URLs in the M3U8 file
	re := regexp.MustCompile(`https?://[^\s]+\.jpg`)
	matches := re.FindAllString(string(data), -1)

	if len(matches) == 0 {
		return fmt.Errorf("no video segments found in M3U8 file")
	}

	s.log.WithFields(logrus.Fields{
		"segments": len(matches),
	}).Debug("Found video segments")

	tsFiles := make([]string, len(matches))

	// Use a wait group to wait for all downloads to complete
	var wg sync.WaitGroup
	// Use a mutex to protect access to the error variable
	var mu sync.Mutex
	var downloadErr error

	// Use the configured number of workers or default to a reasonable number
	numWorkers := s.config.Concurrency
	if numWorkers <= 0 {
		numWorkers = 10 // Default if not configured properly
	}
	if len(matches) < numWorkers {
		numWorkers = len(matches)
	}

	s.log.WithFields(logrus.Fields{
		"workers": numWorkers,
	}).Debug("Starting download with workers")

	// Create a channel to distribute work
	jobs := make(chan downloadJob, len(matches))

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				// Skip if an error has already occurred
				mu.Lock()
				if downloadErr != nil {
					mu.Unlock()
					continue
				}
				mu.Unlock()

				// Log worker activity with worker ID and title
				s.log.WithFields(logrus.Fields{
					"worker_id": workerID,
					"segment":   job.index,
					"title":     title,
				}).Debug("Worker downloading segment")

				// Download the file
				err := s.downloadFile(job.url, job.fileName)
				if err != nil {
					mu.Lock()
					if downloadErr == nil {
						downloadErr = fmt.Errorf("error downloading segment %d: %v", job.index, err)
					}
					mu.Unlock()
					continue
				}

				// Store the file name in the correct position
				tsFiles[job.index] = job.fileName

				// Log successful download with worker ID and title
				s.log.WithFields(logrus.Fields{
					"worker_id": workerID,
					"segment":   job.index,
					"title":     title,
				}).Debug("Worker completed segment")
			}
		}(w)
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
		return downloadErr
	}

	s.log.Info("All segments downloaded successfully, converting to MP4")

	// Convert .ts files to MP4
	err = s.convertTStoMP4(link[1], tsFiles)
	if err != nil {
		return fmt.Errorf("error converting to MP4: %v", err)
	}

	// Cleanup temporary files after successful conversion
	if err := os.RemoveAll(basefolder); err != nil {
		s.log.Warn("Failed to cleanup temporary files after successful conversion:", err)
	} else {
		s.log.Debug("Successfully cleaned up temporary files")
	}

	return nil
}

// Download file from URL with retry mechanism
func (s *DownloaderService) downloadFile(url string, fileName string) error {
	// Create a custom HTTP client with timeout
	client := &http.Client{}

	// Maximum number of retries
	maxRetries := 3
	var lastErr error

	// Try to download with retries
	for attempt := range maxRetries {
		if attempt > 0 {
			s.log.WithFields(logrus.Fields{
				"url":      url,
				"fileName": fileName,
				"attempt":  attempt + 1,
				"error":    lastErr,
			}).Debug("Retrying download")
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		}

		// Make the request
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("error downloading file (attempt %d): %v", attempt+1, err)
			continue
		}

		// Create the output file
		out, err := os.Create(fileName)
		if err != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("error creating file (attempt %d): %v", attempt+1, err)
			continue
		}

		// Copy the response body to the output file
		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()

		if err != nil {
			lastErr = fmt.Errorf("error writing to file (attempt %d): %v", attempt+1, err)
			continue
		}

		// If we get here, the download was successful
		return nil
	}

	// If we get here, all retries failed
	return lastErr
}

// Convert .ts files to .mp4 with progress reporting
func (s *DownloaderService) convertTStoMP4(foldername string, tsFiles []string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll("output", 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	listFileName := filepath.Join("temp", foldername, fmt.Sprintf("%s.txt", foldername))
	listFile, err := os.Create(listFileName)
	if err != nil {
		return fmt.Errorf("error creating filelist: %v", err)
	}
	defer listFile.Close()

	// Write .ts file paths to list file
	s.log.Debug("Preparing file list for conversion")
	for i, tsFile := range tsFiles {
		// Verify file exists
		if _, err := os.Stat(tsFile); os.IsNotExist(err) {
			return fmt.Errorf("segment file missing: %s", tsFile)
		}

		// Convert to absolute path and escape single quotes
		absPath, err := filepath.Abs(tsFile)
		if err != nil {
			return fmt.Errorf("error getting absolute path: %v", err)
		}
		// Escape single quotes in the path and ensure proper formatting
		escapedPath := strings.ReplaceAll(absPath, "'", "'\\''")
		_, err = listFile.WriteString(fmt.Sprintf("file '%s'\n", escapedPath))
		if err != nil {
			return fmt.Errorf("error writing to filelist: %v", err)
		}

		// Log progress for large files
		if len(tsFiles) > 100 && i%50 == 0 {
			s.log.WithFields(logrus.Fields{
				"progress": fmt.Sprintf("%.1f%%", float64(i)/float64(len(tsFiles))*100),
			}).Debug("Preparing file list")
		}
	}

	// Output file name
	outputFile := filepath.Join("output", fmt.Sprintf("%s.mp4", foldername))
	s.log.WithFields(logrus.Fields{
		"output":   outputFile,
		"segments": len(tsFiles),
	}).Info("Starting conversion to MP4")

	// Run ffmpeg command to concatenate .ts files with progress reporting
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFileName, "-c:v", "copy", "-c:a", "copy", "-y", outputFile)

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting ffmpeg: %v", err)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg error: %v", err)
	}

	s.log.WithFields(logrus.Fields{
		"output": outputFile,
	}).Info("Conversion completed successfully")

	return nil
}

// Stop stops the Downloader service
func (s *DownloaderService) Stop() error {
	s.log.Info("Downloader service stopped successfully")
	return nil
}

// MessageJob represents a job to be processed by a worker
type MessageJob struct {
	Page models.Page
}

// Start starts the Downloader service
func (s *DownloaderService) Start() error {
	// Setup message consumer
	if err := s.setupMessaging(); err != nil {
		return fmt.Errorf("failed to setup messaging: %w", err)
	}

	// Start consuming messages
	return s.message.Consume(s.rabbitCfg.Queue.DownloaderQueue, func(msg []byte, routingKey string) error {
		return s.handleMessage(msg)
	})
}

func (s *DownloaderService) prepare(worker int) error {
	// Create a channel to distribute work
	jobs := make(chan MessageJob, s.config.Concurrency)
	// Create a semaphore channel to limit concurrency
	sem := make(chan struct{}, s.config.Concurrency)

	// Wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Start workers
	for w := 1; w <= worker; w++ {
		wg.Add(1)
		go s.worker(w, &wg, jobs, sem)
	}

	// Start consuming messages
	return s.message.Consume(s.rabbitCfg.Queue.DownloaderQueue, func(msg []byte, routingKey string) error {
		// Check routing key
		if routingKey == ConfigRoutingKey {
			// Unmarshal the message
			var cfg models.ScrapingCommand
			if err := json.Unmarshal(msg, &cfg); err != nil {
				return fmt.Errorf("failed to unmarshal config: %w", err)
			}

			// Log the command
			s.log.WithFields(logrus.Fields{
				"concurrency": cfg.Data.Concurrent,
			}).Info("Received command to set concurrency")

			// Send the job to the workers
			worker = cfg.Data.Concurrent
			return nil
		}

		if routingKey != TaskRoutingKey {
			return nil
		}

		// Unmarshal the message
		var page models.Page
		if err := json.Unmarshal(msg, &page); err != nil {
			return fmt.Errorf("failed to unmarshal page: %w", err)
		}

		// Log the command
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
			"m3u8":  page.M3u8,
		}).Info("Received command to download video")

		// Send the job to the workers
		jobs <- MessageJob{
			Page: page,
		}

		return nil
	})
}

// worker handles download jobs
func (s *DownloaderService) worker(id int, wg *sync.WaitGroup, jobs <-chan MessageJob, sem chan struct{}) {
	defer wg.Done()
	s.log.WithField("worker_id", id).Info("Starting download worker")

	for job := range jobs {
		sem <- struct{}{} // Acquire semaphore

		s.log.WithFields(logrus.Fields{
			"worker_id": id,
			"title":     job.Page.Title,
		}).Info("Worker processing download")

		if err := s.DownloadVideo(job.Page); err != nil {
			s.log.WithError(err).Error("Error downloading video")
		}

		s.log.WithFields(logrus.Fields{
			"worker_id": id,
			"title":     job.Page.Title,
		}).Info("Worker finished download")

		<-sem // Release semaphore
	}

	s.log.WithField("worker_id", id).Info("Worker shutting down")
}
