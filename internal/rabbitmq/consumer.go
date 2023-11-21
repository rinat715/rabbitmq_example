package rabbitmq

import (
	"context"
	"spread_message/internal/logger"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	queueName       string
	consumerName    string
	deliveries      <-chan amqp.Delivery
	channel         *Channel
	notifyChanClose chan *amqp.Error // другое чем chanel.notifyChanClose
	callback        func(data []byte) error
	IsReconnected   chan bool
}

func (c *Consumer) consume() error {
	if c.channel.ch == nil {
		return errNotConnected
	}

	var err error
	c.deliveries, err = c.channel.ch.Consume(
		c.queueName,
		c.consumerName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *Consumer) init() error {
	err := c.consume()
	if err != nil {
		return err
	}
	c.notifyChanClose = c.channel.ch.NotifyClose(make(chan *amqp.Error, 1))
	return nil
}

func (c *Consumer) Consume(ctx context.Context) error {
	err := c.init()
	if err != nil {
		logger.Debug("Could not start consuming: %s\n", err)
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-c.notifyChanClose:

			<-c.IsReconnected

			err := c.init()
			if err != nil {
				logger.Debug("Could not start consuming: %s\n", err)
				return err
			}

			logger.Debug("консумер реконнетед")

		case delivery := <-c.deliveries:
			// Ack a message every 2 seconds
			err := c.callback(delivery.Body)

			if err != nil {
				logger.Error(err)
				err = delivery.Reject(false)
				if err != nil {
					logger.Error("Error acknowledging message: %s\n", err)
				}
			} else {
				err := delivery.Ack(false)
				if err != nil {
					logger.Debug("Error acknowledging message: %s\n", err)
				}
			}

			<-time.After(time.Second * 2)
		}
	}
}

func NewConsumer(channel *Channel, queueName string, consumerName string, callback func(data []byte) error) *Consumer {
	return &Consumer{
		queueName:     queueName,
		consumerName:  consumerName,
		channel:       channel,
		callback:      callback,
		IsReconnected: channel.NotifyReconnect(make(chan bool, 1)),
	}
}
