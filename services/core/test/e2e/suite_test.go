//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dockercontainer "github.com/moby/moby/api/types/container"
	dockernetwork "github.com/moby/moby/api/types/network"
	skafka "github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/foodsea/core/ent"
	_ "github.com/foodsea/core/ent/runtime"
	"github.com/foodsea/core/internal/modules/barcode"
	"github.com/foodsea/core/internal/modules/cart"
	"github.com/foodsea/core/internal/modules/catalog"
	"github.com/foodsea/core/internal/modules/identity"
	"github.com/foodsea/core/internal/modules/partners"
	"github.com/foodsea/core/internal/modules/search"
	"github.com/foodsea/core/internal/platform/cache"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	"github.com/foodsea/core/internal/platform/grpcserver"
	"github.com/foodsea/core/internal/platform/kafka"
	"github.com/foodsea/core/internal/platform/middleware"
)

var (
	httpClient      = &http.Client{Timeout: 10 * time.Second}
	testBaseURL     string
	testGRPCAddr    string
	testKafkaBroker string

	seededProductID      string
	seededProductBarcode = "4607025390015"
	seededStoreID        string
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Exit(run(ctx, m))
}

//nolint:cyclop,funlen
func run(ctx context.Context, m *testing.M) int {
	gin.SetMode(gin.TestMode)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// ── Postgres ──────────────────────────────────────────────────────────────
	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_db"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		return 1
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	dbDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres DSN: %v\n", err)
		return 1
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start redis: %v\n", err)
		return 1
	}
	defer redisContainer.Terminate(ctx) //nolint:errcheck

	redisHost, _ := redisContainer.Host(ctx)
	redisPort, _ := redisContainer.MappedPort(ctx, "6379")
	redisURL := fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort.Port())

	// ── Kafka (Redpanda) ──────────────────────────────────────────────────────
	// Pre-allocate a free port so Redpanda can advertise it before container start.
	kafkaPort, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find free port: %v\n", err)
		return 1
	}
	testKafkaBroker = fmt.Sprintf("127.0.0.1:%d", kafkaPort)

	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redpandadata/redpanda:v23.3.21",
			ExposedPorts: []string{"9092/tcp"},
			Cmd: []string{
				"redpanda", "start",
				"--overprovisioned",
				"--smp=1",
				"--default-log-level=warn",
				"--set", "redpanda.auto_create_topics_enabled=true",
				"--kafka-addr=0.0.0.0:9092",
				fmt.Sprintf("--advertise-kafka-addr=127.0.0.1:%d", kafkaPort),
			},
			HostConfigModifier: func(hc *dockercontainer.HostConfig) {
				hc.PortBindings = dockernetwork.PortMap{
					dockernetwork.MustParsePort("9092/tcp"): {
						{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: strconv.Itoa(kafkaPort)},
					},
				}
			},
			WaitingFor: wait.ForListeningPort("9092/tcp").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start kafka: %v\n", err)
		return 1
	}
	defer kafkaContainer.Terminate(ctx) //nolint:errcheck
	if err := createKafkaTopics(ctx, testKafkaBroker, "cart.events"); err != nil {
		fmt.Fprintf(os.Stderr, "create kafka topics: %v\n", err)
		return 1
	}

	// ── Wire dependencies ─────────────────────────────────────────────────────
	dbCfg := config.DatabaseConfig{
		URL:             dbDSN,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}
	jwtCfg := config.JWTConfig{
		Secret:     "test-secret-min-32-chars-long-abc",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}

	entClient, sqlDB, err := database.Open(ctx, dbCfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open DB: %v\n", err)
		return 1
	}
	defer entClient.Close()

	redisCache, err := cache.NewRedis(redisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect redis: %v\n", err)
		return 1
	}
	defer redisCache.Close()

	cartProducer := kafka.NewProducer([]string{testKafkaBroker}, "cart.events", log)
	defer cartProducer.Close()

	// Apply schema (tests only; production uses Atlas migrations).
	if err := entClient.Schema.Create(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create schema: %v\n", err)
		return 1
	}

	// ── Build modules ─────────────────────────────────────────────────────────
	identityMod := identity.NewModule(identity.Deps{
		Ent:   entClient,
		Redis: redisCache.Client(),
		Cache: redisCache,
		Log:   log,
		JWT:   jwtCfg,
	})
	catalogMod := catalog.NewModule(catalog.Deps{
		Ent:   entClient,
		Cache: redisCache,
		Log:   log,
	})
	partnersMod := partners.NewModule(partners.Deps{
		Ent:   entClient,
		Cache: redisCache,
		Log:   log,
	})
	cartMod := cart.NewModule(cart.Deps{
		Ent:      entClient,
		Producer: cartProducer,
		Log:      log,
	})
	searchMod := search.NewModule(search.Deps{
		Ent:   entClient,
		DB:    sqlDB,
		Cache: redisCache,
		Log:   log,
	})
	barcodeMod := barcode.NewModule(barcode.Deps{
		ProductGetter: catalogMod.ProductGetter(),
		Log:           log,
	})

	// ── HTTP router ───────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(middleware.Recovery(log), middleware.RequestID())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/api/v1")
	public := v1
	protected := v1.Group("", middleware.Auth(jwtCfg.Secret))

	identityMod.RegisterRoutes(public, protected)
	catalogMod.RegisterRoutes(public)
	partnersMod.RegisterRoutes(public)
	searchMod.RegisterRoutes(public)
	barcodeMod.RegisterRoutes(public)
	cartMod.RegisterRoutes(protected)

	srv := httptest.NewServer(router)
	defer srv.Close()
	testBaseURL = srv.URL

	// ── gRPC test server ──────────────────────────────────────────────────────
	grpcSrv := grpcserver.New(log)
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc listen: %v\n", err)
		return 1
	}
	cartMod.RegisterGRPC(grpcSrv)
	partnersMod.RegisterGRPC(grpcSrv)
	go grpcSrv.Serve(grpcLis) //nolint:errcheck
	defer grpcSrv.GracefulStop()
	testGRPCAddr = grpcLis.Addr().String()

	// ── Seed ─────────────────────────────────────────────────────────────────
	if err := seed(ctx, entClient); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		return 1
	}

	return m.Run()
}

