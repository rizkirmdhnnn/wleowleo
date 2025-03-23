package message

import (
	"encoding/json"
	"fmt"
	"wleowleo/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Message represents a message
type Message struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	M3U8  string `json:"m3u8"`
}

// Producer represents a message producer
type Producer struct {
	cfg *config.Config
	log *logrus.Logger
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
		cfg: cfg,
		log: log,
	}
}

// Produce sends a message to the RabbitMQ queue
func (p *Producer) Produce(msg Message) error {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/", p.cfg.RabbitMQUser, p.cfg.RabbitMQPassword, p.cfg.RabbitMQHost, p.cfg.RabbitMQPort))
	if err != nil {
		p.log.WithError(err).Error("Error connecting to Rabbit MQ")
	}

	// Open a channel
	ch, err := conn.Channel()
	if err != nil {
		p.log.WithError(err).Error("Error opening channel")
	}

	defer ch.Close()

	// Declare the queue
	// This will create the queue if it does not exist
	_, err = ch.QueueDeclare(p.cfg.RabbitMQQueue, false, false, false, false, nil)
	if err != nil {
		p.log.WithError(err).Error("Error declaring queue")
	}

	// Marshal the message
	body, _ := json.Marshal(msg)

	// Publish the message
	err = ch.Publish("", p.cfg.RabbitMQQueue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(body),
	})
	if err != nil {
		p.log.WithError(err).Error("Error publishing message")
	}

	return nil
}
