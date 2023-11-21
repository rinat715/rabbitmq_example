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
	ch              Channeler
	notifyChanClose chan *amqp.Error
	notified        []chan bool
	done            chan bool
	IsReady         bool
}

func (c *Channel) Connect(conn Connector) error {
	var err error
	if c.ch, err = conn.Channel(); err != nil {
		logger.Error(err)
		return err
	}

	if err := c.ch.Confirm(false); err != nil {
		logger.Error(err)
		return err
	}

	c.notifyChanClose = c.ch.NotifyClose(make(chan *amqp.Error, 1))
	c.IsReady = true

	return nil
}

func (c *Channel) handleReConnect(conn Connector) error {
	for {
		select {
		case <-c.notifyChanClose:
			c.IsReady = false

			if conn.IsClosed() {
				return errNotConnected
			}

			logger.Debug("Channel closed. Re-running init...")
			for {
				err := c.Connect(conn)

				if err != nil {
					select {
					case <-time.After(reInitDelay):
						continue
					case <-c.done:
						return nil
					}
				}
				c.SendNotifyIsReconnect()
				break
			}
		case <-c.done:
			return nil
		}
	}
}

func (c *Channel) NotifyReconnect(receiver chan bool) chan bool {
	c.notified = append(c.notified, receiver)
	return receiver
}

func (c *Channel) SendNotifyIsReconnect() {
	for _, i := range c.notified {
		i <- true
	}
}

func (c *Channel) Close() error {
	c.done <- true
	close(c.done)

	for _, i := range c.notified {
		close(i)
	}

	err := c.ch.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *Channel) create_queue(queue Queue) error {
	_, err := c.ch.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, false, false, queue.Arguments)
	return err
}

func (c *Channel) create_exchange(exchange Exchange) error {
	return c.ch.ExchangeDeclare(exchange.Name, exchange.Type, true, false, false, false, exchange.Arguments)
}

func (c *Channel) create_bind(binding Binding) error {
	if binding.Type == exchangeType {
		return c.ch.ExchangeBind(binding.Destination, binding.RoutingKey, binding.Source, false, binding.Arguments)
	}
	if binding.Type == queueType {
		return c.ch.QueueBind(binding.Destination, binding.RoutingKey, binding.Source, false, binding.Arguments)
	}
	return errors.New("wrong binding type") // TODO перенести в валидацию toml чтобы передавать в ошибке номер строчки

}

func NewChannel() *Channel {
	return &Channel{}
}
