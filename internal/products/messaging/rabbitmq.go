package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"product-notifications/internal/products"

	amqp "github.com/rabbitmq/amqp091-go"
)

const contentTypeJSON = "application/json"

type RabbitPublisher struct {
	channel *amqp.Channel
	queue   string
}

func NewRabbitPublisher(conn *amqp.Connection, queue string) (*RabbitPublisher, error) {
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

	return &RabbitPublisher{
		channel: ch,
		queue:   queue,
	}, nil
}

func (p *RabbitPublisher) Publish(ctx context.Context, event products.ProductEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.channel.PublishWithContext(
		ctx,
		"",
		p.queue,
		false,
		false,
		amqp.Publishing{
			ContentType: contentTypeJSON,
			Body:        payload,
		},
	); err != nil {
		return fmt.Errorf("publish to %q: %w", p.queue, err)
	}

	return nil
}

func (p *RabbitPublisher) Close() error {
	return p.channel.Close()
}
