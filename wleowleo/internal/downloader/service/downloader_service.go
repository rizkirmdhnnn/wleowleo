package service

import (
	"context"
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
	"github.com/rizkirmdhn/wleowleo/pkg/utils"
	"github.com/sirupsen/logrus"
)

// Global variables to store worker channels
var (
	cancelFunc         context.CancelFunc
	jobsChannel        chan models.Page
	workersInitialized bool
	ctx                = context.Background()
	// workerMutex        sync.Mutex // Mutex to protect access to global variables
)

// MessageJob represents a job to be processed by a worker
type MessageJob struct {
	Page models.Page
}

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

// Start starts the Downloader service
func (s *DownloaderService) Start() error {
	// Setup message consumer
	if err := s.setupMessaging(); err != nil {
		return fmt.Errorf("failed to setup messaging: %w", err)
	}

	// Consume command messages
	if err := s.message.Consume(s.rabbitCfg.Queue.Downloader, func(msg []byte, routingKey string) error {
		s.log.WithFields(logrus.Fields{
			"routing_key": routingKey,
			"message":     string(msg),
		}).Debug("Received command message")
		return s.handleCommand(msg, routingKey)
	}); err != nil {
		s.log.WithError(err).Error("Failed to consume command messages")
	}

	s.log.Info("Downloader service started successfully")
	return nil
}

// handleTask handles incoming task messages
func (s *DownloaderService) setupTask() error {
	if err := s.message.Consume(s.rabbitCfg.Queue.DownloadTask, func(msg []byte, routingKey string) error {
		s.log.WithFields(logrus.Fields{
			"routing_key": routingKey,
			"message":     string(msg),
		}).Debug("Received task message")
		return s.prosesTask(msg, routingKey)
	}); err != nil {
		s.log.WithError(err).Error("Failed to consume task messages")
	}

	s.log.Info("Task message started successfully")
	return nil
}

// handleTask handles incoming task messages
func (s *DownloaderService) prosesTask(msg []byte, routingKey string) error {
	var page models.Page
	if err := json.Unmarshal(msg, &page); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// Publish start download message
	// s.publishDownloaderLog("start", page, nil, nil)

	// Lock mutex to safely access jobsChannel
	// workerMutex.Lock()
	// defer workerMutex.Unlock()

	// Send the job to the workers if channel exists and workers are initialized
	if jobsChannel != nil && workersInitialized {
		// Use non-blocking send with select to prevent panic on closed channel
		select {
		case jobsChannel <- page:
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
			}).Debug("Task sent to workers successfully")
		default:
			// Channel is either full or closed
			s.log.WithFields(logrus.Fields{
				"title": page.Title,
				"url":   page.URL,
			}).Warn("Could not send task to workers, channel might be closed or full")
		}
	} else {
		s.log.WithFields(logrus.Fields{
			"title": page.Title,
		}).Warn("Workers not initialized or channel is nil, cannot process task")
	}

	return nil
}