func seed(ctx context.Context, client *ent.Client) error {
	now := time.Now()

	cat, err := client.Category.Create().
		SetName("Молоко и яйца").
		SetSlug("moloko-i-yaytsa").
		SetSortOrder(0).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed category: %w", err)
	}

	brand, err := client.Brand.Create().
		SetName("ВкусВилл").
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed brand: %w", err)
	}

	product, err := client.Product.Create().
		SetName("Молоко 2.5%").
		SetBarcode(seededProductBarcode).
		SetInStock(true).
		SetCategoryID(cat.ID).
		SetBrandID(brand.ID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed product: %w", err)
	}
	seededProductID = product.ID.String()

	store, err := client.Store.Create().
		SetName("ВкусВилл").
		SetSlug("vkusvill-store").
		SetIsActive(true).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed store: %w", err)
	}
	seededStoreID = store.ID.String()

	_, err = client.Offer.Create().
		SetProductID(product.ID).
		SetStoreID(store.ID).
		SetPriceKopecks(12900).
		SetInStock(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed offer: %w", err)
	}

	return nil
}

// freePort returns a free TCP port on localhost.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func createKafkaTopics(ctx context.Context, broker string, topics ...string) error {
	conn, err := skafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return fmt.Errorf("dial kafka broker: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get kafka controller: %w", err)
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConn, err := skafka.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		return fmt.Errorf("dial kafka controller %s: %w", controllerAddr, err)
	}
	defer controllerConn.Close()

	cfgs := make([]skafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		cfgs = append(cfgs, skafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	if err := controllerConn.CreateTopics(cfgs...); err != nil {
		return fmt.Errorf("create topics: %w", err)
	}
	return nil
}

// postJSON makes a POST request with a JSON body.
func postJSON(url string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return httpClient.Post(url, "application/json", bytes.NewReader(data))
}

// postJSONAuth makes an authenticated POST request.
func postJSONAuth(url, token string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}

// getAuth makes an authenticated GET request.
func getAuth(url, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}

// putJSONAuth makes an authenticated PUT request.
func putJSONAuth(url, token string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}

// deleteAuth makes an authenticated DELETE request.
func deleteAuth(url, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}

// decodeJSON reads and decodes a JSON response body into dst.
func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}
