package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rizkirmdhn/wleowleo/internal/common/config"
)

// Client defines the messaging client interface
type Client interface {
	// PublishMessage publishes a message to the exchange with the given routing key
	PublishMessage(exchange, routingKey string, body []byte) error

	// PublishJSON publishes a JSON message to the exchange with the given routing key
	PublishJSON(exchange, routingKey string, data interface{}) error

	// DeclareQueue declares a queue with the given name
	DeclareQueue(name string) error

	// BindQueue binds a queue to an exchange with the given routing key
	BindQueue(queueName, exchange, routingKey string) error

	// Consume consumes messages from the given queue
	Consume(queueName string, handler func([]byte, string) error) error

	// ConsumeWithContext consumes messages from the given queue with context support
	ConsumeWithContext(ctx context.Context, queueName string, handler func([]byte) error) error

	// Close closes the connection
	Close() error

	// GetConfig returns the RabbitMQ configuration
	GetConfig() *config.RabbitMQConfig

	// SetQos sets the prefetch count for the channel
	SetQos(prefetch int) error

	// PurgeQueue purges all messages from the queue
	PurgeQueue(queueName string) error

	// Nack all unacknowledged messages
	AckAll()

	// CloseConsumer closes the consumer channel
	CloseConsumer() error
}

// RabbitMQClient implements the Client interface using RabbitMQ
type RabbitMQClient struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	config      *config.RabbitMQConfig
	consumerTag string
}

// NewRabbitMQClient creates a new RabbitMQ client
func NewRabbitMQClient(config *config.RabbitMQConfig) (*RabbitMQClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("rabbitmq URL is required")
	}

	if config.Exchange == "" {
		return nil, fmt.Errorf("rabbitmq exchange is required")
	}

	client := &RabbitMQClient{
		config: config,
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// SetQos sets the prefetch count for the channel
func (c *RabbitMQClient) SetQos(prefetch int) error {
	return c.channel.Qos(prefetch, 0, false)
}

// PurgeQueue purges all messages from the queue
func (c *RabbitMQClient) PurgeQueue(queueName string) error {
	_, err := c.channel.QueuePurge(queueName, false)
	return err
}

// AckAll acknowledges all unacknowledged messages
func (c *RabbitMQClient) AckAll() {
	c.channel.Ack(0, true)
}

// CloseConsumer closes the consumer channel
func (c *RabbitMQClient) CloseConsumer() error {
	if c.channel != nil {
		return c.channel.Cancel("", false)
	}
	return nil
}

// connect establishes a connection to RabbitMQ
func (c *RabbitMQClient) connect() error {
	var err error

	// Connect to RabbitMQ server
	c.conn, err = amqp.Dial(c.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	// Declare log exchange
	err = c.channel.ExchangeDeclare(
		c.config.Exchange,        // name
		config.ExchangeTypeTopic, // type
		false,                    // durable
		false,                    // auto-deleted
		false,                    // internal
		false,                    // no-wait
		nil,                      // arguments
	)
	if err != nil {
		c.Close()
		return fmt.Errorf("failed to declare an exchange: %w", err)
	}

	// Declare task exchange
	err = c.channel.ExchangeDeclare(
		c.config.Exchange,        // name
		config.ExchangeTypeTopic, // type
		false,                    // durable
		false,                    // auto-deleted
		false,                    // internal
		false,                    // no-wait
		nil,                      // arguments
	)
	if err != nil {
		c.Close()
		return fmt.Errorf("failed to declare an exchange: %w", err)
	}

	// Set up connection recovery
	go c.handleReconnect()

	return nil
}

// handleReconnect attempts to reconnect to RabbitMQ when the connection is lost
func (c *RabbitMQClient) handleReconnect() {
	// Create a notifier for connection errors
	connErrChan := c.conn.NotifyClose(make(chan *amqp.Error))

	// Wait for connection errors
	for err := range connErrChan {
		fmt.Printf("RabbitMQ connection closed: %v. Attempting to reconnect...\n", err)

		for i := range c.config.ReconnectRetries {
			time.Sleep(time.Duration(c.config.ReconnectTimeout) * time.Microsecond)

			if err := c.connect(); err == nil {
				fmt.Println("Successfully reconnected to RabbitMQ")
				break
			}

			fmt.Printf("Failed to reconnect to RabbitMQ (attempt %d/%d)\n", i+1, c.config.ReconnectRetries)
		}

		fmt.Println("Failed to reconnect to RabbitMQ after multiple attempts")
		return
	}
}

// PublishMessage publishes a message to the exchange with the given routing key
func (c *RabbitMQClient) PublishMessage(exchange, routingKey string, body []byte) error {
	if exchange == "" {
		return fmt.Errorf("exchange name is required")
	}

	return c.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/octet-stream",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)
}

// PublishJSON publishes a JSON message to the exchange with the given routing key
func (c *RabbitMQClient) PublishJSON(exchange, routingKey string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON message: %w", err)
	}

	if exchange == "" {
		return fmt.Errorf("exchange name is required")
	}

	return c.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)
}

// DeclareQueue declares a queue with the given name
func (c *RabbitMQClient) DeclareQueue(name string) error {
	_, err := c.channel.QueueDeclare(
		name,  // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{"x-max-priority": 10},
	)

	return err
}

// BindQueue binds a queue to an exchange with the given routing key
func (c *RabbitMQClient) BindQueue(queueName, exchange, routingKey string) error {
	if exchange == "" {
		return fmt.Errorf("exchange name is required")
	}

	return c.channel.QueueBind(
		queueName,  // queue name
		routingKey, // routing key
		exchange,   // exchange
		false,      // no-wait
		nil,        // arguments
	)
}

// Consume consumes messages from the given queue
func (c *RabbitMQClient) Consume(queueName string, handler func(msgs []byte, routingKey string) error) error {

	// Set up consumer
	msgs, err := c.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	// Process messages
	go func() {
		for msg := range msgs {
			err := handler(msg.Body, msg.RoutingKey)
			if err != nil {
				fmt.Printf("Error processing message: %v\n", err)
				// Negative acknowledgement, message will be requeued
				msg.Nack(false, true)
			} else {
				// Acknowledge successful processing
				msg.Ack(false)
			}
		}
	}()

	return nil
}

// ConsumeWithContext consumes messages from the given queue with context support
func (c *RabbitMQClient) ConsumeWithContext(ctx context.Context, queueName string, handler func([]byte) error) error {
	// Ensure queue exists
	if err := c.DeclareQueue(queueName); err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Set up consumer
	msgs, err := c.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Consumer stopped due to context cancellation")
				return
			case msg, ok := <-msgs:
				if !ok {
					fmt.Println("Consumer channel closed")
					return
				}

				err := handler(msg.Body)
				if err != nil {
					fmt.Printf("Error processing message: %v\n", err)
					// Negative acknowledgement, message will be requeued
					msg.Nack(false, true)
				} else {
					// Acknowledge successful processing
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

// Close closes the connection and channel
func (c *RabbitMQClient) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// GetConfig returns the RabbitMQ configuration
func (c *RabbitMQClient) GetConfig() *config.RabbitMQConfig {
	return c.config
}
