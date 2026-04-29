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
	"google.golang.org/grpc"

	_ "github.com/foodsea/optimization/docs/swagger"
	_ "github.com/foodsea/optimization/ent/runtime"
	analogs "github.com/foodsea/optimization/internal/modules/analogs"
	optimizer "github.com/foodsea/optimization/internal/modules/optimizer"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/sync/errgroup"

	"github.com/foodsea/optimization/internal/platform/cache"
	"github.com/foodsea/optimization/internal/platform/config"
	"github.com/foodsea/optimization/internal/platform/database"
	"github.com/foodsea/optimization/internal/platform/grpcclient"
	"github.com/foodsea/optimization/internal/platform/grpcserver"
	"github.com/foodsea/optimization/internal/platform/kafka"
	"github.com/foodsea/optimization/internal/platform/logger"
	"github.com/foodsea/optimization/internal/platform/middleware"
)

// @title FoodSea Optimization Service API
// @version 1.0
// @description Сервис оптимизации корзины и подбора аналогов.
// @host localhost:8082
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT токен. Формат: "Bearer {token}"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	entClient, _, err := database.Open(ctx, cfg.DB, log)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	var redisCache *cache.RedisCache
	if cfg.Redis.URL != "" {
		redisCache, err = cache.NewRedis(cfg.Redis.URL)
		if err != nil {
			log.WarnContext(ctx, "redis unavailable, cache disabled", "error", err)
			redisCache = nil
		}
	}

	grpcClients, err := grpcclient.Dial(ctx, *cfg, log)
	if err != nil {
		if redisCache != nil {
			_ = redisCache.Close()
		}
		_ = entClient.Close()
		return fmt.Errorf("dialing grpc clients: %w", err)
	}

	optProducer := kafka.NewProducer(cfg.Kafka.Brokers, "optimization.events", log)
	cartConsumer := kafka.NewConsumer(cfg.Kafka.Brokers, "cart.events", "optimization-group", log)

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

	router.GET("/health", healthHandler)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := router.Group("/api/v1")
	auth.Use(middleware.Auth(cfg.JWT.Secret))

	analogsModule := analogs.NewModule(analogs.Deps{
		MLClient: grpcClients.Analog,
		Cache:    redisCache,
		Log:      log,
	})
	optimizerModule := optimizer.NewModule(&optimizer.Deps{
		Ent:                  entClient,
		CartClient:           grpcClients.Cart,
		OfferClient:          grpcClients.Offer,
		AnalogProvider:       analogsModule.Provider,
		GetAnalogsForProduct: analogsModule.GetAnalogsForProduct,
		Producer:             optProducer,
		Cache:                redisCache,
		Timeout:              cfg.OptimizationTimeout,
		Log:                  log,
	})
	optimizerModule.RegisterRoutes(auth)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	grpcSrv := grpcserver.New(log)
	grpcListener, err := grpcserver.Listen(cfg.GRPC.Port)
	if err != nil {
		_ = cartConsumer.Close()
		_ = optProducer.Close()
		_ = grpcClients.Close()
		if redisCache != nil {
			_ = redisCache.Close()
		}
		_ = entClient.Close()
		return fmt.Errorf("grpc listen: %w", err)
	}
	optimizerModule.RegisterGRPC(grpcSrv)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		log.InfoContext(egCtx, "http server started", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		log.InfoContext(egCtx, "grpc server started", "port", cfg.GRPC.Port)
		if err := grpcSrv.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		log.InfoContext(egCtx, "kafka consumer started", "topic", "cart.events")
		if err := cartConsumer.Run(egCtx, func(ctx context.Context, event kafka.Event) error {
			return optimizerModule.HandleCartEvent(ctx, &event)
		}); err != nil {
			return fmt.Errorf("cart consumer: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-egCtx.Done():
				return nil
			case <-ticker.C:
				cutoff := time.Now().Add(-cfg.ResultTTL)
				n, err := optimizerModule.ExpireOld(egCtx, cutoff)
				if err != nil {
					log.WarnContext(egCtx, "expire old optimization results failed", "error", err)
					continue
				}
				if n > 0 {
					log.InfoContext(egCtx, "expired old optimization results", "count", n)
				}
			}
		}
	})

	eg.Go(func() error {
		<-egCtx.Done()
		log.InfoContext(context.Background(), "shutdown initiated")

		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()

		grpcSrv.GracefulStop()

		if err := httpServer.Shutdown(shutCtx); err != nil {
			log.WarnContext(shutCtx, "http server shutdown error", "error", err)
		}

		if err := cartConsumer.Close(); err != nil {
			log.WarnContext(shutCtx, "kafka consumer close error", "error", err)
		}
		if err := optProducer.Close(); err != nil {
			log.WarnContext(shutCtx, "kafka producer close error", "error", err)
		}
		if err := grpcClients.Close(); err != nil {
			log.WarnContext(shutCtx, "grpc clients close error", "error", err)
		}
		if redisCache != nil {
			if err := redisCache.Close(); err != nil {
				log.WarnContext(shutCtx, "redis close error", "error", err)
			}
		}
		if err := entClient.Close(); err != nil {
			log.WarnContext(shutCtx, "ent client close error", "error", err)
		}

		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	log.InfoContext(context.Background(), "optimization-service stopped")
	return nil
}

// healthHandler godoc
// @Summary      Health check
// @Tags         service
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "optimization-service"})
}
