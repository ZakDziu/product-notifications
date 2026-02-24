package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"product-notifications/internal/products"

	"github.com/prometheus/client_golang/prometheus"
)

type mockRepo struct {
	createFn func(ctx context.Context, name string) (products.Product, error)
	deleteFn func(ctx context.Context, id int64) error
	listFn   func(ctx context.Context, limit, offset int) ([]products.Product, error)
	countFn  func(ctx context.Context) (int64, error)
}

func (m *mockRepo) Create(ctx context.Context, name string) (products.Product, error) {
	return m.createFn(ctx, name)
}
func (m *mockRepo) Delete(ctx context.Context, id int64) error {
	return m.deleteFn(ctx, id)
}
func (m *mockRepo) List(ctx context.Context, limit, offset int) ([]products.Product, error) {
	return m.listFn(ctx, limit, offset)
}
func (m *mockRepo) Count(ctx context.Context) (int64, error) {
	return m.countFn(ctx)
}

type mockPublisher struct {
	events []products.ProductEvent
	err    error
}

func (m *mockPublisher) Publish(_ context.Context, event products.ProductEvent) error {
	m.events = append(m.events, event)
	return m.err
}

func newTestService(repo Repository, pub Publisher) *Service {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return New(
		repo, pub, logger,
		prometheus.NewCounter(prometheus.CounterOpts{Name: "t_created", Help: "t"}),
		prometheus.NewCounter(prometheus.CounterOpts{Name: "t_deleted", Help: "t"}),
	)
}

func defaultRepo() *mockRepo {
	return &mockRepo{
		createFn: func(_ context.Context, name string) (products.Product, error) {
			return products.Product{ID: 1, Name: name, CreatedAt: time.Now()}, nil
		},
		deleteFn: func(_ context.Context, _ int64) error { return nil },
		listFn:   func(_ context.Context, _, _ int) ([]products.Product, error) { return nil, nil },
		countFn:  func(_ context.Context) (int64, error) { return 0, nil },
	}
}

func TestCreateProduct(t *testing.T) {
	errDB := errors.New("db down")

	tests := []struct {
		name      string
		input     string
		repoErr   error
		wantErr   error
		wantName  string
		wantEvent string
	}{
		{
			name:      "success",
			input:     "Phone",
			wantName:  "Phone",
			wantEvent: products.EventCreated,
		},
		{
			name:    "empty name",
			input:   "   ",
			wantErr: products.ErrInvalidName,
		},
		{
			name:    "repo error is wrapped",
			input:   "Phone",
			repoErr: errDB,
			wantErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := defaultRepo()
			if tt.repoErr != nil {
				repo.createFn = func(_ context.Context, _ string) (products.Product, error) {
					return products.Product{}, tt.repoErr
				}
			}
			pub := &mockPublisher{}
			svc := newTestService(repo, pub)

			product, err := svc.CreateProduct(context.Background(), tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error wrapping %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if product.Name != tt.wantName {
				t.Fatalf("want name %q, got %q", tt.wantName, product.Name)
			}
			if len(pub.events) != 1 || pub.events[0].EventType != tt.wantEvent {
				t.Fatalf("want event %q, got %v", tt.wantEvent, pub.events)
			}
		})
	}
}

func TestDeleteProduct(t *testing.T) {
	tests := []struct {
		name      string
		id        int64
		repoErr   error
		wantErr   error
		wantEvent string
	}{
		{
			name:      "success",
			id:        42,
			wantEvent: products.EventDeleted,
		},
		{
			name:    "not found",
			id:      999,
			repoErr: products.ErrNotFound,
			wantErr: products.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := defaultRepo()
			repo.deleteFn = func(_ context.Context, _ int64) error {
				return tt.repoErr
			}
			pub := &mockPublisher{}
			svc := newTestService(repo, pub)

			err := svc.DeleteProduct(context.Background(), tt.id)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pub.events) != 1 || pub.events[0].EventType != tt.wantEvent {
				t.Fatalf("want event %q, got %v", tt.wantEvent, pub.events)
			}
		})
	}
}

func TestListProducts(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		limit     int
		items     []products.Product
		total     int64
		wantLen   int
		wantTotal int64
		wantLimit int
		wantOff   int
	}{
		{
			name:  "page 2 with limit 2",
			page:  2,
			limit: 2,
			items: []products.Product{
				{ID: 3, Name: "C"},
				{ID: 4, Name: "D"},
			},
			total:     10,
			wantLen:   2,
			wantTotal: 10,
			wantLimit: 2,
			wantOff:   2,
		},
		{
			name:      "defaults for invalid input",
			page:      -1,
			limit:     0,
			items:     []products.Product{},
			total:     0,
			wantLen:   0,
			wantTotal: 0,
			wantLimit: 10,
			wantOff:   0,
		},
		{
			name:      "limit capped at 100",
			page:      1,
			limit:     500,
			items:     []products.Product{},
			total:     0,
			wantLen:   0,
			wantTotal: 0,
			wantLimit: 100,
			wantOff:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := defaultRepo()
			repo.listFn = func(_ context.Context, limit, offset int) ([]products.Product, error) {
				if limit != tt.wantLimit {
					t.Fatalf("want limit %d, got %d", tt.wantLimit, limit)
				}
				if offset != tt.wantOff {
					t.Fatalf("want offset %d, got %d", tt.wantOff, offset)
				}
				return tt.items, nil
			}
			repo.countFn = func(_ context.Context) (int64, error) {
				return tt.total, nil
			}

			pub := &mockPublisher{}
			svc := newTestService(repo, pub)

			items, total, err := svc.ListProducts(context.Background(), tt.page, tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) != tt.wantLen {
				t.Fatalf("want %d items, got %d", tt.wantLen, len(items))
			}
			if total != tt.wantTotal {
				t.Fatalf("want total %d, got %d", tt.wantTotal, total)
			}
		})
	}
}

func TestCreateProduct_PublishFail_StillReturnsProduct(t *testing.T) {
	repo := defaultRepo()
	pub := &mockPublisher{err: errors.New("broker down")}
	svc := newTestService(repo, pub)

	product, err := svc.CreateProduct(context.Background(), "Widget")
	if err != nil {
		t.Fatalf("expected no error despite publish failure, got: %v", err)
	}
	if product.Name != "Widget" {
		t.Fatalf("want name Widget, got %q", product.Name)
	}
}
