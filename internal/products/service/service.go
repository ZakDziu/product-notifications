package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"product-notifications/internal/products"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

type Repository interface {
	Create(ctx context.Context, name string) (products.Product, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]products.Product, error)
	Count(ctx context.Context) (int64, error)
}

type Publisher interface {
	Publish(ctx context.Context, event products.ProductEvent) error
}

type Service struct {
	repo      Repository
	publisher Publisher
	logger    *slog.Logger
	created   prometheus.Counter
	deleted   prometheus.Counter
}

func New(repo Repository, publisher Publisher, logger *slog.Logger, created, deleted prometheus.Counter) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		created:   created,
		deleted:   deleted,
	}
}

func (s *Service) CreateProduct(ctx context.Context, name string) (products.Product, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return products.Product{}, products.ErrInvalidName
	}

	product, err := s.repo.Create(ctx, name)
	if err != nil {
		return products.Product{}, fmt.Errorf("repo create: %w", err)
	}

	if err := s.publisher.Publish(ctx, products.ProductEvent{
		EventType: products.EventCreated,
		ProductID: product.ID,
		Name:      product.Name,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		s.logger.Error("publish product_created event failed",
			"product_id", product.ID,
			"error", err,
		)
	}

	s.created.Inc()
	return product, nil
}

func (s *Service) DeleteProduct(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("repo delete: %w", err)
	}

	if err := s.publisher.Publish(ctx, products.ProductEvent{
		EventType: products.EventDeleted,
		ProductID: id,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		s.logger.Error("publish product_deleted event failed",
			"product_id", id,
			"error", err,
		)
	}

	s.deleted.Inc()
	return nil
}

func (s *Service) ListProducts(ctx context.Context, page, limit int) ([]products.Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	offset := (page - 1) * limit

	items, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repo list: %w", err)
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("repo count: %w", err)
	}

	return items, total, nil
}
