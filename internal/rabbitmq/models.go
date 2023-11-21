package rabbitmq

type Queue struct {
	Name       string
	Durable    bool
	AutoDelete bool `toml:"auto_delete"`
	Arguments  map[string]interface{}
}

type Exchange struct {
	Name      string
	Type      string
	Arguments map[string]interface{}
}

type Binding struct {
	Source      string
	Destination string
	RoutingKey  string `toml:"routing_key"`
	Type        string
	Arguments   map[string]interface{}
}
type Definition struct {
	Queues    []Queue
	Exchanges []Exchange
	Bindings  []Binding
}
