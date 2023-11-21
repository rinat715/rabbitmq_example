package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml"

	"spread_message/internal/config"
	"spread_message/internal/logger"
	"spread_message/internal/models"
	"spread_message/internal/rabbitmq"
)

var (
	errTelegramConsumer = errors.New("telegram always falls")
	errPhoneConsumer    = errors.New("phone not send message error")
	errEmailConsumer    = errors.New("email not send message error")
	errShutdown         = errors.New("client is shutting down")
)

func create_definitions(config *config.Config) *rabbitmq.Definition {
	definition := rabbitmq.Definition{}
	t := config.Get_definitions()
	toml.Unmarshal(t, &definition)
	return &definition
}

func main() {

	// глобальные переменные
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		<-c
		cancel()
	}()

	config := config.NewConfig()
	addr := config.Get_url()

	client := rabbitmq.New("test_connect", addr)

	go func() {
		<-ctx.Done()
		err := client.Close()
		if err != nil {
			logger.Error("ошибка при остановке сервиса: %s\n", err)
		}
	}()

	client.Connect(ctx)

	if !client.IsReady {
		return
	}

	go client.HandleReconnect(ctx)

	definition := create_definitions(config)
	err := client.CreateDefinitions(definition)
	if err != nil {
		logger.Error("ошибка при создании definitions: %s\n", err)
	}

	/// generator messages
	messages := make(chan models.Message)
	wg.Add(1)
	go func(out chan models.Message) {
		defer wg.Done()
		for {
			out <- models.Message{
				Email:    "foo@bar.com",
				Telegram: "@foobar",
				Phone:    123456789,
				Code:     "ABCD",
				Uuid:     uuid.New().String(),
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * 2):
				continue
			}
		}
	}(messages)

	// консумер1
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer := client.NewConsumer("default", "TelegramConsumer", func(data []byte) error {
			return errTelegramConsumer
		})
		if client.IsReady {
			consumer.Consume(ctx)
		}
	}()

	// консумер2
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer := client.NewConsumer("phone", "PhoneConsumer", func(data []byte) error {
			if rand.Float64() > 0.5 {
				return errPhoneConsumer
			} else {
				fmt.Printf("PhoneConsumer отправил сообщение: %c\n", data)
			}
			return nil
		})
		if client.IsReady {
			consumer.Consume(ctx)
		}
	}()

	// консумер3
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer := client.NewConsumer("mail", "EmailConsumer", func(data []byte) error {
			if rand.Float64() < 0.2 {
				return errEmailConsumer
			} else {
				fmt.Printf("EmailConsumer отправил сообщение: %c\n", data)
			}

			return nil
		})
		if client.IsReady {
			consumer.Consume(ctx)
		}
	}()

	// паблишер
	wg.Add(1)
	go func(in chan models.Message) {
		defer wg.Done()
		pusher, err := client.NewProduser("", "default")
		if err != nil {
			logger.Error("пушер не запустился: %s\n", err)
		}

		go pusher.ConfirmHandler(ctx)
		go pusher.ReInit(ctx)

		for {
			select {
			case message := <-in:
				raw, err := message.MarshalJSON()
				if err != nil {
					logger.Error("Некорректный мессадж: %s\n", err)
				}
				if client.IsReady {
					if err := pusher.Publish(ctx, raw); err != nil {
						logger.Error("Push failed: %s\n", err)
					} else {
						logger.Debug("Push succeeded!")
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}(messages)

	wg.Wait()
}
