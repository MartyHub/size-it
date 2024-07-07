package internal

import (
	"net"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL       string
	Dev               bool
	EmptySessionsTick time.Duration `envDefault:"1h"`
	Host              string
	MaxInactiveTime   time.Duration `envDefault:"5s"`
	Path              string
	Port              int `envDefault:"8080"`
}

func ParseConfig() (Config, error) {
	var cfg Config

	err := env.ParseWithOptions(&cfg, env.Options{
		Prefix:                "SIZE_IT_",
		UseFieldNameByDefault: true,
	})

	return cfg, err
}

func (cfg Config) Address() string {
	return net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}
