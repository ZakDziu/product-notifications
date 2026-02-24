package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"product-notifications/internal/products"

	amqp "github.com/rabbitmq/amqp091-go"
)

const consumerTag = "notifications-service"

type Consumer struct {
	channel *amqp.Channel
	queue   string
	logger  *slog.Logger
}

func NewConsumer(conn *amqp.Connection, queue string, logger *slog.Logger) (*Consumer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}

	_, err = ch.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("declare queue %q: %w", queue, err)
	}

	return &Consumer{
		channel: ch,
		queue:   queue,
		logger:  logger,
	}, nil
}

func (c *Consumer) Listen(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.queue,
		consumerTag,
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue %q: %w", c.queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}

			if err := c.handleMessage(&msg); err != nil {
				c.logger.Error("handle message failed", "error", err)
				_ = msg.Nack(false, true)
				continue
			}

			_ = msg.Ack(false)
		}
	}
}

func (c *Consumer) handleMessage(msg *amqp.Delivery) error {
	var event products.ProductEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	c.logger.Info("notification event",
		"event_type", event.EventType,
		"product_id", event.ProductID,
		"name", event.Name,
		"timestamp", event.Timestamp,
	)

	return nil
}

func (c *Consumer) Close() error {
	return c.channel.Close()
}
