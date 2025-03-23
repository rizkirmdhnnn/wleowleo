package message

import (
	"encoding/json"
	"fmt"
	"sync"
	"wleowleo-downloader/config"
	"wleowleo-downloader/scraper"

	"github.com/sirupsen/logrus"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer represents a RabbitMQ consumer
type Consumer struct {
	cfg        *config.Config
	log        *logrus.Logger
	downloader *scraper.Scraper
}

func NewConsumer(cfg *config.Config, downloader *scraper.Scraper, log *logrus.Logger) *Consumer {
	return &Consumer{
		cfg:        cfg,
		log:        log,
		downloader: downloader,
	}
}

// Listen listens for messages on the RabbitMQ queue
func (c *Consumer) Listen() error {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/", c.cfg.RabbitMQUser, c.cfg.RabbitMQPassword, c.cfg.RabbitMQHost, c.cfg.RabbitMQPort))
	if err != nil {
		c.log.WithError(err).Error("Error connecting to Rabbit MQ")
		return err
	}
	defer conn.Close()

	// Open a channel
	ch, err := conn.Channel()
	if err != nil {
		c.log.WithError(err).Error("Error opening channel")
		return err
	}
	defer ch.Close()

	// Declare the queue
	// This will create the queue if it does not exist
	_, err = ch.QueueDeclare(c.cfg.RabbitMQQueue, false, false, false, false, nil)
	if err != nil {
		c.log.WithError(err).Error("Error declaring queue")
		return err
	}

	// Consume messages
	msgs, err := ch.Consume(c.cfg.RabbitMQQueue, "", true, false, false, false, nil)
	if err != nil {
		c.log.WithError(err).Error("Error consuming messages")
		return err
	}

	// Create a worker pool with a limit of concurrent downloads
	maxConcurrent := c.cfg.LimitConcurrentDownload
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default to 10 if not set or invalid
	}

	c.log.WithField("concurrent_limit", maxConcurrent).Info("Starting download worker pool")

	// Create a channel to limit concurrent downloads (semaphore pattern)
	semaphore := make(chan struct{}, maxConcurrent)

	// Create a channel for download jobs
	jobCh := make(chan scraper.PageLink)

	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.log.WithField("worker_id", workerID).Info("Starting download worker")

			for job := range jobCh {
				// Acquire semaphore
				semaphore <- struct{}{}

				c.log.WithFields(logrus.Fields{
					"worker_id": workerID,
					"title":     job.Title,
				}).Info("Worker processing download")

				// Process the download
				c.downloader.DownloadVideo(job)

				// Release semaphore
				<-semaphore
			}

			c.log.WithField("worker_id", workerID).Info("Worker shutting down")
		}(i)
	}

	// Listen for messages and send them to workers
	go func() {
		for msg := range msgs {
			c.log.WithField("message", string(msg.Body)).Info("Received message")

			// Unmarshal the message
			var message scraper.PageLink
			err = json.Unmarshal(msg.Body, &message)
			if err != nil {
				c.log.WithError(err).Error("Error unmarshalling message")
				continue
			}

			// Send to worker pool
			jobCh <- message
		}

		// Close job channel when no more messages
		close(jobCh)
	}()

	// Wait for all workers to finish
	wg.Wait()
	return nil
}
