package rabbitmq

import (
	"context"
	"errors"
	"spread_message/internal/logger"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	queueType    = "queue"
	exchangeType = "exchange"
)

type Connector interface {
	NotifyClose(receiver chan *amqp.Error) chan *amqp.Error
	Channel() (*amqp.Channel, error)
	Close() error
	IsClosed() bool
}

type Client struct {
	addr            string
	connection      Connector
	notifyConnClose chan *amqp.Error
	get_conn        func(url string, config amqp.Config) (*amqp.Connection, error)
	configAmqp      amqp.Config
	channels        []*Channel
	count_channels  int
	mu              sync.Mutex
	IsReady         bool
}

func (c *Client) connect() error {

	var err error
	c.connection, err = c.get_conn(c.addr, c.configAmqp)
	if err != nil {
		return err
	}

	c.notifyConnClose = c.connection.NotifyClose(make(chan *amqp.Error, 1))

	return nil
}

func (client *Client) Connect(ctx context.Context) {
	for {
		err := client.connect()

		if err != nil {
			logger.Debug("Failed to connect. Retrying...")

			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnectDelay):
				continue
			}

		}
		client.IsReady = true
		break
	}
}

func (client *Client) HandleReconnect(ctx context.Context) {
	for {

		select {
		case <-ctx.Done():
			return
		case <-client.notifyConnClose:
			logger.Debug("Connection closed. Reconnecting...")

			for {
				err := client.connect()

				if err != nil {
					logger.Debug("Failed to connect. Retrying...")

					select {
					case <-ctx.Done():
						return
					case <-time.After(reconnectDelay):
						continue
					}

				}

				break
			}

			logger.Info("Подключение восстановлено")
			for _, c := range client.channels {
				c.Connect(client.connection)
				go c.handleReConnect(client.connection)
				c.SendNotifyIsReconnect()
			}
		}
	}
}

func (c *Client) createCh() *Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		if c.IsReady {

			c.count_channels++
			channel := NewChannel()
			c.channels = append(c.channels, channel)

			channel.Connect(c.connection)
			go channel.handleReConnect(c.connection)

			return channel
		}

		<-time.After(reInitDelay)
	}
}

func (c *Client) NewProduser(exchange string, routing_key string) (*Producer, error) {
	return NewProduser(c.createCh(), exchange, routing_key)
}

func (c *Client) NewConsumer(queueName string, consumer string, callback func(data []byte) error) *Consumer {
	return NewConsumer(c.createCh(), queueName, consumer, callback)
}

func (c *Client) CreateDefinitions(definition *Definition) error {
	if definition == nil {
		return errors.New("empty definition struct")
	}
	channel := c.createCh()

	for _, q := range definition.Queues {
		err := channel.create_queue(q)
		if err != nil {
			logger.Error("не создалась очередь: %s\n", err)
		}
	}

	for _, e := range definition.Exchanges {
		err := channel.create_exchange(e)
		if err != nil {
			logger.Error("не создался обменник: %s\n", err)
		}
	}

	for _, b := range definition.Bindings {
		err := channel.create_bind(b)
		if err != nil {
			logger.Error("не создалась привязка: %s\n", err)
		}
	}
	return nil
}

func (client *Client) Close() error {
	logger.Debug("Close is stopping")
	if !client.IsReady {
		return errAlreadyClosed
	}

	for _, ch := range client.channels {
		ch.Close()
	}

	client.connection.Close()

	client.IsReady = false
	return nil
}

func New(name string, addr string) *Client {
	configAmqp := amqp.Config{Properties: amqp.NewConnectionProperties()}
	configAmqp.Properties.SetClientConnectionName(name)
	return &Client{configAmqp: configAmqp, addr: addr, get_conn: amqp.DialConfig}
}
