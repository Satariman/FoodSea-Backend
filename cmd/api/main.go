package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"foodsea-backend/internal/platform/config"
	"foodsea-backend/internal/platform/database"
	"foodsea-backend/internal/platform/redis"
	"foodsea-backend/internal/platform/router"

	_ "foodsea-backend/docs"
)

// @title FoodSea Backend API
// @version 1.0
// @description API для мобильного приложения FoodSea
// @host localhost:8085
// @BasePath /api/v1
func main() {
	// Загрузка конфигурации
	cfg := config.Load()

	// Инициализация базы данных PostgreSQL
	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Инициализация Redis
	rdb, err := redis.NewRedisConnection(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	// Проверка подключений
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	log.Println("Database and Redis connections established successfully")

	// Настройка роутера
	r := router.SetupRouter(db.DB, rdb.Client, cfg)

	// Запуск HTTP сервера
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("Server is running on http://localhost:%s", cfg.Server.Port)
	log.Printf("Health check: http://localhost:%s/health", cfg.Server.Port)
	log.Printf("Swagger UI: http://localhost:%s/swagger/index.html", cfg.Server.Port)

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