// handleCommand handles incoming command messages
func (s *DownloaderService) handleCommand(msg []byte, routingKey string) error {
	// Unmarshal the message
	var cmd models.ScrapingCommand
	if err := json.Unmarshal(msg, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Start the downloader
	if cmd.Action == models.StartScrapingAction {
		s.log.WithFields(logrus.Fields{
			"concurrent": cmd.Data.Concurrent,
		}).Info("Received config to start downloader")

		// Set qos for prefetch count
		if err := s.message.SetQos(cmd.Data.Concurrent); err != nil {
			s.log.WithError(err).Error("Failed to set QoS")
		}

		if err := s.setupWorker(cmd.Data.Concurrent); err != nil {
			return fmt.Errorf("failed to setup workers: %w", err)
		}

		if err := s.setupTask(); err != nil {
			return fmt.Errorf("failed to setup task: %w", err)
		}
		return nil
	}

	// Stop the downloader
	if cmd.Action == models.StopScrapingAction {
		s.log.Info("Received config to stop downloader")
		return s.Stop()
	}

	return nil
}

func (s *DownloaderService) Stop() error {
	// Remove all workers and cleanup temporary files
	if err := s.stopWorker(); err != nil {
		return fmt.Errorf("failed to stop workers: %w", err)
	}

	// cleanup temp directory
	if err := utils.ClearFolder("temp"); err != nil {
		return fmt.Errorf("failed to cleanup temp directory: %w", err)
	}

	s.log.Info("Downloader service stopped successfully")
	return nil
}

//

// setupMessaging sets up the messaging infrastructure
func (s *DownloaderService) setupMessaging() error {
	queues := []struct {
		name        string
		exchange    string
		routingKeys []string
	}{
		{
			// Declare the task queue
			name:        s.rabbitCfg.Queue.DownloadTask,
			exchange:    s.rabbitCfg.Exchange,
			routingKeys: []string{config.RoutingTaskDownload},
		},
		{
			// Declare the command downloader service queue
			name:        s.rabbitCfg.Queue.Downloader,
			exchange:    s.rabbitCfg.Exchange,
			routingKeys: []string{config.RoutingCommandDownloader},
		},
		{
			// Declare the command scraper service queue
			name:        s.rabbitCfg.Queue.Scraper,
			exchange:    s.rabbitCfg.Exchange,
			routingKeys: []string{config.RoutingCommandScraper},
		},
		{
			// Declare the log queue
			name:        s.rabbitCfg.Queue.Log,
			exchange:    s.rabbitCfg.Exchange,
			routingKeys: []string{config.RoutingLogDownloader, config.RoutingLogScraper},
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
func (s *DownloaderService) DownloadVideo(ctx context.Context, urls models.Page) error {
	// Check context before starting download
	if ctx.Err() != nil {
		s.log.WithFields(logrus.Fields{"title": urls.Title}).Info("Download cancelled before starting")
		s.publishDownloaderLog("cancelled", urls, fmt.Errorf("download cancelled before starting"), nil)
		return ctx.Err()
	}

	s.log.WithFields(logrus.Fields{"title": urls.Title, "m3u8": urls.M3u8}).Debug("Downloading video")

	// Publish start download message
	s.publishDownloaderLog("start", urls, nil, nil)

	// Create a channel to monitor for cancellation during processing
	processDone := make(chan error, 1)
	go func() {
		processDone <- s.processM3U8File(ctx, urls.M3u8, urls.Title)
	}()

	// Wait for either completion or cancellation
	select {
	case err := <-processDone:
		if err != nil {
			// Check if the error is due to cancellation
			if ctx.Err() != nil {
				s.log.WithFields(logrus.Fields{"title": urls.Title}).Info("Download cancelled")
				s.publishDownloaderLog("cancelled", urls, fmt.Errorf("download cancelled"), nil)
				return ctx.Err()
			}

			// Publish error log for other errors
			s.publishDownloaderLog("error", urls, err, nil)
			return err
		}

		s.log.WithFields(logrus.Fields{"title": urls.Title, "m3u8": urls.M3u8}).Debug("Download completed")

		// Publish success log
		s.publishDownloaderLog("success", urls, nil, nil)
		return nil

	case <-ctx.Done():
		// Context was cancelled while waiting for processing
		s.log.WithFields(logrus.Fields{"title": urls.Title}).Info("Download cancelled while processing")
		s.publishDownloaderLog("cancelled", urls, fmt.Errorf("download cancelled while processing"), nil)
		return ctx.Err()
	}
}

// Process M3U8 file
func (s *DownloaderService) processM3U8File(ctx context.Context, m3u8URL string, title string) error {
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

	// Progress tracking variables
	totalSegments := len(matches)
	downloadedSegments := 0
	progressMutex := sync.Mutex{}

	// Create a progress info object to track download progress
	progressInfo := &models.ProgressInfo{
		TotalSegments: totalSegments,
		Downloaded:    0,
	}

	// Report initial progress
	s.publishDownloaderLog("progress", models.Page{Title: title, URL: "", M3u8: m3u8URL}, nil, progressInfo)

	// Function to update progress and report it
	updateProgress := func(increment int) {
		// Check if context is cancelled before updating progress
		if ctx.Err() != nil {
			return
		}

		progressMutex.Lock()
		defer progressMutex.Unlock()

		downloadedSegments += increment
		progressInfo.Downloaded = downloadedSegments

		// Report progress periodically (e.g., every 5% or when complete)
		if downloadedSegments == totalSegments || downloadedSegments%max(1, totalSegments/20) == 0 {
			// Check context again before publishing log
			if ctx.Err() != nil {
				return
			}
			s.publishDownloaderLog("progress", models.Page{Title: title, URL: "", M3u8: m3u8URL}, nil, progressInfo)
		}
	}

	// Use the configured number of workers or default to a reasonable number
	numWorkers := s.config.DefaultWorker
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
			for {
				select {
				case <-ctx.Done():
					// Context was cancelled, exit the worker
					s.log.WithFields(logrus.Fields{
						"worker_id": workerID,
						"title":     title,
					}).Debug("Segment download worker cancelled")
					return

				case job, ok := <-jobs:
					// Check if channel is closed
					if !ok {
						return
					}

					// Skip if an error has already occurred
					mu.Lock()
					if downloadErr != nil {
						mu.Unlock()
						continue
					}
					mu.Unlock()

					// Check context before starting download
					if ctx.Err() != nil {
						mu.Lock()
						if downloadErr == nil {
							downloadErr = ctx.Err()
						}
						mu.Unlock()
						return
					}

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

					// Update progress
					updateProgress(1)
				}
			}
		}(w)
	}

	// Send jobs to the workers
	jobSendDone := make(chan struct{})
	go func() {
		defer close(jobSendDone)
		defer close(jobs)

		// Check context before starting job distribution
		if ctx.Err() != nil {
			s.log.WithFields(logrus.Fields{
				"title": title,
			}).Info("Not starting job distribution due to cancellation")
			return
		}

		for i, match := range matches {
			// Check if context was cancelled - check more frequently
			if ctx.Err() != nil {
				s.log.WithFields(logrus.Fields{
					"title": title,
				}).Info("Cancelling job distribution due to stop request")
				return
			}

			tsFileName := filepath.Join(basefolder, fmt.Sprintf("segment-%d.ts", i))

			// Try to send the job with a shorter timeout to ensure we can cancel quickly
			select {
			case jobs <- downloadJob{
				index:    i,
				url:      match,
				fileName: tsFileName,
			}:
				// Job sent successfully
			case <-ctx.Done():
				s.log.WithFields(logrus.Fields{
					"title": title,
				}).Info("Cancelling job distribution due to stop request")
				return
			case <-time.After(20 * time.Millisecond): // Reduced timeout for faster cancellation
				// If we can't send the job quickly, check if context is cancelled
				if ctx.Err() != nil {
					s.log.WithFields(logrus.Fields{
						"title": title,
					}).Info("Cancelling job distribution due to stop request")
					return
				}
				// Try again in the next iteration
				i--
				continue
			}

			// Check context every few segments to ensure responsive cancellation
			if i > 0 && i%10 == 0 {
				if ctx.Err() != nil {
					s.log.WithFields(logrus.Fields{
						"title": title,
						"sent":  i,
						"total": len(matches),
					}).Info("Cancelling job distribution due to stop request")
					return
				}
			}
		}
	}()

	// Wait for all downloads to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All downloads completed normally
	case <-ctx.Done():
		// Context was cancelled
		s.log.WithFields(logrus.Fields{
			"title": title,
		}).Info("Download process cancelled")

		// Wait for job sending to finish
		<-jobSendDone

		// Wait with timeout for workers to finish
		select {
		case <-done:
			s.log.Debug("All segment workers completed after cancellation")
		case <-time.After(2 * time.Second):
			s.log.Debug("Timeout waiting for segment workers to complete after cancellation")
		}

		return ctx.Err()
	}

	// Check if any errors occurred during download
	if downloadErr != nil {
		return downloadErr
	}

	s.log.Info("All segments downloaded successfully, converting to MP4")

	// Check context before sending final progress update
	if ctx.Err() == nil {
		// Final progress update (100%)
		progressInfo.Downloaded = totalSegments
		s.publishDownloaderLog("complete", models.Page{Title: title, URL: "", M3u8: m3u8URL}, nil, progressInfo)
	} else {
		s.log.WithFields(logrus.Fields{
			"title": title,
		}).Debug("Skipping final progress update due to cancellation")
	}

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
	outputFile := filepath.Join(s.config.DownloadDir, fmt.Sprintf("%s.mp4", foldername))
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

// stopWorker stops all download workers and cleans up temporary files
func (s *DownloaderService) stopWorker() error {
	s.log.Info("Stopping all download workers")

	// Lock the global worker mutex to prevent race conditions
	// workerMutex.Lock()
	// defer workerMutex.Unlock()

	if !workersInitialized {
		s.log.Info("No active workers to stop")
		return nil
	}

	// First set workersInitialized to false to prevent new tasks from being processed
	workersInitialized = false

	// Store local reference to channel before setting global to nil
	jobsChan := jobsChannel
	jobsChannel = nil

	// Cancel context to signal workers to stop
	if cancelFunc != nil {
		s.log.Debug("Cancelling worker context")
		cancelFunc()
		cancelFunc = nil
	}

	// Close channel after cancelling context
	if jobsChan != nil {
		s.log.Debug("Closing jobs channel")
		// We already have the mutex locked, so it's safe to close the channel
		close(jobsChan)
	}

	// Increase wait time to ensure all workers have time to detect cancellation
	// and properly shut down their operations
	s.log.Debug("Waiting for workers to shut down")
	time.Sleep(1000 * time.Millisecond) // Increased wait time to ensure workers have time to clean up

	// Purge download task queue to prevent processing new tasks after restart
	if err := s.message.PurgeQueue(s.rabbitCfg.Queue.DownloadTask); err != nil {
		s.log.WithError(err).Warn("Failed to purge download task queue")
	} else {
		s.log.Debug("Successfully purged download task queue")
	}

	return nil
}
func (s *DownloaderService) setupWorker(worker int) error {
	// Lock mutex to prevent race conditions
	// workerMutex.Lock()
	// defer workerMutex.Unlock()

	// If workers are already initialized, stop them first
	if workersInitialized {
		s.log.Info("Workers already initialized, stopping them first")
		s.stopWorker()

		// Add a small delay to ensure previous workers are fully stopped
		time.Sleep(500 * time.Millisecond)
	}

	// Create new context with cancellation and timeout
	// Using a background context to ensure it's not tied to any previous context
	var newCtx context.Context
	newCtx, cancelFunc = context.WithCancel(context.Background())

	// Create a buffered channel to prevent blocking
	jobsChannel = make(chan models.Page, worker*2) // Double buffer size for better throughput

	// Start worker goroutines
	for w := 1; w <= worker; w++ {
		go s.worker(w, newCtx, jobsChannel)
	}

	workersInitialized = true
	s.log.WithField("workers", worker).Info("Workers initialized successfully")

	return nil
}

// worker handles download jobs
func (s *DownloaderService) worker(id int, ctx context.Context, jobs <-chan models.Page) {
	s.log.WithField("worker_id", id).Info("Starting download worker")

	// Create a WaitGroup to track active downloads
	var activeDownloads sync.WaitGroup

	// Cleanup function to ensure all resources are released
	cleanup := func() {
		// Wait for active downloads with timeout
		done := make(chan struct{})
		go func() {
			activeDownloads.Wait()
			close(done)
		}()

		select {
		case <-done:
			s.log.WithField("worker_id", id).Debug("All downloads cleaned up successfully")
		case <-time.After(2 * time.Second):
			s.log.WithField("worker_id", id).Warn("Timeout waiting for downloads to clean up")
		}
	}

	// Ensure cleanup happens when worker exits
	defer cleanup()

	for {
		select {
		case <-ctx.Done():
			s.log.WithField("worker_id", id).Info("Worker received stop signal, shutting down")
			return

		case job, ok := <-jobs:
			if !ok {
				s.log.WithField("worker_id", id).Info("Jobs channel closed, worker shutting down")
				return
			}

			// Check if context is already cancelled before starting new download
			if ctx.Err() != nil {
				s.log.WithFields(logrus.Fields{
					"worker_id": id,
					"title":     job.Title,
				}).Info("Skipping download due to stop request")
				return
			}

			s.log.WithFields(logrus.Fields{
				"worker_id": id,
				"title":     job.Title,
			}).Info("Worker processing download")

			// Track this download
			activeDownloads.Add(1)

			// Create a context with timeout for this job
			jobCtx, jobCancel := context.WithCancel(ctx)
			downloadDone := make(chan error, 1)

			go func() {
				defer activeDownloads.Done()
				downloadDone <- s.DownloadVideo(jobCtx, job)
			}()

			select {
			case err := <-downloadDone:
				jobCancel() // Pastikan context dibatalkan setelah selesai
				if err != nil {
					if err == context.Canceled {
						s.log.WithFields(logrus.Fields{
							"worker_id": id,
							"title":     job.Title,
						}).Info("Download was cancelled")
					} else {
						s.log.WithError(err).Error("Error downloading video")
					}
				} else {
					s.log.WithFields(logrus.Fields{
						"worker_id": id,
						"title":     job.Title,
					}).Info("Worker finished download successfully")
				}

			case <-ctx.Done():
				// Batalkan job context untuk menghentikan download yang sedang berjalan
				jobCancel()
				s.log.WithFields(logrus.Fields{
					"worker_id": id,
					"title":     job.Title,
				}).Info("Cancelling in-progress download due to stop request")

				// Tunggu dengan timeout yang lebih lama untuk memastikan download benar-benar berhenti
				select {
				case <-downloadDone:
					s.log.WithField("worker_id", id).Debug("Download successfully cancelled")
				case <-time.After(1 * time.Second):
					s.log.WithField("worker_id", id).Warn("Timeout waiting for download to cancel, forcing exit")
				}
				return
			}
		}
	}
}

// publishDownloaderLog publishes a downloader log message
func (s *DownloaderService) publishDownloaderLog(status string, page models.Page, err error, progress *models.ProgressInfo) {
	// Skip progress updates if status is "progress" and workers are not initialized
	// This prevents sending progress updates after the service has been stopped
	if status == "progress" && !workersInitialized {
		s.log.WithFields(logrus.Fields{
			"title":  page.Title,
			"status": status,
		}).Debug("Skipping progress update because workers are not initialized")
		return
	}

	// Create the VideoLog message
	log := models.VideoLog{
		Status: status,
		Data: models.Video{
			Title:    page.Title,
			URL:      page.URL,
			M3U8:     &page.M3u8,
			Progress: progress,
		},
	}

	if err != nil {
		log.Error = err.Error()
	}

	// Publish to the log exchange with downloader.log routing key
	publishErr := s.message.PublishJSON(
		s.rabbitCfg.Exchange,
		config.RoutingLogDownloader,
		log,
	)

	if publishErr != nil {
		s.log.WithError(publishErr).Error("Failed to publish downloader log")
	} else if progress != nil {
		// Log the progress if available
		s.log.WithFields(logrus.Fields{
			"title":    page.Title,
			"status":   status,
			"progress": progress.Downloaded,
			"total":    progress.TotalSegments,
			"percent":  fmt.Sprintf("%.1f%%", float64(progress.Downloaded)/float64(progress.TotalSegments)*100),
		}).Debug("Published progress update")
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
