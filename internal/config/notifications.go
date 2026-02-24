package config

import (
	"fmt"
	"time"
)

type Notifications struct {
	RabbitMQURL     string
	ShutdownTimeout time.Duration
}

func LoadNotifications() (Notifications, error) {
	cfg := Notifications{
		RabbitMQURL:     getEnv("RABBITMQ_URL", ""),
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if cfg.RabbitMQURL == "" {
		return Notifications{}, fmt.Errorf("RABBITMQ_URL is required")
	}

	return cfg, nil
}
