package rabbitmq

import (
	"context"
	"errors"
	"spread_message/internal/logger"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	reconnectDelay = 5 * time.Second
	reInitDelay    = 2 * time.Second
	resendDelay    = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
)

type Channeler interface {
	Confirm(noWait bool) error
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	ExchangeBind(destination, key, source string, noWait bool, args amqp.Table) error
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	NotifyPublish(confirm chan amqp.Confirmation) chan amqp.Confirmation
	GetNextPublishSeqNo() uint64
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

type Channel struct {
	Channeler
	IsDisconnected     chan *amqp.Error
	notifiedReconnect  []chan bool
	notifiedDisconnect []chan *amqp.Error
	IsReady            bool
}

func (c *Channel) connect(conn Connector) (Channeler, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		return nil, err
	}
	return ch, err
}

func (c *Channel) Connect(ctx context.Context, conn Connector) error {
	for {
		ch, err := c.connect(conn)

		if err != nil {
			select {
			case <-time.After(reInitDelay):
				continue
			case <-ctx.Done():
				return err
			}
		}

		c.Channeler = ch
		c.IsDisconnected = c.NotifyClose(make(chan *amqp.Error, 1))
		c.IsReady = true
		return nil
	}

}

func (c *Channel) handleReConnect(ctx context.Context, conn Connector) error {
	for {
		select {
		case channel_error := <-c.IsDisconnected:
			if !c.IsReady {
				return nil
			}

			logger.Debug("Channel closed. Re-running init...")
			c.SendNotifyIsDisconnect(channel_error)

			if conn.IsClosed() {
				return errNotConnected
			}

			err := c.Connect(ctx, conn)
			if err != nil {
				continue
			}

			c.SendNotifyIsReconnect()

		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Channel) NotifyReconnect(receiver chan bool) chan bool {
	c.notifiedReconnect = append(c.notifiedReconnect, receiver)
	return receiver
}

func (c *Channel) SendNotifyIsReconnect() {
	for _, i := range c.notifiedReconnect {
		i <- true
	}
}

func (c *Channel) NotifyDisconnect(receiver chan *amqp.Error) chan *amqp.Error {
	c.notifiedDisconnect = append(c.notifiedDisconnect, receiver)
	return receiver
}

func (c *Channel) SendNotifyIsDisconnect(err *amqp.Error) {
	for _, i := range c.notifiedDisconnect {
		i <- err
	}
}

func (c *Channel) Close() error {
	c.IsReady = false

	for _, i := range c.notifiedReconnect {
		close(i)
	}

	for _, i := range c.notifiedDisconnect {
		close(i)
	}

	return c.Channeler.Close()
}
