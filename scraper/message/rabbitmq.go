package message

import (
	"encoding/json"
	"fmt"
	"wleowleo-scraper/config"
	"wleowleo-scraper/model"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Producer represents a RabbitMQ producer
type Producer struct {
	cfg     *config.Config
	log     *logrus.Logger
	conn    *amqp.Connection
	channel *amqp.Channel
	queues  map[string]string
}

// NewProducer creates a new RabbitMQ producer
func NewProducer(cfg *config.Config, log *logrus.Logger) *Producer {
	return &Producer{
		cfg: cfg,
		log: log,
		queues: map[string]string{
			"video": "wleo_video_queue",
			"log":   "wleo_scrap_log",
		},
	}
}

// Initialize creates connection and channel to RabbitMQ
func (p *Producer) Initialize() error {
	var err error
	// Connect to RabbitMQ
	p.conn, err = amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/",
		p.cfg.RabbitMQUser,
		p.cfg.RabbitMQPassword,
		p.cfg.RabbitMQHost,
		p.cfg.RabbitMQPort))
	if err != nil {
		p.log.WithError(err).Error("Error connecting to RabbitMQ")
		return err
	}

	// Open a channel
	p.channel, err = p.conn.Channel()
	if err != nil {
		p.log.WithError(err).Error("Error opening channel")
		return err
	}

	// Declare queues
	if err := p.declareQueues(); err != nil {
		return err
	}

	return nil
}

// declareQueues declares all required queues
func (p *Producer) declareQueues() error {
	// Declare the queue for video messages
	if err := p.declareQueue("wleo_video_queue"); err != nil {
		return err
	}

	// Declare the queue for scrap logs
	if err := p.declareQueue("wleo_scrap_log"); err != nil {
		return err
	}

	return nil
}

// declareQueue declares a single queue
func (p *Producer) declareQueue(name string) error {
	_, err := p.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		p.log.WithError(err).Errorf("Error declaring queue: %s", name)
		return err
	}
	return nil
}

// Close closes the RabbitMQ connection and channel
func (p *Producer) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// publish is a generic method to publish messages to RabbitMQ
func (p *Producer) publish(routingKey string, body []byte) error {
	err := p.channel.Publish(
		"",         // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		})
	if err != nil {
		p.log.WithError(err).Error("Error publishing message")
		return err
	}
	return nil
}

// ProduceMsg sends a video link message to the RabbitMQ queue
func (p *Producer) ProduceMsg(msg model.PageLink) error {
	// Marshal the message
	body, err := json.Marshal(msg)
	if err != nil {
		p.log.WithError(err).Error("Error marshaling message")
		return err
	}

	return p.publish(p.queues["video"], body)
}

// ProduceLog sends a log message to the RabbitMQ queue
func (p *Producer) ProduceLog(msg model.ScrapLog) error {
	// Marshal the message
	body, err := json.Marshal(msg)
	if err != nil {
		p.log.WithError(err).Error("Error marshaling message")
		return err
	}

	return p.publish(p.queues["log"], body)
}
