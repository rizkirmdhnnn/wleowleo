package message

import (
	"encoding/json"
	"fmt"
	"sync"
	"wleowleo-downloader/config"
	"wleowleo-downloader/scraper"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Consumer represents a RabbitMQ consumer
type Consumer struct {
	cfg        *config.Config
	log        *logrus.Logger
	downloader *scraper.Scraper
	conn       *amqp.Connection
	channel    *amqp.Channel
}

func NewConsumer(cfg *config.Config, downloader *scraper.Scraper, log *logrus.Logger) *Consumer {
	return &Consumer{
		cfg:        cfg,
		log:        log,
		downloader: downloader,
	}
}

// Initialize creates connection and channel to RabbitMQ
func (c *Consumer) Initialize() error {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/",
		c.cfg.RabbitMQUser,
		c.cfg.RabbitMQPassword,
		c.cfg.RabbitMQHost,
		c.cfg.RabbitMQPort))
	if err != nil {
		c.log.WithError(err).Error("Error connecting to RabbitMQ")
		return err
	}
	c.conn = conn

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		c.log.WithError(err).Error("Error opening channel")
		return err
	}
	c.channel = ch

	// Declare the queue
	_, err = ch.QueueDeclare(
		c.cfg.RabbitMQQueue, // queue name
		false,               // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		c.log.WithError(err).Error("Error declaring queue")
		return err
	}

	return nil
}

// Cleanup closes RabbitMQ connection and channel
func (c *Consumer) Cleanup() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// Listen listens for messages on the RabbitMQ queue
func (c *Consumer) Listen() error {
	if c.conn == nil || c.channel == nil {
		return fmt.Errorf("connection not initialized")
	}

	// Consume messages
	msgs, err := c.channel.Consume(
		c.cfg.RabbitMQQueue, // queue
		"",                  // consumer
		true,                // auto-ack
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	if err != nil {
		c.log.WithError(err).Error("Error consuming messages")
		return err
	}

	// Set up worker pool
	maxConcurrent := c.cfg.LimitConcurrentDownload
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	c.log.WithField("concurrent_limit", maxConcurrent).Info("Starting download worker pool")

	semaphore := make(chan struct{}, maxConcurrent)
	jobCh := make(chan scraper.PageLink)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go c.worker(i, &wg, jobCh, semaphore)
	}

	// Process messages
	go func() {
		for msg := range msgs {
			var message scraper.PageLink
			if err := json.Unmarshal(msg.Body, &message); err != nil {
				c.log.WithError(err).Error("Error unmarshalling message")
				continue
			}
			c.log.WithField("message", message).Info("Received message")
			jobCh <- message
		}
		close(jobCh)
	}()

	wg.Wait()
	return nil
}

// worker handles download jobs
func (c *Consumer) worker(id int, wg *sync.WaitGroup, jobs <-chan scraper.PageLink, sem chan struct{}) {
	defer wg.Done()
	c.log.WithField("worker_id", id).Info("Starting download worker")

	for job := range jobs {
		sem <- struct{}{} // Acquire semaphore

		c.log.WithFields(logrus.Fields{
			"worker_id": id,
			"title":     job.Title,
		}).Info("Worker processing download")

		c.downloader.DownloadVideo(job)

		<-sem // Release semaphore
	}

	c.log.WithField("worker_id", id).Info("Worker shutting down")
}
