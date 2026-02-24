package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"product-notifications/internal/config"
	"product-notifications/internal/products"
	producthttp "product-notifications/internal/products/http"
	"product-notifications/internal/products/messaging"
	"product-notifications/internal/products/repository"
	"product-notifications/internal/products/service"

	_ "product-notifications/docs"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	metricCreatedTotal    = "products_created_total"
	metricDeletedTotal    = "products_deleted_total"
	migrateSourcePrefix   = "file://"
	postgresDriverName    = "postgres"
)

// @title        Products API
// @version      1.0
// @description  Product management microservice with event notifications.
// @host         localhost:8080
// @BasePath     /
func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadProducts()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		logger.Error("run migrations", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open(postgresDriverName, cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}

	rabbitConn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Error("connect rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	publisher, err := messaging.NewRabbitPublisher(rabbitConn, products.EventsQueue)
	if err != nil {
		logger.Error("init publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	createdCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: metricCreatedTotal,
		Help: "Total number of products created",
	})
	deletedCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: metricDeletedTotal,
		Help: "Total number of products deleted",
	})
	prometheus.MustRegister(createdCounter, deletedCounter)

	repo := repository.NewPostgres(db)
	svc := service.New(repo, publisher, logger, createdCounter, deletedCounter)
	handler := producthttp.NewHandler(svc)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(producthttp.RequestIDMiddleware())
	router.Use(producthttp.AccessLogMiddleware(logger))
	producthttp.RegisterRoutes(router, handler, repo)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("products service started", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("http server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("products service stopped")
}

func runMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New(migrateSourcePrefix+migrationsPath, databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
