package router

import (
	"database/sql"

	"foodsea-backend/internal/platform/config"
	"foodsea-backend/internal/platform/router/handlers"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(db *sql.DB, rdb *redis.Client, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Редирект с корня на Swagger
	r.GET("/", func(c *gin.Context) {
		c.Redirect(301, "/swagger/index.html")
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		healthHandler := handlers.NewHealthHandler(db, rdb)
		v1.GET("/health", healthHandler.Check)
	}

	// Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}

