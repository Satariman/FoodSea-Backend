//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	_ "github.com/foodsea/optimization/ent/runtime"
	analogs "github.com/foodsea/optimization/internal/modules/analogs"
	optimizer "github.com/foodsea/optimization/internal/modules/optimizer"
	"github.com/foodsea/optimization/internal/platform/config"
	"github.com/foodsea/optimization/internal/platform/database"
	"github.com/foodsea/optimization/internal/platform/grpcserver"
	"github.com/foodsea/optimization/internal/platform/kafka"
	"github.com/foodsea/optimization/internal/platform/middleware"
	pbcore "github.com/foodsea/proto/core"
	pbml "github.com/foodsea/proto/ml"
	pbopt "github.com/foodsea/proto/optimization"
)

const (
	bufSize       = 1024 * 1024
	testJWTSecret = "test-secret-min-32-chars-long-abc"
)

var (
	httpClient  = &http.Client{Timeout: 30 * time.Second}
	testBaseURL string

	optGRPCConn   *grpc.ClientConn
	optGRPCClient pbopt.OptimizationServiceClient

	testUserID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherUserID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

var (
	productMilk      = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1")
	productBread     = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2")
	productCheeseRU  = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3")
	productCheeseKOS = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa4")

	storeA = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1")
	storeB = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
)

type mockCoreService struct {
	pbcore.UnimplementedCartServiceServer
	pbcore.UnimplementedOfferServiceServer
}

func (m *mockCoreService) GetCartItems(context.Context, *pbcore.GetCartItemsRequest) (*pbcore.GetCartItemsResponse, error) {
	return &pbcore.GetCartItemsResponse{
		Items: []*pbcore.CartItemProto{
			{ProductId: productMilk.String(), ProductName: "Молоко 3.2%", Quantity: 2},
			{ProductId: productBread.String(), ProductName: "Хлеб белый", Quantity: 1},
			{ProductId: productCheeseRU.String(), ProductName: "Сыр Российский", Quantity: 1},
			{ProductId: productCheeseKOS.String(), ProductName: "Сыр Костромской", Quantity: 1},
		},
	}, nil
}

func (m *mockCoreService) GetOffers(_ context.Context, req *pbcore.GetOffersRequest) (*pbcore.GetOffersResponse, error) {
	offersByProduct := map[string][]*pbcore.OfferProto{
		productMilk.String(): {
			{ProductId: productMilk.String(), StoreId: storeA.String(), StoreName: "Store A", PriceKopecks: 12000, InStock: true},
			{ProductId: productMilk.String(), StoreId: storeB.String(), StoreName: "Store B", PriceKopecks: 11000, InStock: true},
		},
		productBread.String(): {
			{ProductId: productBread.String(), StoreId: storeA.String(), StoreName: "Store A", PriceKopecks: 5000, InStock: true},
			{ProductId: productBread.String(), StoreId: storeB.String(), StoreName: "Store B", PriceKopecks: 5500, InStock: true},
		},
		productCheeseRU.String(): {
			{ProductId: productCheeseRU.String(), StoreId: storeA.String(), StoreName: "Store A", PriceKopecks: 35000, InStock: true},
			{ProductId: productCheeseRU.String(), StoreId: storeB.String(), StoreName: "Store B", PriceKopecks: 36000, InStock: true},
		},
		productCheeseKOS.String(): {
			{ProductId: productCheeseKOS.String(), StoreId: storeA.String(), StoreName: "Store A", PriceKopecks: 30000, InStock: true},
			{ProductId: productCheeseKOS.String(), StoreId: storeB.String(), StoreName: "Store B", PriceKopecks: 24000, InStock: true},
		},
	}

	resp := &pbcore.GetOffersResponse{}
	for _, id := range req.GetProductIds() {
		resp.Offers = append(resp.Offers, offersByProduct[id]...)
	}
	return resp, nil
}

func (m *mockCoreService) GetDeliveryConditions(_ context.Context, req *pbcore.GetDeliveryConditionsRequest) (*pbcore.GetDeliveryConditionsResponse, error) {
	conditions := make(map[string]*pbcore.DeliveryConditionProto, 2)
	conditions[storeA.String()] = &pbcore.DeliveryConditionProto{
		StoreId:             storeA.String(),
		MinOrderKopecks:     0,
		DeliveryCostKopecks: 30000,
		FreeFromKopecks:     ptrInt64(100000),
	}
	conditions[storeB.String()] = &pbcore.DeliveryConditionProto{
		StoreId:             storeB.String(),
		MinOrderKopecks:     0,
		DeliveryCostKopecks: 25000,
		FreeFromKopecks:     ptrInt64(80000),
	}

	resp := &pbcore.GetDeliveryConditionsResponse{}
	for _, id := range req.GetStoreIds() {
		if cond, ok := conditions[id]; ok {
			resp.Conditions = append(resp.Conditions, cond)
		}
	}
	return resp, nil
}

type mockAnalogService struct {
	pbml.UnimplementedAnalogServiceServer
}

func (m *mockAnalogService) GetBatchAnalogs(_ context.Context, req *pbml.GetBatchAnalogsRequest) (*pbml.GetBatchAnalogsResponse, error) {
	resp := &pbml.GetBatchAnalogsResponse{AnalogsByProduct: map[string]*pbml.AnalogList{}}
	for _, pid := range req.GetProductIds() {
		if pid == productCheeseRU.String() {
			resp.AnalogsByProduct[pid] = &pbml.AnalogList{
				Analogs: []*pbml.AnalogProto{{
					ProductId:       productCheeseKOS.String(),
					ProductName:     "Сыр Костромской",
					Score:           0.92,
					MinPriceKopecks: 24000,
				}},
			}
		}
	}
	return resp, nil
}

func (m *mockAnalogService) GetAnalogs(_ context.Context, req *pbml.GetAnalogsRequest) (*pbml.GetAnalogsResponse, error) {
	if req.GetProductId() == productMilk.String() {
		return &pbml.GetAnalogsResponse{Analogs: []*pbml.AnalogProto{{
			ProductId:       productBread.String(),
			ProductName:     "Молочный напиток",
			Score:           0.81,
			MinPriceKopecks: 10000,
		}}}, nil
	}
	return &pbml.GetAnalogsResponse{Analogs: []*pbml.AnalogProto{}}, nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Exit(run(ctx, m))
}

//nolint:cyclop,funlen
func run(ctx context.Context, m *testing.M) int {
	gin.SetMode(gin.TestMode)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("optimization_db"),
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

	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redpandadata/redpanda:v23.3.21",
			ExposedPorts: []string{"9092/tcp"},
			Cmd: []string{
				"redpanda", "start",
				"--overprovisioned",
				"--smp=1",
				"--default-log-level=warn",
				"--kafka-addr=0.0.0.0:9092",
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

	kafkaHost, err := kafkaContainer.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kafka host: %v\n", err)
		return 1
	}
	kafkaMappedPort, err := kafkaContainer.MappedPort(ctx, "9092/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "kafka mapped port: %v\n", err)
		return 1
	}
	testKafkaBroker := fmt.Sprintf("%s:%s", kafkaHost, kafkaMappedPort.Port())

	coreMock := &mockCoreService{}
	mlMock := &mockAnalogService{}

	coreLis := bufconn.Listen(bufSize)
	mlLis := bufconn.Listen(bufSize)

	coreSrv := grpc.NewServer()
	mlSrv := grpc.NewServer()
	pbcore.RegisterCartServiceServer(coreSrv, coreMock)
	pbcore.RegisterOfferServiceServer(coreSrv, coreMock)
	pbml.RegisterAnalogServiceServer(mlSrv, mlMock)

	go coreSrv.Serve(coreLis) //nolint:errcheck
	go mlSrv.Serve(mlLis)     //nolint:errcheck
	defer coreSrv.GracefulStop()
	defer mlSrv.GracefulStop()

	bufDialer := func(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
		return func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	}

	coreConn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(bufDialer(coreLis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial core bufconn: %v\n", err)
		return 1
	}
	defer coreConn.Close()

	mlConn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(bufDialer(mlLis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial ml bufconn: %v\n", err)
		return 1
	}
	defer mlConn.Close()

	entClient, _, err := database.Open(ctx, config.DatabaseConfig{
		URL:             dbDSN,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open DB: %v\n", err)
		return 1
	}
	defer entClient.Close()

	if err := entClient.Schema.Create(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create schema: %v\n", err)
		return 1
	}

	producer := kafka.NewProducer([]string{testKafkaBroker}, "optimization.events", log)
	defer producer.Close()

	analogsModule := analogs.NewModule(analogs.Deps{
		MLClient: pbml.NewAnalogServiceClient(mlConn),
		Cache:    nil,
		Log:      log,
	})
	optimizerModule := optimizer.NewModule(&optimizer.Deps{
		Ent:                  entClient,
		CartClient:           pbcore.NewCartServiceClient(coreConn),
		OfferClient:          pbcore.NewOfferServiceClient(coreConn),
		AnalogProvider:       analogsModule.Provider,
		GetAnalogsForProduct: analogsModule.GetAnalogsForProduct,
		Producer:             producer,
		Cache:                nil,
		Timeout:              10 * time.Second,
		Log:                  log,
	})

	router := gin.New()
	router.Use(middleware.Recovery(log), middleware.RequestID())
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })

	v1 := router.Group("/api/v1")
	v1.Use(middleware.Auth(testJWTSecret))
	optimizerModule.RegisterRoutes(v1)

	httpSrv := httptest.NewServer(router)
	defer httpSrv.Close()
	testBaseURL = httpSrv.URL

	grpcSrv := grpcserver.New(log)
	optimizerModule.RegisterGRPC(grpcSrv)
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc listen: %v\n", err)
		return 1
	}
	go grpcSrv.Serve(grpcLis) //nolint:errcheck
	defer grpcSrv.GracefulStop()

	optGRPCConn, err = grpc.NewClient(grpcLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc client dial optimization: %v\n", err)
		return 1
	}
	defer optGRPCConn.Close()
	optGRPCClient = pbopt.NewOptimizationServiceClient(optGRPCConn)

	return m.Run()
}

func authHeader(userID uuid.UUID) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
	})
	signed, _ := token.SignedString([]byte(testJWTSecret))
	return "Bearer " + signed
}

