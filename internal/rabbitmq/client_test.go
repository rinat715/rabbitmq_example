package rabbitmq

import (
	"testing"

	toml "github.com/pelletier/go-toml"
)

func TestDefinition(t *testing.T) {
	document := []byte(`
	[[queues]]
	name = "default"
	durable = true
	auto_delete = false
	[queues.arguments]
	x-dead-letter-exchange = "dlx"
	x-dead-letter-routing-key = "phone"
	x-queue-type = "classic"
	`)

	definition := Definition{}
	toml.Unmarshal(document, &definition)
	if len(definition.Queues) != 1 {
		t.Error("Exepted count Queues: 1")
	}
	if definition.Queues[0].Name != "default" {
		t.Error("Wrong queue name")
	}

	if definition.Queues[0].Arguments["x-dead-letter-exchange"] != "dlx" {
		t.Error("Wrong argument x-dead-letter-exchange")
	}

}

// type MyMockedObject struct {
// 	mock.Mock
// }

// func mock_get_conn(url string, config amqp.Config) (*amqp.Connection, error) {
// 	return new(MyMockedObject), nil
// }

// func TestConnect(t *testing.T) {

// 	Client{"test"}
// }
