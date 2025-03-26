package message

import (
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rizkirmdhn/wleowleo/web/config"
	"github.com/rizkirmdhn/wleowleo/web/model"
	"github.com/rizkirmdhn/wleowleo/web/websocket"
)

// Consumer represents a RabbitMQ consumer
type Consumer struct {
	cfg     *config.Config
	hub     *websocket.Hub
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(cfg *config.Config, hub *websocket.Hub) *Consumer {
	return &Consumer{
		cfg: cfg,
		hub: hub,
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
		log.Printf("Error connecting to RabbitMQ: %v\n", err)
		return err
	}
	c.conn = conn

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Error opening channel: %v\n", err)
		return err
	}
	c.channel = ch

	// Declare the queue
	_, err = ch.QueueDeclare(
		"scrap_log", // queue name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		log.Printf("Error declaring queue: %v\n", err)
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
		log.Println("Connection not initialized")
		return fmt.Errorf("connection not initialized")
	}

	// Consume messages
	msgs, err := c.channel.Consume(
		"wleo_scrap_log", // queue
		"",               // consumer
		true,             // auto-ack
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		log.Printf("Error consuming messages: %v\n", err)
		return err
	}

	log.Println("Started consuming messages from RabbitMQ")

	// Process messages
	go func() {
		for msg := range msgs {
			c.processMessage(msg.Body)
		}
	}()

	return nil
}

// processMessage processes a message from RabbitMQ
func (c *Consumer) processMessage(body []byte) {
	// Try to unmarshal as PageLink
	var data model.ScrapLog
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("Error unmarshalling PageLink: %v\n", err)
		return
	}

	messageJSON, _ := json.Marshal(data)
	c.hub.Broadcast(messageJSON)
}
