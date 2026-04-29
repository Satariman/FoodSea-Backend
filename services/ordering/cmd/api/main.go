package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/sync/errgroup"

	_ "github.com/foodsea/ordering/docs/swagger"
	_ "github.com/foodsea/ordering/ent/runtime"
	"github.com/foodsea/ordering/internal/modules/orders"
	"github.com/foodsea/ordering/internal/modules/saga"
	"github.com/foodsea/ordering/internal/platform/cache"
	"github.com/foodsea/ordering/internal/platform/config"
	"github.com/foodsea/ordering/internal/platform/database"
	"github.com/foodsea/ordering/internal/platform/grpcclient"
	"github.com/foodsea/ordering/internal/platform/grpcserver"
	"github.com/foodsea/ordering/internal/platform/kafka"
	"github.com/foodsea/ordering/internal/platform/logger"
	"github.com/foodsea/ordering/internal/platform/middleware"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	log := logger.New(cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Database
	entClient, _, err := database.Open(ctx, cfg.DB, log)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer entClient.Close()

	// Redis (optional — errors are non-fatal)
	redisCache, err := cache.NewRedis(cfg.Redis.URL)
	if err != nil {
		log.WarnContext(ctx, "redis unavailable, cache disabled", "error", err)
		redisCache = nil
	}
	if redisCache != nil {
		defer redisCache.Close()
	}

	// gRPC clients to upstream services
	grpcClients, err := grpcclient.Dial(ctx, *cfg, log)
	if err != nil {
		return fmt.Errorf("dialing grpc clients: %w", err)
	}
	defer grpcClients.Close()

	// Kafka producers
	orderProducer := kafka.NewProducer(cfg.Kafka.Brokers, "order.events", log)
	defer orderProducer.Close()

	sagaCmdProducer := kafka.NewProducer(cfg.Kafka.Brokers, "saga.commands", log)
	defer sagaCmdProducer.Close()

	// Kafka consumer for saga replies (audit log)
	sagaReplyConsumer := kafka.NewConsumer(cfg.Kafka.Brokers, "saga.replies", "ordering-saga", log)
	defer sagaReplyConsumer.Close()

	sagaReplyProducer := kafka.NewProducer(cfg.Kafka.Brokers, "saga.replies", log)
	defer sagaReplyProducer.Close()

	// Orders module
	ordersModule := orders.NewModule(orders.Deps{
		Ent:      entClient,
		Producer: orderProducer,
		Log:      log,
	})

	// Saga module
	sagaModule := saga.NewModule(saga.Deps{
		Ent:             entClient,
		OrdersFacade:    ordersModule.OrderFacade(),
		CartClient:      grpcClients.Cart,
		OptClient:       grpcClients.Optimization,
		CommandProducer: sagaCmdProducer,
		ReplyProducer:   sagaReplyProducer,
		ReplyConsumer:   sagaReplyConsumer,
		Log:             log,
		StepTimeout:     cfg.Saga.StepTimeout,
		MaxCompAttempts: cfg.Saga.MaxCompensationAttempts,
	})

	// HTTP server
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(
		middleware.Recovery(log),
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.CORS(middleware.DefaultCORSConfig()),
	)

	// Health endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "ordering-service"})
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Protected routes group
	auth := router.Group("/api/v1")
	auth.Use(middleware.Auth(cfg.JWT.Secret))
	ordersModule.RegisterRoutes(auth)
	sagaModule.RegisterRoutes(auth)

	// WriteTimeout 90s: POST /orders blocks until saga completes (up to 60s).
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// gRPC server
	grpcSrv := grpcserver.New(log)
	grpcListener, err := grpcserver.Listen(cfg.GRPC.Port)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}
	// TODO(ordering/99-integration): register module gRPC servers

	eg, egCtx := errgroup.WithContext(ctx)

	// HTTP server goroutine
	eg.Go(func() error {
		log.InfoContext(egCtx, "http server started", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	// gRPC server goroutine
	eg.Go(func() error {
		log.InfoContext(egCtx, "grpc server started", "port", cfg.GRPC.Port)
		if err := grpcSrv.Serve(grpcListener); err != nil {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})

	// Saga recovery — single pass at startup, goroutine exits when done.
	eg.Go(func() error {
		log.InfoContext(egCtx, "saga recovery started")
		if err := sagaModule.RecoverPending(egCtx); err != nil {
			log.WarnContext(egCtx, "saga recovery error", "error", err)
		}
		return nil
	})

	// Saga reply consumer goroutine
	eg.Go(func() error {
		log.InfoContext(egCtx, "kafka consumer started", "topic", "saga.replies")
		if err := sagaModule.RunConsumer(egCtx); err != nil {
			return fmt.Errorf("saga reply consumer: %w", err)
		}
		return nil
	})

	// Shutdown listener goroutine
	eg.Go(func() error {
		<-egCtx.Done()
		log.InfoContext(context.Background(), "shutdown initiated")

		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()

		grpcSrv.GracefulStop()

		if err := httpServer.Shutdown(shutCtx); err != nil {
			log.WarnContext(shutCtx, "http server shutdown error", "error", err)
		}

		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	log.InfoContext(context.Background(), "ordering-service stopped")
	return nil
}
