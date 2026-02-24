package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"product-notifications/internal/products"

	"github.com/gin-gonic/gin"
)

const (
	defaultPage  = 1
	defaultLimit = 10
)

type ProductService interface {
	CreateProduct(ctx context.Context, name string) (products.Product, error)
	DeleteProduct(ctx context.Context, id int64) error
	ListProducts(ctx context.Context, page, limit int) ([]products.Product, int64, error)
}

type Handler struct {
	service ProductService
}

func NewHandler(svc ProductService) *Handler {
	return &Handler{service: svc}
}

type createProductRequest struct {
	Name string `json:"name" binding:"required" example:"iPhone 16"`
}

type errorResponse struct {
	Error string `json:"error" example:"product not found"`
}

type listProductsResponse struct {
	Items      []products.Product `json:"items"`
	Pagination paginationMeta     `json:"pagination"`
}

type paginationMeta struct {
	Page  int   `json:"page" example:"1"`
	Limit int   `json:"limit" example:"10"`
	Total int64 `json:"total" example:"42"`
}

// CreateProduct godoc
// @Summary      Create a new product
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        body  body      createProductRequest  true  "Product data"
// @Success      201   {object}  products.Product
// @Failure      400   {object}  errorResponse
// @Failure      500   {object}  errorResponse
// @Router       /products [post]
func (h *Handler) CreateProduct(c *gin.Context) {
	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	product, err := h.service.CreateProduct(c.Request.Context(), req.Name)
	if err != nil {
		if errors.Is(err, products.ErrInvalidName) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// DeleteProduct godoc
// @Summary      Delete a product by ID
// @Tags         products
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      204
// @Failure      400  {object}  errorResponse
// @Failure      404  {object}  errorResponse
// @Failure      500  {object}  errorResponse
// @Router       /products/{id} [delete]
func (h *Handler) DeleteProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid product id"})
		return
	}

	if err := h.service.DeleteProduct(c.Request.Context(), id); err != nil {
		if errors.Is(err, products.ErrNotFound) {
			c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to delete product"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListProducts godoc
// @Summary      List products with pagination
// @Tags         products
// @Produce      json
// @Param        page   query     int  false  "Page number"   default(1)
// @Param        limit  query     int  false  "Items per page" default(10)
// @Success      200    {object}  listProductsResponse
// @Failure      500    {object}  errorResponse
// @Router       /products [get]
func (h *Handler) ListProducts(c *gin.Context) {
	page := parseQueryInt(c.Query("page"), defaultPage)
	limit := parseQueryInt(c.Query("limit"), defaultLimit)

	items, total, err := h.service.ListProducts(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get products"})
		return
	}

	c.JSON(http.StatusOK, listProductsResponse{
		Items: items,
		Pagination: paginationMeta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

func parseQueryInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
