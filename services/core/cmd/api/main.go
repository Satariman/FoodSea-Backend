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

	// Import generated docs (populated after swag init)
	_ "github.com/foodsea/core/docs/swagger"
	// Initialize ent default functions (uuid, timestamps)
	_ "github.com/foodsea/core/ent/runtime"

	"github.com/foodsea/core/internal/modules/barcode"
	"github.com/foodsea/core/internal/modules/cart"
	"github.com/foodsea/core/internal/modules/catalog"
	"github.com/foodsea/core/internal/modules/identity"
	"github.com/foodsea/core/internal/modules/images"
	"github.com/foodsea/core/internal/modules/partners"
	"github.com/foodsea/core/internal/modules/search"
	"github.com/foodsea/core/internal/platform/cache"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	"github.com/foodsea/core/internal/platform/grpcserver"
	"github.com/foodsea/core/internal/platform/kafka"
	"github.com/foodsea/core/internal/platform/logger"
	"github.com/foodsea/core/internal/platform/middleware"
	s3platform "github.com/foodsea/core/internal/platform/s3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Env)

	// ── Infrastructure ────────────────────────────────────────────────────────

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	entClient, sqlDB, err := database.Open(ctx, cfg.DB, log)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer entClient.Close()

	redisCache, err := cache.NewRedis(cfg.Redis.URL)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisCache.Close()

	cartProducer := kafka.NewProducer(cfg.Kafka.Brokers, "cart.events", log)
	defer cartProducer.Close()

	s3Client, err := s3platform.NewClient(ctx, s3platform.Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		BucketName:      cfg.S3.BucketName,
		UseSSL:          cfg.S3.UseSSL,
		PublicBaseURL:   cfg.S3.PublicBaseURL,
	})
	if err != nil {
		log.Error("failed to initialise S3 client", "error", err)
		os.Exit(1)
	}

	// ── Modules ───────────────────────────────────────────────────────────────

	identityModule := identity.NewModule(identity.Deps{
		Ent:   entClient,
		Redis: redisCache.Client(),
		Cache: redisCache,
		Log:   log,
		JWT:   cfg.JWT,
	})
	partnersModule := partners.NewModule(partners.Deps{
		Ent:   entClient,
		Cache: redisCache,
		Log:   log,
	})
	catalogModule := catalog.NewModule(catalog.Deps{
		Ent:               entClient,
		Cache:             redisCache,
		Log:               log,
		BestOfferProvider: partnersModule.GetBestOffer,
	})
	cartModule := cart.NewModule(cart.Deps{
		Ent:      entClient,
		Producer: cartProducer,
		Log:      log,
	})
	searchModule := search.NewModule(search.Deps{
		Ent:   entClient,
		DB:    sqlDB,
		Cache: redisCache,
		Log:   log,
	})
	barcodeModule := barcode.NewModule(barcode.Deps{
		ProductGetter: catalogModule.ProductGetter(),
		Log:           log,
	})
	imagesModule := images.NewModule(images.Deps{
		EntClient: entClient,
		S3Client:  s3Client,
		Logger:    log,
	})

	// ── HTTP server ───────────────────────────────────────────────────────────

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

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/api/v1")
	public := v1
	protected := v1.Group("", middleware.Auth(cfg.JWT.Secret))
	admin := v1.Group("/admin")

	identityModule.RegisterRoutes(public, protected)
	catalogModule.RegisterRoutes(public)
	partnersModule.RegisterRoutes(public)
	searchModule.RegisterRoutes(public)
	barcodeModule.RegisterRoutes(public)
	cartModule.RegisterRoutes(protected)
	imagesModule.RegisterRoutes(admin)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── gRPC server ───────────────────────────────────────────────────────────

	grpcSrv := grpcserver.New(log)

	grpcLis, err := grpcserver.Listen(cfg.GRPC.Port)
	if err != nil {
		log.Error("failed to create grpc listener", "error", err)
		os.Exit(1)
	}

	cartModule.RegisterGRPC(grpcSrv)
	partnersModule.RegisterGRPC(grpcSrv)
	catalogModule.RegisterGRPC(grpcSrv)

	// ── Start ─────────────────────────────────────────────────────────────────

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("HTTP server starting", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		log.Info("gRPC server starting", "port", cfg.GRPC.Port)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		log.Info("shutting down servers...")

		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()

		grpcSrv.GracefulStop()
		if err := httpServer.Shutdown(shutCtx); err != nil {
			log.Warn("http server shutdown error", "error", err)
		}

		log.Info("shutdown complete")
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
