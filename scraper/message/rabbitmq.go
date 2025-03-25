package message

import (
	"encoding/json"
	"fmt"
	"sync"
	"wleowleo-scraper/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type Message struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	M3U8  string `json:"m3u8"`
}

type Producer struct {
	cfg     *config.Config
	log     *logrus.Logger
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
	isInit  bool
}

func NewMessage(title, url, m3u8 string) Message {
	return Message{
		Title: title,
		URL:   url,
		M3U8:  m3u8,
	}
}

func NewProducer(cfg *config.Config, log *logrus.Logger) *Producer {
	return &Producer{
		cfg:    cfg,
		log:    log,
		isInit: false,
	}
}

// Initialize creates connection and channel to RabbitMQ
func (p *Producer) Initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isInit {
		return nil
	}

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

	// Declare the queue
	_, err = p.channel.QueueDeclare(
		p.cfg.RabbitMQQueue, // name
		false,               // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		p.log.WithError(err).Error("Error declaring queue")
		return err
	}

	p.isInit = true
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
	p.isInit = false
}

// Produce sends a message to the RabbitMQ queue
func (p *Producer) Produce(msg Message) error {
	if !p.isInit {
		if err := p.Initialize(); err != nil {
			return err
		}
	}

	// Marshal the message
	body, err := json.Marshal(msg)
	if err != nil {
		p.log.WithError(err).Error("Error marshaling message")
		return err
	}

	// Publish the message
	err = p.channel.Publish(
		"",                  // exchange
		p.cfg.RabbitMQQueue, // routing key
		false,               // mandatory
		false,               // immediate
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
