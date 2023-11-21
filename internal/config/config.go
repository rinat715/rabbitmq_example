package config

import (
	"fmt"
	"os"

	"github.com/cristalhq/aconfig"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Config struct {
	Host        string `env:"HOST" usage:"host url rabbitmq"`
	Port        int    `env:"PORT" usage:"just a number"`
	User        string `env:"USER" usage:"login rabbitmq"`
	Pass        string `env:"PASS" usage:"pass rabbitmq"`
	LogLevel    int    `env:"LOGLEVEL" usage:"set number"`
	Definitions string `env:"DEFINITIONS" usage:"Definitions rabbitmq"`
}

func (c *Config) Get_url() string {
	// f"amqp://{self.user}:{self.password}@{self.host}:{self.port}"
	return fmt.Sprintf("amqp://%v:%v@%v:%v", c.User, c.Pass, c.Host, c.Port)
}

func (c *Config) Get_definitions() []byte {
	t, err := os.ReadFile(c.Definitions)
	check(err)
	return t
}

func NewConfig() *Config {
	var config Config
	loader := aconfig.LoaderFor(&config, aconfig.Config{
		SkipFiles:        true,
		SkipFlags:        true,
		AllFieldRequired: true,
	})

	err := loader.Load()
	check(err)
	return &config
}
