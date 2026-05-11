package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Import generated docs (populated after swag init)
	_ "github.com/foodsea/core/docs/swagger"
	// Initialize ent default functions (uuid, timestamps)
	_ "github.com/foodsea/core/ent/runtime"

	"github.com/foodsea/core/internal/modules/barcode"
	"github.com/foodsea/core/internal/modules/cart"
	"github.com/foodsea/core/internal/modules/catalog"
	"github.com/foodsea/core/internal/modules/identity"
	"github.com/foodsea/core/internal/modules/images"
	"github.com/foodsea/core/internal/modules/notifications"
	notificationsapns "github.com/foodsea/core/internal/modules/notifications/apns"
	"github.com/foodsea/core/internal/modules/partners"
	"github.com/foodsea/core/internal/modules/photo_search"
	"github.com/foodsea/core/internal/modules/search"
	"github.com/foodsea/core/internal/modules/voice"
	"github.com/foodsea/core/internal/platform/cache"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	"github.com/foodsea/core/internal/platform/grpcclient"
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

	notificationsAPNSClient, err := notificationsapns.NewClient(cfg.APNS)
	if err != nil {
		if cfg.Env == "production" {
			log.Error("failed to initialise notifications APNS client", "error", err)
			os.Exit(1)
		}
		log.Warn("failed to initialise notifications APNS client, using noop client in non-production",
			"error", err,
			"env", cfg.Env,
		)
		notificationsAPNSClient = notificationsapns.NewNoopClient()
	}

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

	// Prefer explicit voice addr; fallback to generic ML addr; local-safe default.
	mlVoiceAddr := getenv("ML_SERVICE_VOICE_GRPC_ADDR", getenv("ML_GRPC_ADDR", "localhost:50051"))
	mlVoiceConn, err := grpc.NewClient(mlVoiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("failed to dial ml-service voice gRPC", "error", err, "addr", mlVoiceAddr)
		os.Exit(1)
	}
	defer mlVoiceConn.Close()

	mlClients, err := grpcclient.DialML(ctx, cfg.ML.GRPCAddr, log)
	if err != nil {
		log.Error("failed to initialise ML gRPC clients", "error", err, "addr", cfg.ML.GRPCAddr)
		os.Exit(1)
	}
	defer func() {
		if closeErr := mlClients.Close(); closeErr != nil {
			log.Warn("failed to close ML gRPC clients", "error", closeErr)
		}
	}()

	// ── Modules ───────────────────────────────────────────────────────────────

	identityModule := identity.NewModule(identity.Deps{
		Ent:   entClient,
		Redis: redisCache.Client(),
		Cache: redisCache,
		Log:   log,
		JWT:   cfg.JWT,
		OAuth: cfg.OAuth,
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
	voiceModule := voice.NewModule(voice.Deps{
		MLVoiceConn:    mlVoiceConn,
		RequestTimeout: envDurationMs("VOICE_REQUEST_TIMEOUT_MS", 5000),
	})
	photoSearchModule := photo_search.NewModule(photo_search.Deps{
		MLClient:      mlClients.Analog,
		ProductLoader: catalogModule.ProductLoader(),
		MaxImageBytes: cfg.PhotoSearch.MaxImageBytes,
		Timeout:       cfg.PhotoSearch.Timeout,
	})
	notificationsModule := notifications.NewModule(notifications.Deps{
		Ent:          entClient,
		Log:          log,
		KafkaBrokers: cfg.Kafka.Brokers,
		KafkaTopic:   cfg.Notifications.Kafka.Topic,
		KafkaGroupID: cfg.Notifications.Kafka.GroupID,
		APNS:         notificationsAPNSClient,
	})
	defer func() {
		if err := notificationsModule.CloseOrderEventsConsumer(); err != nil {
			log.Warn("failed to close notifications consumer", "error", err)
		}
	}()

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
	voiceModule.RegisterRoutes(protected)
	photoSearchModule.RegisterRoutes(protected)
	notificationsModule.RegisterRoutes(protected)
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
		log.Info("notifications consumer starting",
			"topic", cfg.Notifications.Kafka.Topic,
			"group_id", cfg.Notifications.Kafka.GroupID,
		)
		if err := notificationsModule.RunOrderEventsConsumer(gCtx); err != nil {
			return fmt.Errorf("notifications consumer: %w", err)
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

func getenv(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return fallback
}

func envDurationMs(name string, defaultMs int) time.Duration {
	if v, ok := os.LookupEnv(name); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Duration(defaultMs) * time.Millisecond
}
