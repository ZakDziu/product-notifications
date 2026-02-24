package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"product-notifications/internal/config"
	"product-notifications/internal/notifications"
	"product-notifications/internal/products"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	os.Exit(run(logger))
}

func run(logger *slog.Logger) int {
	cfg, err := config.LoadNotifications()
	if err != nil {
		logger.Error("load config", "error", err)
		return 1
	}

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Error("connect rabbitmq", "error", err)
		return 1
	}
	defer conn.Close()

	consumer, err := notifications.NewConsumer(conn, products.EventsQueue, logger)
	if err != nil {
		logger.Error("init consumer", "error", err)
		return 1
	}
	defer consumer.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("notifications service started")
		errCh <- consumer.Listen(ctx)
	}()

	waitForDrain := false
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		waitForDrain = true
	case err := <-errCh:
		if err != nil {
			logger.Error("consumer failed", "error", err)
			return 1
		}
	}

	if waitForDrain {
		shutdownDeadline := time.NewTimer(cfg.ShutdownTimeout)
		defer shutdownDeadline.Stop()
		select {
		case err := <-errCh:
			if err != nil {
				logger.Error("consumer stop failed", "error", err)
				return 1
			}
		case <-shutdownDeadline.C:
			logger.Warn("consumer shutdown timeout reached")
		}
	}

	logger.Info("notifications service stopped")
	return 0
}
