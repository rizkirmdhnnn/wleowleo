package scraper

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// downloadJob represents a single download task
type downloadJob struct {
	index    int
	url      string
	fileName string
}

// Download video from m3u8 link
func (s *Scraper) DownloadVideo(urlM3U8 string) {
	log.Println("Downloading video from", urlM3U8)
	log.Println("Using parallel downloading with goroutines")
	err := processM3U8File(urlM3U8)
	if err != nil {
		fmt.Println(err)
	}
	log.Println("Download complete")
}

// Convert .ts files to .mp4
func convertTStoMP4(foldername string, tsFiles []string) error {
	// Create list file to concatenate .ts files
	listFileName := filepath.Join("temp", foldername, fmt.Sprintf("%s.txt", foldername))
	listFile, err := os.Create(listFileName)
	if err != nil {
		return fmt.Errorf("error creating filelist.txt: %v", err)
	}
	defer listFile.Close()

	// Write .ts file paths to list file
	for _, tsFile := range tsFiles {
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
	}

	// Output file name
	outputFile := filepath.Join("output", fmt.Sprintf("%s.mp4", foldername))

	// Run ffmpeg command to concatenate .ts files
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFileName, "-c:v", "copy", "-c:a", "copy", "-y", outputFile)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v", err)
	}
	return nil
}

// Download file from URL
func downloadFile(url string, fileName string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading file: %v", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

// Process M3U8 file
func processM3U8File(m3u8URL string) error {
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

	// Create base folder
	if err := os.MkdirAll(filepath.Join("temp", link[1]), 0755); err != nil {
		return fmt.Errorf("error creating temp directory: %v", err)
	}
	basefolder := filepath.Join("temp", link[1])

	// Download M3U8 file
	fileName := filepath.Join(basefolder, fmt.Sprintf("%s.m3u8", link[1]))
	if err := downloadFile(m3u8URL, fileName); err != nil {
		return err
	}

	// Read M3U8 file
	data, err := os.ReadFile(fileName)
	if err != nil {
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
	numWorkers := 5
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
				err := downloadFile(job.url, job.fileName)
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
		return downloadErr
	}

	// Convert .ts files to MP4
	err = convertTStoMP4(link[1], tsFiles)
	if err != nil {
		return err
	}
	os.RemoveAll(basefolder)

	fmt.Println("Proses selesai, file MP4 disimpan dengan nama: output.mp4")
	return nil
}
