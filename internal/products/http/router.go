package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	healthStatusOK        = "ok"
	healthStatusUnhealthy = "unhealthy"
)

type HealthChecker interface {
	Health() error
}

func RegisterRoutes(router *gin.Engine, handler *Handler, checker HealthChecker) {
	router.POST("/products", handler.CreateProduct)
	router.GET("/products", handler.ListProducts)
	router.DELETE("/products/:id", handler.DeleteProduct)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/healthz", func(c *gin.Context) {
		if err := checker.Health(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": healthStatusUnhealthy})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": healthStatusOK})
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
