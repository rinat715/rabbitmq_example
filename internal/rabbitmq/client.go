package rabbitmq

import (
	"context"
	"errors"
	"spread_message/internal/logger"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
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
	Connect(ctx context.Context) error
	HandleReconnect(ctx context.Context) error
}

type Connection struct {
	*amqp.Connection
	addr           string
	name           string
	IsDisconnected chan *amqp.Error
	IsReconnected  chan bool
}

func (conn *Connection) connect() (*amqp091.Connection, error) {
	configAmqp := amqp.Config{Properties: amqp.NewConnectionProperties()}
	configAmqp.Properties.SetClientConnectionName(conn.name)

	return amqp.DialConfig(conn.addr, configAmqp)
}

func (conn *Connection) Connect(ctx context.Context) error {
	for {
		connect, err := conn.connect()

		if err != nil {
			logger.Debug("Failed to connect. Retrying...")

			select {
			case <-ctx.Done():
				return errNotConnected // Заменить на ошибку Context end
			case <-time.After(reconnectDelay):
				continue
			}
		}
		conn.Connection = connect
		conn.IsDisconnected = conn.NotifyClose(make(chan *amqp.Error, 1))

		return nil
	}
}

func (conn *Connection) HandleReconnect(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return errNotConnected // Заменить на ошибку Context end
		case <-conn.IsDisconnected:
			logger.Debug("Connection closed. Reconnecting...")

			err := conn.Connect(ctx)
			if err != nil {
				return err
			}
			conn.IsReconnected <- true
		}
	}
}

type ChannelPool struct {
	mu             sync.Mutex
	channels       []*Channel
	count_channels int
}

func (c *ChannelPool) appendCh(channel *Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.channels = append(c.channels, channel)
	c.count_channels++
}

type Client struct {
	ChannelPool
	connection    Connector
	IsReconnected chan bool
}

func (client *Client) Connect(ctx context.Context) error {
	err := client.connection.Connect(ctx)
	if err == nil {
		go client.connection.HandleReconnect(ctx)
	}
	return err
}

func (client *Client) HandleReconnect(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return errNotConnected // Заменить на ошибку Context end
		case <-client.IsReconnected:
			logger.Info("Подключение восстановлено")

			for _, channel := range client.channels {
				err := channel.Connect(ctx, client.connection) // Заменить на ошибку Context end
				if err != nil {
					channel.SendNotifyIsReconnect()
				}
			}
		}
	}
}

func (c *Client) createCh(ctx context.Context) (*Channel, error) {
	if c.connection.IsClosed() {
		return nil, errNotConnected
	}

	channel := &Channel{}

	for {
		err := channel.Connect(ctx, c.connection)

		if err == nil {
			go channel.handleReConnect(ctx, c.connection) // кто обработает ошибку errNotConnected?
			c.appendCh(channel)
			return channel, nil
		}

		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(reInitDelay):
			continue
		}
	}
}

func (c *Client) NewProduser(ctx context.Context, exchange string, routing_key string) (*Producer, error) {
	channel, err := c.createCh(ctx)
	if err != nil {
		return nil, err
	}
	return NewProduser(channel, exchange, routing_key)
}

func (c *Client) NewConsumer(ctx context.Context, queueName string, consumer string, callback func(data []byte) error) (*Consumer, error) {
	channel, err := c.createCh(ctx)
	if err != nil {
		return nil, err
	}
	return NewConsumer(channel, queueName, consumer, callback), nil
}

func (c *Client) CreateDefinitions(ctx context.Context, definition *Definition) error {

	if definition == nil {
		return errors.New("empty definition struct")
	}
	channel, err := c.createCh(ctx) // TODO удалить после использования
	logger.Debug("создан канал")
	if err != nil {
		return err
	}

	for _, queue := range definition.Queues {
		_, err := channel.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, false, false, queue.Arguments)
		if err != nil {
			logger.Error("не создалась очередь: %s\n", err)
		}
	}

	for _, exchange := range definition.Exchanges {
		err := channel.ExchangeDeclare(exchange.Name, exchange.Type, true, false, false, false, exchange.Arguments)
		if err != nil {
			logger.Error("не создался обменник: %s\n", err)
		}
	}

	for _, binding := range definition.Bindings {
		if binding.Type == exchangeType {
			err := channel.ExchangeBind(binding.Destination, binding.RoutingKey, binding.Source, false, binding.Arguments)
			if err != nil {
				logger.Error("не создалась привязка: %s\n", err)
			}
		}

		if binding.Type == queueType {
			err := channel.QueueBind(binding.Destination, binding.RoutingKey, binding.Source, false, binding.Arguments)
			if err != nil {
				logger.Error("не создалась привязка: %s\n", err)
			}
		}
	}
	err = channel.Close()
	if err != nil {
		logger.Error("не закрылся канал: %s\n", err)
		return err
	}
	return nil

}

func (client *Client) Close() error {
	logger.Debug("client is stopping")
	if client.connection.IsClosed() {
		return errAlreadyClosed
	}

	for _, ch := range client.channels {
		err := ch.Close()
		if err != nil {
			logger.Error("не закрылся канал: %s\n", err)
		}
	}

	return client.connection.Close()
}

func (client *Client) IsReady() bool {
	return !client.connection.IsClosed()
}

func New(name string, addr string) *Client {
	client := &Client{IsReconnected: make(chan bool)}
	client.connection = &Connection{name: name, addr: addr, IsReconnected: client.IsReconnected}
	return client
}
