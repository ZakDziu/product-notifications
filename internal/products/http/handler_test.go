package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"product-notifications/internal/products"

	"github.com/gin-gonic/gin"
)

type stubService struct {
	createFn func(ctx context.Context, name string) (products.Product, error)
	deleteFn func(ctx context.Context, id int64) error
	listFn   func(ctx context.Context, page, limit int) ([]products.Product, int64, error)
}

func (s *stubService) CreateProduct(ctx context.Context, name string) (products.Product, error) {
	return s.createFn(ctx, name)
}
func (s *stubService) DeleteProduct(ctx context.Context, id int64) error {
	return s.deleteFn(ctx, id)
}
func (s *stubService) ListProducts(ctx context.Context, page, limit int) ([]products.Product, int64, error) {
	return s.listFn(ctx, page, limit)
}

func setupRouter(svc ProductService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(svc)
	r.POST("/products", h.CreateProduct)
	r.GET("/products", h.ListProducts)
	r.DELETE("/products/:id", h.DeleteProduct)
	return r
}

func TestHandler_CreateProduct(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		svcProduct products.Product
		svcErr     error
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"name":"Laptop"}`,
			svcProduct: products.Product{ID: 1, Name: "Laptop"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty body",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation error",
			body:       `{"name":"x"}`,
			svcErr:     products.ErrInvalidName,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{
				createFn: func(_ context.Context, name string) (products.Product, error) {
					if tt.svcErr != nil {
						return products.Product{}, tt.svcErr
					}
					return tt.svcProduct, nil
				},
			}

			r := setupRouter(svc)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("want status %d, got %d, body: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_DeleteProduct(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		svcErr     error
		wantStatus int
	}{
		{
			name:       "success",
			url:        "/products/1",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "not found",
			url:        "/products/999",
			svcErr:     products.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			url:        "/products/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{
				deleteFn: func(_ context.Context, _ int64) error {
					return tt.svcErr
				},
			}

			r := setupRouter(svc)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("want status %d, got %d, body: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_ListProducts(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		items      []products.Product
		total      int64
		wantStatus int
		wantLen    int
	}{
		{
			name: "returns items",
			url:  "/products?page=1&limit=2",
			items: []products.Product{
				{ID: 1, Name: "A"},
				{ID: 2, Name: "B"},
			},
			total:      5,
			wantStatus: http.StatusOK,
			wantLen:    2,
		},
		{
			name:       "empty list",
			url:        "/products",
			items:      []products.Product{},
			total:      0,
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{
				listFn: func(_ context.Context, _, _ int) ([]products.Product, int64, error) {
					return tt.items, tt.total, nil
				},
			}

			r := setupRouter(svc)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("want status %d, got %d", tt.wantStatus, w.Code)
			}

			var resp listProductsResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Items) != tt.wantLen {
				t.Fatalf("want %d items, got %d", tt.wantLen, len(resp.Items))
			}
		})
	}
}
