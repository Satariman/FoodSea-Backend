//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	dockercontainer "github.com/moby/moby/api/types/container"
	dockernetwork "github.com/moby/moby/api/types/network"
	skafka "github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"log/slog"

	pb_core "github.com/foodsea/proto/core"
	pb_opt "github.com/foodsea/proto/optimization"

	"github.com/foodsea/ordering/ent"
	_ "github.com/foodsea/ordering/ent/runtime"
	"github.com/foodsea/ordering/internal/modules/orders"
	"github.com/foodsea/ordering/internal/modules/saga"
	"github.com/foodsea/ordering/internal/platform/config"
	"github.com/foodsea/ordering/internal/platform/database"
	"github.com/foodsea/ordering/internal/platform/kafka"
	"github.com/foodsea/ordering/internal/platform/middleware"
)

const (
	bufSize       = 1024 * 1024
	testJWTSecret = "test-secret-min-32-chars-long-abc"
)

var (
	httpClient      = &http.Client{Timeout: 90 * time.Second}
	testBaseURL     string
	testKafkaBroker string
	testLog         *slog.Logger
	testEntClient   *ent.Client
	testCartConn    *grpc.ClientConn
	testOptConn     *grpc.ClientConn
	testSagaModule  *saga.Module

	cartMock *mockCartServer
	optMock  *mockOptServer
)

// mockCartServer is an in-memory CartServiceServer for testing.
type mockCartServer struct {
	pb_core.UnimplementedCartServiceServer
	mu               sync.Mutex
	clearCartErr     error
	restoreCartErr   error
	clearCartCalls   int
	restoreCartCalls int
}

func (m *mockCartServer) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCartErr = nil
	m.restoreCartErr = nil
	m.clearCartCalls = 0
	m.restoreCartCalls = 0
}

func (m *mockCartServer) setClearCartErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCartErr = err
}

func (m *mockCartServer) ClearCart(_ context.Context, _ *pb_core.ClearCartRequest) (*pb_core.ClearCartResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCartCalls++
	if m.clearCartErr != nil {
		return nil, m.clearCartErr
	}
	return &pb_core.ClearCartResponse{Success: true}, nil
}

func (m *mockCartServer) RestoreCart(_ context.Context, _ *pb_core.RestoreCartRequest) (*pb_core.RestoreCartResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreCartCalls++
	if m.restoreCartErr != nil {
		return nil, m.restoreCartErr
	}
	return &pb_core.RestoreCartResponse{Success: true}, nil
}

func (m *mockCartServer) GetCartItems(_ context.Context, _ *pb_core.GetCartItemsRequest) (*pb_core.GetCartItemsResponse, error) {
	return &pb_core.GetCartItemsResponse{}, nil
}

// mockOptServer is an in-memory OptimizationServiceServer for testing.
type mockOptServer struct {
	pb_opt.UnimplementedOptimizationServiceServer
	mu             sync.Mutex
	results        map[string]*pb_opt.OptimizationResultProto
	lockErr        error
	unlockCalls    int
	lockCalls      int
	getResultCalls int
}

func newMockOptServer() *mockOptServer {
	return &mockOptServer{results: make(map[string]*pb_opt.OptimizationResultProto)}
}

func (m *mockOptServer) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = make(map[string]*pb_opt.OptimizationResultProto)
	m.lockErr = nil
	m.unlockCalls = 0
	m.lockCalls = 0
	m.getResultCalls = 0
}

func (m *mockOptServer) addResult(r *pb_opt.OptimizationResultProto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[r.Id] = r
}

func (m *mockOptServer) GetResult(_ context.Context, req *pb_opt.GetResultRequest) (*pb_opt.GetResultResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getResultCalls++
	r, ok := m.results[req.ResultId]
	if !ok {
		return nil, fmt.Errorf("result not found")
	}
	return &pb_opt.GetResultResponse{Result: r}, nil
}

func (m *mockOptServer) LockResult(_ context.Context, req *pb_opt.LockResultRequest) (*pb_opt.LockResultResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockCalls++
	if m.lockErr != nil {
		return nil, m.lockErr
	}
	if r, ok := m.results[req.ResultId]; ok {
		r.Status = "locked"
	}
	return &pb_opt.LockResultResponse{Success: true}, nil
}

func (m *mockOptServer) UnlockResult(_ context.Context, req *pb_opt.UnlockResultRequest) (*pb_opt.UnlockResultResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unlockCalls++
	if r, ok := m.results[req.ResultId]; ok {
		r.Status = "active"
	}
	return &pb_opt.UnlockResultResponse{Success: true}, nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Exit(run(ctx, m))
}

