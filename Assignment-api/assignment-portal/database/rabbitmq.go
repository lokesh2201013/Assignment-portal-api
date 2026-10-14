package database

import (
	"fmt"

	"github.com/lokesh2201013/config"
	"github.com/streadway/amqp"
)

type RabbitPublisher struct {
	url       string
	queueName string
}

func NewRabbitPublisher(cfg config.Config) *RabbitPublisher {
	return &RabbitPublisher{url: cfg.RabbitMQURL, queueName: "task_queue"}
}

func (p *RabbitPublisher) PublishMessage(id string) error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		p.queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare RabbitMQ queue: %w", err)
	}

	if err := ch.Publish(
		"",
		q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(id),
		}); err != nil {
		return fmt.Errorf("publish RabbitMQ message: %w", err)
	}

	return nil
}

func PublishMessage(id string) error {
	return NewRabbitPublisher(config.Load()).PublishMessage(id)
}
