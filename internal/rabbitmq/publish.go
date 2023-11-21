package rabbitmq

import (
	"context"
	"fmt"
	"spread_message/internal/logger"

	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Producer struct {
	exchange      string
	routing_key   string
	channel       *Channel
	publishes     chan uint64            // номера отправленных сообщений
	confirms      chan amqp.Confirmation //
	IsReconnected chan bool
}

func (p *Producer) ReInit(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.IsReconnected:
			p.confirms = p.channel.ch.NotifyPublish(make(chan amqp.Confirmation, 1))
			logger.Info("паблишер реконнетед")
		}
	}
}

func (p *Producer) Publish(ctx context.Context, body []byte) error {

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var seqNo uint64
	for {
		if p.channel.IsReady && p.channel.ch != nil {
			seqNo = p.channel.ch.GetNextPublishSeqNo()
			break
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(resendDelay):
			continue
		}
	}

	for {
		if p.channel.IsReady && p.channel.ch != nil {
			if err := p.channel.ch.PublishWithContext(ctx,
				p.exchange,    // publish to an exchange
				p.routing_key, // routing to 0 or more queues
				false,         // mandatory  указание Rabbit складировать сообщения, не имеющие маршрута в какую-либо очередь в отдельный Exchange
				false,         // immediate
				amqp.Publishing{
					Headers:         amqp.Table{},
					ContentType:     "application/json",
					ContentEncoding: "application/json",
					Body:            body,
					DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
					Priority:        0,              // 0-9
					// a bunch of application/implementation-specific fields
				},
			); err != nil {
				return fmt.Errorf("exchange publish: %s", err)
			}
			p.publishes <- seqNo
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(resendDelay):
			continue
		}
	}
}

func (p *Producer) ConfirmHandler(ctx context.Context) {
	m := make(map[uint64]bool, 8)
	for {
		select {
		case <-ctx.Done():
			logger.Debug("confirmHandler is stopping")
			if len(m) > 1 {
				logger.Debug("outstanding confirmations: %d", len(m))
			}
			return
		case publishSeqNo := <-p.publishes:
			logger.Debug("waiting for confirmation of %d", publishSeqNo)
			m[publishSeqNo] = false
		case confirmed := <-p.confirms:
			if confirmed.DeliveryTag > 0 {
				if confirmed.Ack {
					logger.Debug("confirmed delivery with delivery tag: %d", confirmed.DeliveryTag)
				} else {
					logger.Debug("failed delivery of delivery tag: %d", confirmed.DeliveryTag)
				}
				delete(m, confirmed.DeliveryTag)
			}
		}
	}
}

func NewProduser(channel *Channel, exchange string, routing_key string) (*Producer, error) {
	publisher := &Producer{
		exchange,
		routing_key,
		channel,
		make(chan uint64, 8),
		channel.ch.NotifyPublish(make(chan amqp.Confirmation, 1)),
		channel.NotifyReconnect(make(chan bool, 1)),
	}
	return publisher, nil
}