//nolint:cyclop,funlen
func run(ctx context.Context, m *testing.M) int {
	gin.SetMode(gin.TestMode)
	testLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// ── Postgres ──────────────────────────────────────────────────────────────
	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("ordering_db"),
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

	// ── Kafka (Redpanda) ──────────────────────────────────────────────────────
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
	if err := createKafkaTopics(ctx, testKafkaBroker, "order.events", "saga.commands", "saga.replies"); err != nil {
		fmt.Fprintf(os.Stderr, "create kafka topics: %v\n", err)
		return 1
	}

	// ── gRPC mock servers via bufconn ──────────────────────────────────────────
	cartMock = &mockCartServer{}
	optMock = newMockOptServer()

	cartLis := bufconn.Listen(bufSize)
	optLis := bufconn.Listen(bufSize)

	cartSrv := grpc.NewServer()
	optSrv := grpc.NewServer()
	pb_core.RegisterCartServiceServer(cartSrv, cartMock)
	pb_opt.RegisterOptimizationServiceServer(optSrv, optMock)

	go cartSrv.Serve(cartLis) //nolint:errcheck
	go optSrv.Serve(optLis)   //nolint:errcheck
	defer cartSrv.GracefulStop()
	defer optSrv.GracefulStop()

	bufDialer := func(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
		return func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}
	}

	testCartConn, err = grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(bufDialer(cartLis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial cart bufconn: %v\n", err)
		return 1
	}
	defer testCartConn.Close()

	testOptConn, err = grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(bufDialer(optLis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial opt bufconn: %v\n", err)
		return 1
	}
	defer testOptConn.Close()

	// ── Wire dependencies ─────────────────────────────────────────────────────
	dbCfg := config.DatabaseConfig{
		URL:             dbDSN,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}

	testEntClient, _, err = database.Open(ctx, dbCfg, testLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open DB: %v\n", err)
		return 1
	}
	defer testEntClient.Close()

	if err := testEntClient.Schema.Create(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create schema: %v\n", err)
		return 1
	}

	brokers := []string{testKafkaBroker}
	orderProducer := kafka.NewProducer(brokers, "order.events", testLog)
	defer orderProducer.Close()

	sagaCmdProducer := kafka.NewProducer(brokers, "saga.commands", testLog)
	defer sagaCmdProducer.Close()

	sagaReplyProducer := kafka.NewProducer(brokers, "saga.replies", testLog)
	defer sagaReplyProducer.Close()

	sagaReplyConsumer := kafka.NewConsumer(brokers, "saga.replies", "ordering-saga-e2e", testLog)
	defer sagaReplyConsumer.Close()

	ordersModule := orders.NewModule(orders.Deps{
		Ent:      testEntClient,
		Producer: orderProducer,
		Log:      testLog,
	})

	testSagaModule = saga.NewModule(saga.Deps{
		Ent:             testEntClient,
		OrdersFacade:    ordersModule.OrderFacade(),
		CartClient:      pb_core.NewCartServiceClient(testCartConn),
		OptClient:       pb_opt.NewOptimizationServiceClient(testOptConn),
		CommandProducer: sagaCmdProducer,
		ReplyProducer:   sagaReplyProducer,
		ReplyConsumer:   sagaReplyConsumer,
		Log:             testLog,
		StepTimeout:     10 * time.Second,
		MaxCompAttempts: 3,
	})

	// ── HTTP router ───────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(middleware.Recovery(testLog), middleware.RequestID())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	protected := router.Group("/api/v1", middleware.Auth(testJWTSecret))
	ordersModule.RegisterRoutes(protected)
	testSagaModule.RegisterRoutes(protected)

	srv := httptest.NewServer(router)
	defer srv.Close()
	testBaseURL = srv.URL

	return m.Run()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func makeJWT(userID uuid.UUID) string {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	return tok
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

func getAuth(url, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return httpClient.Do(req)
}

func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// buildOptResult creates a test OptimizationResultProto for a given user.
func buildOptResult(userID uuid.UUID) *pb_opt.OptimizationResultProto {
	return &pb_opt.OptimizationResultProto{
		Id:              uuid.New().String(),
		UserId:          userID.String(),
		TotalKopecks:    300_000,
		DeliveryKopecks: 15_000,
		Status:          "active",
		Items: []*pb_opt.OptimizationItemProto{
			{ProductId: uuid.New().String(), StoreId: uuid.New().String(), StoreName: "Магазин А", Quantity: 2, PriceKopecks: 100_000},
			{ProductId: uuid.New().String(), StoreId: uuid.New().String(), StoreName: "Магазин Б", Quantity: 1, PriceKopecks: 200_000},
		},
	}
}