func doJSONRequest(t *testing.T, method, path, auth string) (*http.Response, []byte) {
	t.Helper()

	req, err := http.NewRequest(method, testBaseURL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, body
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

type optimizationResultResponse struct {
	ID           string `json:"id"`
	TotalKopecks int64  `json:"total_kopecks"`
	Status       string `json:"status"`
	Items        []struct {
		ProductID string `json:"product_id"`
	} `json:"items"`
	Substitutions []struct {
		OriginalProductName string `json:"original_product_name"`
		AnalogProductName   string `json:"analog_product_name"`
	} `json:"substitutions"`
}

type analogsResponse struct {
	Analogs []struct {
		Score           float64 `json:"score"`
		MinPriceKopecks int64   `json:"min_price_kopecks"`
	} `json:"analogs"`
}

func decodeEnvelope[T any](t *testing.T, body []byte, out *T) {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if len(env.Data) == 0 {
		t.Fatalf("empty response data, error=%q body=%s", env.Error, string(body))
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("unmarshal data: %v; body=%s", err, string(body))
	}
}

func postOptimize(t *testing.T, userID uuid.UUID) optimizationResultResponse {
	t.Helper()
	resp, body := doJSONRequest(t, http.MethodPost, "/api/v1/optimize", authHeader(userID))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /optimize: status=%d body=%s", resp.StatusCode, string(body))
	}
	var result optimizationResultResponse
	decodeEnvelope(t, body, &result)
	return result
}

func ptrInt64(v int64) *int64 { return &v }
