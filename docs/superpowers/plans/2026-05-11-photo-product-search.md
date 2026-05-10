# Photo Product Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build protected backend product search by photo: iOS sends a product image plus OCR text to core-service, core calls ml-service, ml-service searches a persisted Google-embedding index, and core returns ranked catalog candidates.

**Architecture:** Public HTTP lives in `core-service` as a protected catalog discovery endpoint. `ml-service` owns OCR parsing, embedding providers, index persistence, rebuild command, and vector search. Catalog embeddings are rebuilt only by an explicit command and loaded between service restarts.

**Tech Stack:** Go 1.25, Gin, gRPC/protobuf, Ent, Swagger/swaggo, Python 3.11, grpcio, numpy, scikit-learn, Google Gemini/Vertex embedding providers behind fakes for CI.

---

## File Structure

Create:

- `services/core/internal/platform/grpcclient/client.go`: core outbound gRPC dialer for `ml.AnalogService`.
- `services/core/internal/platform/grpcclient/client_test.go`: bufconn-style or dial-option tests for the dialer.
- `services/core/internal/modules/photo_search/architecture-notes.md`: module boundary notes.
- `services/core/internal/modules/photo_search/domain/photo_search.go`: core photo-search domain DTOs and interfaces.
- `services/core/internal/modules/photo_search/usecase/search_by_photo.go`: validation, ML call, product loading, stale-ID filtering.
- `services/core/internal/modules/photo_search/usecase/search_by_photo_test.go`: usecase tests.
- `services/core/internal/modules/photo_search/grpc/ml_client.go`: adapter over generated `pbml.AnalogServiceClient`.
- `services/core/internal/modules/photo_search/grpc/ml_client_test.go`: gRPC adapter tests.
- `services/core/internal/modules/photo_search/handler/dto.go`: HTTP response DTOs.
- `services/core/internal/modules/photo_search/handler/handler.go`: multipart HTTP handler.
- `services/core/internal/modules/photo_search/handler/handler_test.go`: handler tests.
- `services/core/internal/modules/photo_search/module.go`: DI container and protected route registration.
- `services/ml/src/photo_search/__init__.py`: package marker.
- `services/ml/src/photo_search/parser.py`: deterministic OCR brand/name parser.
- `services/ml/src/photo_search/index.py`: persisted photo-search KNN index.
- `services/ml/src/photo_search/embeddings.py`: provider interfaces, fake provider, Gemini API key provider, Vertex provider stub.
- `services/ml/src/photo_search/rebuild_index.py`: explicit rebuild CLI.
- `services/ml/src/photo_search/service.py`: `SearchByPhoto` orchestration.
- `services/ml/tests/test_photo_parser.py`: OCR parser tests.
- `services/ml/tests/test_photo_index.py`: index persistence/query tests.
- `services/ml/tests/test_photo_embeddings.py`: provider configuration and request-shape tests without network.
- `services/ml/tests/test_photo_service.py`: gRPC/service-level photo search tests.
- `services/ml/tests/test_photo_rebuild.py`: rebuild command tests with fakes.

Modify:

- `proto/ml/analogs.proto`: add `SearchByPhoto` RPC and messages.
- `proto/core/catalog.proto`: add human-readable catalog fields for ML.
- `services/core/internal/platform/config/config.go`: add ML/photo-search config.
- `services/core/internal/platform/config/config_test.go`: test new config defaults/custom validation.
- `services/core/internal/shared/errors/errors.go`: add sentinel `ErrUnavailable` for 503 mapping.
- `services/core/internal/platform/httputil/response.go`: map `ErrUnavailable` to HTTP 503.
- `services/core/internal/modules/catalog/domain/product.go`: extend `ProductMLData`.
- `services/core/internal/modules/catalog/repository/product_repo.go`: load category/brand names and image URL for ML export.
- `services/core/internal/modules/catalog/grpc/catalog_server.go`: include new proto fields.
- `services/core/internal/modules/catalog/grpc/catalog_server_test.go`: assert new fields.
- `services/core/internal/modules/catalog/module.go`: export `ProductLoader()` backed by `GetProduct`.
- `services/core/cmd/api/main.go`: dial ML, wire `photo_search`, close ML connection.
- `services/core/cmd/api/architecture-notes.md`: document outbound ML gRPC client and route.
- `services/core/internal/platform/architecture-notes.md`: document new config and outbound gRPC client.
- `services/ml/src/config.py`: add photo-search env values.
- `services/ml/src/data_loader.py`: map new catalog fields into `ProductData`.
- `services/ml/src/service.py`: expose `SearchByPhoto` on existing `AnalogServicer`.
- `services/ml/src/main.py`: load optional photo index and provider.
- `services/ml/Makefile`: add `rebuild-photo-index`.
- `services/ml/architecture-notes.md`: document photo-search index, command, providers, privacy behavior.
- `Makefile`: add root target `rebuild-photo-search-index`.
- `AGENTS.md`: update command/env sections after implementation.

Generated:

- `proto/ml/analogs.pb.go`
- `proto/ml/analogs_grpc.pb.go`
- `proto/core/catalog.pb.go`
- `proto/core/catalog_grpc.pb.go`
- `services/ml/src/proto/analogs_pb2.py`
- `services/ml/src/proto/analogs_pb2_grpc.py`
- `services/ml/src/proto/catalog_pb2.py`
- `services/ml/src/proto/catalog_pb2_grpc.py`
- `services/core/docs/swagger/docs.go`
- `services/core/docs/swagger/swagger.json`
- `services/core/docs/swagger/swagger.yaml`

---

### Task 1: Extend Proto Contracts

**Files:**
- Modify: `proto/ml/analogs.proto`
- Modify: `proto/core/catalog.proto`
- Generated by command: `proto/ml/*.pb.go`, `proto/core/catalog*.pb.go`, `services/ml/src/proto/*.py`

- [ ] **Step 1: Add failing contract checks by grepping generated APIs before regeneration**

Run:

```bash
rg -n "SearchByPhoto|BrandName|CategoryName|ImageUrl" proto services/ml/src/proto
```

Expected: no `SearchByPhoto` generated APIs yet.

- [ ] **Step 2: Modify `proto/ml/analogs.proto`**

Add the RPC and messages without changing existing field numbers:

```proto
service AnalogService {
    rpc GetAnalogs(GetAnalogsRequest) returns (GetAnalogsResponse);
    rpc GetBatchAnalogs(GetBatchAnalogsRequest) returns (GetBatchAnalogsResponse);
    rpc SearchByPhoto(SearchByPhotoRequest) returns (SearchByPhotoResponse);
}

message SearchByPhotoRequest {
    bytes image = 1;
    string image_mime_type = 2;
    string ocr_text = 3;
    int32 top_k = 4;
}

message SearchByPhotoResponse {
    string matched_name = 1;
    string matched_brand = 2;
    repeated PhotoSearchCandidate candidates = 3;
}

message PhotoSearchCandidate {
    string product_id = 1;
    double score = 2;
}
```

- [ ] **Step 3: Modify `proto/core/catalog.proto`**

Append fields to `ProductFeaturesProto`:

```proto
message ProductFeaturesProto {
    string product_id = 1;
    string name = 2;
    string description = 3;
    string composition = 4;
    string category_id = 5;
    string subcategory_id = 6;
    string brand_id = 7;
    string weight = 8;
    double calories = 9;
    double protein = 10;
    double fat = 11;
    double carbohydrates = 12;
    repeated ProductOfferBrief offers = 13;
    string brand_name = 14;
    string category_name = 15;
    string subcategory_name = 16;
    string image_url = 17;
}
```

- [ ] **Step 4: Regenerate Go protobufs**

Run:

```bash
make proto
```

Expected: generated Go files update successfully.

- [ ] **Step 5: Regenerate Python protobufs**

Run:

```bash
cd services/ml && make proto
```

Expected: generated Python files update successfully and relative imports are patched by the existing Makefile command.

- [ ] **Step 6: Verify generated APIs exist**

Run:

```bash
rg -n "SearchByPhoto|PhotoSearchCandidate|BrandName|CategoryName|ImageUrl" proto services/ml/src/proto
```

Expected: generated Go and Python code contains the new RPC/messages/fields.

- [ ] **Step 7: Commit proto contract changes**

Run:

```bash
git add proto/ml/analogs.proto proto/core/catalog.proto proto/ml/analogs.pb.go proto/ml/analogs_grpc.pb.go proto/core/catalog.pb.go proto/core/catalog_grpc.pb.go services/ml/src/proto/analogs_pb2.py services/ml/src/proto/analogs_pb2_grpc.py services/ml/src/proto/catalog_pb2.py services/ml/src/proto/catalog_pb2_grpc.py
git commit -m "feat: extend ml proto for photo search"
```

---

### Task 2: Add Core Config, 503 Sentinel, And ML gRPC Dialer

**Files:**
- Modify: `services/core/internal/platform/config/config.go`
- Modify: `services/core/internal/platform/config/config_test.go`
- Modify: `services/core/internal/shared/errors/errors.go`
- Modify: `services/core/internal/platform/httputil/response.go`
- Create: `services/core/internal/platform/grpcclient/client.go`
- Create: `services/core/internal/platform/grpcclient/client_test.go`

- [ ] **Step 1: Write config tests**

Add to `services/core/internal/platform/config/config_test.go`:

```go
func TestLoad_PhotoSearchDefaults(t *testing.T) {
	unsetenv(t, "ML_GRPC_ADDR")
	unsetenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES")
	unsetenv(t, "PHOTO_SEARCH_TIMEOUT")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "ml-service:50051", cfg.ML.GRPCAddr)
	assert.Equal(t, int64(8*1024*1024), cfg.PhotoSearch.MaxImageBytes)
	assert.Equal(t, 10*time.Second, cfg.PhotoSearch.Timeout)
}

func TestLoad_PhotoSearchCustomValues(t *testing.T) {
	setenv(t, "ML_GRPC_ADDR", "127.0.0.1:50052")
	setenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES", "1048576")
	setenv(t, "PHOTO_SEARCH_TIMEOUT", "3s")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:50052", cfg.ML.GRPCAddr)
	assert.Equal(t, int64(1048576), cfg.PhotoSearch.MaxImageBytes)
	assert.Equal(t, 3*time.Second, cfg.PhotoSearch.Timeout)
}

func TestLoad_InvalidPhotoSearchConfig(t *testing.T) {
	setenv(t, "PHOTO_SEARCH_MAX_IMAGE_BYTES", "0")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PHOTO_SEARCH_MAX_IMAGE_BYTES")
}
```

- [ ] **Step 2: Run config tests and verify failure**

Run:

```bash
cd services/core && go test ./internal/platform/config -run 'PhotoSearch' -count=1
```

Expected: fail because `Config.ML` and `Config.PhotoSearch` do not exist yet.

- [ ] **Step 3: Implement config structs and validation**

Add to `services/core/internal/platform/config/config.go`:

```go
type Config struct {
	Env         string
	Server      ServerConfig
	GRPC        GRPCConfig
	ML          MLConfig
	PhotoSearch PhotoSearchConfig
	DB          DatabaseConfig
	Redis       RedisConfig
	Kafka       KafkaConfig
	JWT         JWTConfig
	S3          S3Config
	OAuth       OAuthConfig
}

type MLConfig struct {
	GRPCAddr string
}

type PhotoSearchConfig struct {
	MaxImageBytes int64
	Timeout       time.Duration
}
```

Inside `Load()` after port validation:

```go
photoSearchMaxBytes := int64(getEnvInt("PHOTO_SEARCH_MAX_IMAGE_BYTES", 8*1024*1024))
if photoSearchMaxBytes < 1 {
	return nil, fmt.Errorf("PHOTO_SEARCH_MAX_IMAGE_BYTES must be > 0")
}

photoSearchTimeout := getEnvDuration("PHOTO_SEARCH_TIMEOUT", 10*time.Second)
if photoSearchTimeout <= 0 {
	return nil, fmt.Errorf("PHOTO_SEARCH_TIMEOUT must be > 0")
}
```

Inside the `Config` literal:

```go
ML: MLConfig{
	GRPCAddr: getEnv("ML_GRPC_ADDR", "ml-service:50051"),
},
PhotoSearch: PhotoSearchConfig{
	MaxImageBytes: photoSearchMaxBytes,
	Timeout:       photoSearchTimeout,
},
```

- [ ] **Step 4: Add unavailable sentinel and HTTP mapping**

Modify `services/core/internal/shared/errors/errors.go`:

```go
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrConflict      = errors.New("conflict")
	ErrUnavailable   = errors.New("unavailable")
)
```

Modify `services/core/internal/platform/httputil/response.go` inside `HandleError`:

```go
case errors.Is(err, sherrors.ErrUnavailable):
	c.JSON(http.StatusServiceUnavailable, Response{Error: err.Error()})
```

- [ ] **Step 5: Create outbound ML gRPC dialer**

Create `services/core/internal/platform/grpcclient/client.go`:

```go
package grpcclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	pbml "github.com/foodsea/proto/ml"
)

type ClientSet struct {
	Analog  pbml.AnalogServiceClient
	closers []io.Closer
}

type connCloser struct{ conn *grpc.ClientConn }

func (c *connCloser) Close() error { return c.conn.Close() }

func DialML(ctx context.Context, addr string, log *slog.Logger, extraOpts ...grpc.DialOption) (*ClientSet, error) {
	opts := buildDialOptions(log, extraOpts...)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dialing ml gRPC: %w", err)
	}
	log.InfoContext(ctx, "grpc client connected", "service", "ml", "addr", addr)
	return &ClientSet{
		Analog:  pbml.NewAnalogServiceClient(conn),
		closers: []io.Closer{&connCloser{conn}},
	}, nil
}

func (c *ClientSet) Close() error {
	var errs []error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing grpc clients: %v", errs)
	}
	return nil
}

func buildDialOptions(log *slog.Logger, extraOpts ...grpc.DialOption) []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithChainUnaryInterceptor(retryInterceptor(3, log), loggingInterceptor(log)),
	}
	return append(opts, extraOpts...)
}

func retryInterceptor(maxAttempts int, log *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		backoff := 100 * time.Millisecond
		var lastErr error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			lastErr = invoker(ctx, method, req, reply, cc, opts...)
			if lastErr == nil {
				return nil
			}
			if status.Code(lastErr) != codes.Unavailable {
				return lastErr
			}
			if attempt < maxAttempts-1 {
				log.WarnContext(ctx, "grpc unavailable, retrying", "method", method, "attempt", attempt+1, "backoff", backoff)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > time.Second {
					backoff = time.Second
				}
			}
		}
		return lastErr
	}
}

func loggingInterceptor(log *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		log.InfoContext(ctx, "grpc client call", "method", method, "code", code.String(), "duration_ms", time.Since(start).Milliseconds())
		return err
	}
}
```

- [ ] **Step 6: Add dialer test with bufconn**

Create `services/core/internal/platform/grpcclient/client_test.go`:

```go
package grpcclient

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pbml "github.com/foodsea/proto/ml"
)

type fakeAnalogServer struct {
	pbml.UnimplementedAnalogServiceServer
}

func (s *fakeAnalogServer) SearchByPhoto(ctx context.Context, req *pbml.SearchByPhotoRequest) (*pbml.SearchByPhotoResponse, error) {
	return &pbml.SearchByPhotoResponse{MatchedName: "milk"}, nil
}

func TestDialML(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pbml.RegisterAnalogServiceServer(srv, &fakeAnalogServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	clients, err := DialML(
		context.Background(),
		"passthrough:///bufnet",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clients.Close()) })

	resp, err := clients.Analog.SearchByPhoto(context.Background(), &pbml.SearchByPhotoRequest{
		Image:         []byte("img"),
		ImageMimeType: "image/jpeg",
		OcrText:       "milk",
		TopK:          1,
	})
	require.NoError(t, err)
	require.Equal(t, "milk", resp.GetMatchedName())
}
```

- [ ] **Step 7: Run platform tests**

Run:

```bash
cd services/core && go test ./internal/platform/config ./internal/platform/grpcclient ./internal/platform/httputil -count=1
```

Expected: pass.

- [ ] **Step 8: Commit core platform plumbing**

Run:

```bash
git add services/core/internal/platform/config/config.go services/core/internal/platform/config/config_test.go services/core/internal/shared/errors/errors.go services/core/internal/platform/httputil/response.go services/core/internal/platform/grpcclient
git commit -m "feat: add core ml grpc client config"
```

---

### Task 3: Extend Catalog ML Export For Human-Readable Metadata

**Files:**
- Modify: `services/core/internal/modules/catalog/domain/product.go`
- Modify: `services/core/internal/modules/catalog/repository/product_repo.go`
- Modify: `services/core/internal/modules/catalog/grpc/catalog_server.go`
- Modify: `services/core/internal/modules/catalog/grpc/catalog_server_test.go`
- Modify: `services/core/internal/modules/catalog/module.go`

- [ ] **Step 1: Add failing gRPC server assertions**

In `services/core/internal/modules/catalog/grpc/catalog_server_test.go`, extend the existing fake product/test assertion with:

```go
assert.Equal(t, "Простоквашино", got.Products[0].GetBrandName())
assert.Equal(t, "Молочные продукты", got.Products[0].GetCategoryName())
assert.Equal(t, "Молоко", got.Products[0].GetSubcategoryName())
assert.Equal(t, "http://img.test/milk.png", got.Products[0].GetImageUrl())
```

Expected: fail until domain/repository/server carry those fields.

- [ ] **Step 2: Extend `ProductMLData`**

Modify `services/core/internal/modules/catalog/domain/product.go`:

```go
type ProductMLData struct {
	ID              uuid.UUID
	Name            string
	Description     *string
	Composition     *string
	CategoryID      uuid.UUID
	SubcategoryID   *uuid.UUID
	BrandID         *uuid.UUID
	CategoryName    string
	SubcategoryName string
	BrandName       string
	ImageURL        *string
	Weight          *string
	Nutrition       *Nutrition
	Offers          []OfferBrief
}
```

- [ ] **Step 3: Load category/brand/image metadata in repository**

In `ListAllForML`, add eager loads:

```go
rows, err := r.client.Product.Query().
	Where(entproduct.InStock(true)).
	WithCategory().
	WithSubcategory().
	WithBrand().
	WithNutrition().
	WithOffers(func(q *ent.OfferQuery) {
		q.Where(entoffer.InStock(true))
	}).
	All(ctx)
```

Set fields while building `domain.ProductMLData`:

```go
item := domain.ProductMLData{
	ID:            row.ID,
	Name:          row.Name,
	Description:   row.Description,
	Composition:   row.Composition,
	CategoryID:    row.CategoryID,
	SubcategoryID: row.SubcategoryID,
	BrandID:       row.BrandID,
	ImageURL:      row.ImageURL,
	Weight:        row.Weight,
}
if row.Edges.Category != nil {
	item.CategoryName = row.Edges.Category.Name
}
if row.Edges.Subcategory != nil {
	item.SubcategoryName = row.Edges.Subcategory.Name
}
if row.Edges.Brand != nil {
	item.BrandName = row.Edges.Brand.Name
}
```

- [ ] **Step 4: Emit fields from catalog gRPC server**

Modify `services/core/internal/modules/catalog/grpc/catalog_server.go`:

```go
protoProduct := &pb.ProductFeaturesProto{
	ProductId:       p.ID.String(),
	Name:            p.Name,
	Description:     strOrEmpty(p.Description),
	Composition:     strOrEmpty(p.Composition),
	CategoryId:      p.CategoryID.String(),
	SubcategoryId:   uuidPtrToString(p.SubcategoryID),
	BrandId:         uuidPtrToString(p.BrandID),
	Weight:          strOrEmpty(p.Weight),
	BrandName:       p.BrandName,
	CategoryName:    p.CategoryName,
	SubcategoryName: p.SubcategoryName,
	ImageUrl:        strOrEmpty(p.ImageURL),
}
```

- [ ] **Step 5: Export catalog product loader for photo_search**

Modify `services/core/internal/modules/catalog/module.go`:

```go
// ProductLoader returns a use case that loads full product details by ID.
func (m *Module) ProductLoader() *usecase.GetProduct {
	return m.GetProduct
}
```

- [ ] **Step 6: Run catalog tests**

Run:

```bash
cd services/core && go test ./internal/modules/catalog/... -count=1
```

Expected: pass.

- [ ] **Step 7: Commit catalog ML metadata**

Run:

```bash
git add services/core/internal/modules/catalog/domain/product.go services/core/internal/modules/catalog/repository/product_repo.go services/core/internal/modules/catalog/grpc/catalog_server.go services/core/internal/modules/catalog/grpc/catalog_server_test.go services/core/internal/modules/catalog/module.go
git commit -m "feat: enrich catalog ml export"
```

---

### Task 4: Add ML Photo-Search Config And Data Loader Fields

**Files:**
- Modify: `services/ml/src/config.py`
- Modify: `services/ml/src/data_loader.py`
- Modify: `services/ml/tests/test_feature_builder.py` or create focused data loader test if one exists

- [ ] **Step 1: Add config test**

Create or extend `services/ml/tests/test_config.py`:

```python
from __future__ import annotations

from src.config import Config


def test_photo_search_defaults(monkeypatch) -> None:
    for key in [
        "PHOTO_SEARCH_ENABLED",
        "PHOTO_SEARCH_INDEX_PATH",
        "PHOTO_SEARCH_PROVIDER",
        "PHOTO_SEARCH_MODEL",
        "PHOTO_SEARCH_DIMENSIONS",
        "PHOTO_SEARCH_MIN_SCORE",
        "PHOTO_SEARCH_BATCH_SIZE",
    ]:
        monkeypatch.delenv(key, raising=False)

    config = Config()

    assert config.PHOTO_SEARCH_ENABLED is True
    assert config.PHOTO_SEARCH_INDEX_PATH == "data/photo_search_index.pkl"
    assert config.PHOTO_SEARCH_PROVIDER == "gemini_api_key"
    assert config.PHOTO_SEARCH_DIMENSIONS == 768
    assert config.PHOTO_SEARCH_MIN_SCORE == 0.25
    assert config.PHOTO_SEARCH_BATCH_SIZE == 32
```

- [ ] **Step 2: Run config test and verify failure**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_config.py -v
```

Expected: fail because photo-search config fields do not exist.

- [ ] **Step 3: Implement config**

Modify `services/ml/src/config.py`:

```python
def _bool_env(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


class Config:
    """Configuration loaded from environment variables with sane defaults."""

    def __init__(self) -> None:
        self.GRPC_PORT = int(os.getenv("GRPC_PORT", "50051"))
        self.CORE_GRPC_ADDR = os.getenv("CORE_GRPC_ADDR", "localhost:9091")
        self.INDEX_PATH = os.getenv("INDEX_PATH", "data/index.pkl")
        self.TEXT_MODEL = os.getenv("TEXT_MODEL", "all-MiniLM-L6-v2")
        self.TEXT_WEIGHT = float(os.getenv("TEXT_WEIGHT", "1.0"))
        self.CATEGORY_WEIGHT = float(os.getenv("CATEGORY_WEIGHT", "3.0"))
        self.NUTRITION_WEIGHT = float(os.getenv("NUTRITION_WEIGHT", "1.5"))
        self.PRICE_WEIGHT = float(os.getenv("PRICE_WEIGHT", "0.8"))
        self.PRICE_PENALTY = float(os.getenv("PRICE_PENALTY", "0.3"))
        self.MIN_SCORE_THRESHOLD = float(os.getenv("MIN_SCORE_THRESHOLD", "0.3"))

        self.PHOTO_SEARCH_ENABLED = _bool_env("PHOTO_SEARCH_ENABLED", True)
        self.PHOTO_SEARCH_INDEX_PATH = os.getenv("PHOTO_SEARCH_INDEX_PATH", "data/photo_search_index.pkl")
        self.PHOTO_SEARCH_PROVIDER = os.getenv("PHOTO_SEARCH_PROVIDER", "gemini_api_key")
        self.GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "")
        self.VERTEX_PROJECT_ID = os.getenv("VERTEX_PROJECT_ID", "")
        self.VERTEX_LOCATION = os.getenv("VERTEX_LOCATION", "us-central1")
        self.PHOTO_SEARCH_MODEL = os.getenv("PHOTO_SEARCH_MODEL", "gemini-embedding-2")
        self.PHOTO_SEARCH_DIMENSIONS = int(os.getenv("PHOTO_SEARCH_DIMENSIONS", "768"))
        self.PHOTO_SEARCH_MIN_SCORE = float(os.getenv("PHOTO_SEARCH_MIN_SCORE", "0.25"))
        self.PHOTO_SEARCH_BATCH_SIZE = int(os.getenv("PHOTO_SEARCH_BATCH_SIZE", "32"))
```

- [ ] **Step 4: Extend `ProductData`**

Modify `services/ml/src/data_loader.py`:

```python
@dataclass(slots=True)
class ProductData:
    product_id: str
    name: str
    description: str
    composition: str
    category_id: str
    subcategory_id: str
    brand_id: str
    category_name: str
    subcategory_name: str
    brand_name: str
    image_url: str
    weight: str
    calories: float
    protein: float
    fat: float
    carbohydrates: float
    offers: dict[str, int]
    min_price_kopecks: int
```

Map new proto fields:

```python
ProductData(
    product_id=proto_product.product_id,
    name=proto_product.name,
    description=proto_product.description,
    composition=proto_product.composition,
    category_id=proto_product.category_id,
    subcategory_id=proto_product.subcategory_id,
    brand_id=proto_product.brand_id,
    category_name=getattr(proto_product, "category_name", ""),
    subcategory_name=getattr(proto_product, "subcategory_name", ""),
    brand_name=getattr(proto_product, "brand_name", ""),
    image_url=getattr(proto_product, "image_url", ""),
    weight=proto_product.weight,
    calories=proto_product.calories,
    protein=proto_product.protein,
    fat=proto_product.fat,
    carbohydrates=proto_product.carbohydrates,
    offers=offers,
    min_price_kopecks=min_price,
)
```

- [ ] **Step 5: Run ML tests**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_config.py tests/test_feature_builder.py -v
```

Expected: pass.

- [ ] **Step 6: Commit ML config/data model**

Run:

```bash
git add services/ml/src/config.py services/ml/src/data_loader.py services/ml/tests/test_config.py services/ml/tests/test_feature_builder.py
git commit -m "feat: add ml photo search config"
```

---

### Task 5: Implement ML OCR Parser

**Files:**
- Create: `services/ml/src/photo_search/__init__.py`
- Create: `services/ml/src/photo_search/parser.py`
- Create: `services/ml/tests/test_photo_parser.py`

- [ ] **Step 1: Write parser tests**

Create `services/ml/tests/test_photo_parser.py`:

```python
from __future__ import annotations

from src.photo_search.parser import OCRProductTextParser, ProductTextMeta


def parser() -> OCRProductTextParser:
    products = [
        ProductTextMeta(product_id="milk-1", name="Молоко 3.2% 950 мл", brand_name="Простоквашино"),
        ProductTextMeta(product_id="milk-2", name="Молоко безлактозное 1 л", brand_name="Простоквашино"),
        ProductTextMeta(product_id="kefir-1", name="Кефир 2.5% 930 г", brand_name="Домик в деревне"),
    ]
    return OCRProductTextParser(products)


def test_extracts_brand_and_name_inside_brand_scope() -> None:
    result = parser().parse("ПРОСТОКВАШИНО\nмолоко 3,2%\n950 мл")

    assert result.matched_brand == "Простоквашино"
    assert result.matched_name == "Молоко 3.2% 950 мл"
    assert result.name_confidence >= 0.55


def test_handles_yo_equivalence_and_punctuation() -> None:
    result = parser().parse("домик в деревне кефир 2.5 930г")

    assert result.matched_brand == "Домик в деревне"
    assert result.matched_name == "Кефир 2.5% 930 г"


def test_falls_back_when_brand_missing() -> None:
    result = parser().parse("молоко отборное 3.2 процент 1 л")

    assert result.matched_brand == ""
    assert "молоко" in result.matched_name
    assert result.normalized_ocr
```

- [ ] **Step 2: Run parser tests and verify failure**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_parser.py -v
```

Expected: fail because parser package does not exist.

- [ ] **Step 3: Implement parser**

Create `services/ml/src/photo_search/__init__.py` as an empty file.

Create `services/ml/src/photo_search/parser.py`:

```python
"""Deterministic OCR parser for photo product search."""

from __future__ import annotations

from dataclasses import dataclass
import re
from difflib import SequenceMatcher


@dataclass(slots=True)
class ProductTextMeta:
    product_id: str
    name: str
    brand_name: str


@dataclass(slots=True)
class ParsedOCR:
    matched_name: str
    matched_brand: str
    normalized_ocr: str
    name_confidence: float


class OCRProductTextParser:
    def __init__(self, products: list[ProductTextMeta]) -> None:
        self.products = products
        self.brands = sorted({p.brand_name for p in products if p.brand_name}, key=len, reverse=True)

    def parse(self, ocr_text: str) -> ParsedOCR:
        normalized = normalize_text(ocr_text)
        brand = self._match_brand(normalized)
        candidates = [p for p in self.products if not brand or p.brand_name == brand]
        name, confidence = self._match_name(normalized, brand, candidates)
        if confidence < 0.55:
            name = fallback_name(normalized, brand)
        return ParsedOCR(
            matched_name=name,
            matched_brand=brand,
            normalized_ocr=normalized,
            name_confidence=confidence,
        )

    def _match_brand(self, normalized_ocr: str) -> str:
        searchable = normalize_for_match(normalized_ocr)
        for brand in self.brands:
            if normalize_for_match(brand) in searchable:
                return brand
        return ""

    def _match_name(self, normalized_ocr: str, brand: str, candidates: list[ProductTextMeta]) -> tuple[str, float]:
        if not candidates:
            return "", 0.0
        cleaned_ocr = normalize_for_match(normalized_ocr.replace(brand.lower(), ""))
        best_name = ""
        best_score = 0.0
        for product in candidates:
            product_norm = normalize_for_match(product.name)
            overlap = token_overlap(cleaned_ocr, product_norm)
            fuzzy = SequenceMatcher(None, cleaned_ocr, product_norm).ratio()
            important = important_token_overlap(cleaned_ocr, product_norm)
            score = max(overlap, fuzzy * 0.65) + important * 0.2
            if score > best_score:
                best_name = product.name
                best_score = min(score, 1.0)
        return best_name, best_score


def normalize_text(value: str) -> str:
    value = value.lower().replace("ё", "е").replace(",", ".")
    value = re.sub(r"[^0-9a-zа-я.%\s/-]+", " ", value, flags=re.IGNORECASE)
    value = re.sub(r"\s+", " ", value).strip()
    return value


def normalize_for_match(value: str) -> str:
    value = normalize_text(value)
    value = re.sub(r"(\d)\s*(мл|л|г|гр|кг|%)", r"\1 \2", value)
    return value


def tokens(value: str) -> set[str]:
    return {token for token in normalize_for_match(value).split() if len(token) >= 2}


def token_overlap(left: str, right: str) -> float:
    left_tokens = tokens(left)
    right_tokens = tokens(right)
    if not left_tokens or not right_tokens:
        return 0.0
    return len(left_tokens & right_tokens) / len(right_tokens)


def important_token_overlap(left: str, right: str) -> float:
    pattern = re.compile(r"\d+(?:\.\d+)?|%|мл|л|г|гр|кг")
    left_tokens = set(pattern.findall(left))
    right_tokens = set(pattern.findall(right))
    if not right_tokens:
        return 0.0
    return len(left_tokens & right_tokens) / len(right_tokens)


def fallback_name(normalized_ocr: str, brand: str) -> str:
    value = normalized_ocr
    if brand:
        value = value.replace(normalize_text(brand), "")
    words = [word for word in value.split() if len(word) > 2]
    return " ".join(words[:8]).strip()
```

- [ ] **Step 4: Run parser tests**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_parser.py -v
```

Expected: pass.

- [ ] **Step 5: Commit parser**

Run:

```bash
git add services/ml/src/photo_search/__init__.py services/ml/src/photo_search/parser.py services/ml/tests/test_photo_parser.py
git commit -m "feat: add photo search ocr parser"
```

---

### Task 6: Implement ML Photo Product Index

**Files:**
- Create: `services/ml/src/photo_search/index.py`
- Create: `services/ml/tests/test_photo_index.py`

- [ ] **Step 1: Write index tests**

Create `services/ml/tests/test_photo_index.py`:

```python
from __future__ import annotations

import numpy as np

from src.photo_search.index import PhotoProductIndex, PhotoProductMeta


def build_index() -> PhotoProductIndex:
    index = PhotoProductIndex()
    metas = [
        PhotoProductMeta("a", "Молоко 3.2%", "Простоквашино", "Молочные продукты", "", ""),
        PhotoProductMeta("b", "Кефир 2.5%", "Домик в деревне", "Молочные продукты", "", ""),
        PhotoProductMeta("c", "Банан", "", "Фрукты", "", ""),
    ]
    vectors = np.array([[1.0, 0.0], [0.8, 0.2], [0.0, 1.0]], dtype=np.float32)
    index.build(metas, vectors, provider="fake", model="fake-model", dimensions=2)
    return index


def test_query_returns_sorted_candidates() -> None:
    index = build_index()

    results = index.query(np.array([1.0, 0.0], dtype=np.float32), top_k=2, min_score=0.0)

    assert [r.product_id for r in results] == ["a", "b"]
    assert results[0].score >= results[1].score


def test_save_and_load_roundtrip(tmp_path) -> None:
    path = tmp_path / "photo.pkl"
    index = build_index()
    index.save(str(path))

    loaded = PhotoProductIndex()
    assert loaded.load(str(path), provider="fake", model="fake-model", dimensions=2)
    assert loaded.product_ids == ["a", "b", "c"]


def test_incompatible_metadata_is_rejected(tmp_path) -> None:
    path = tmp_path / "photo.pkl"
    index = build_index()
    index.save(str(path))

    loaded = PhotoProductIndex()
    assert not loaded.load(str(path), provider="fake", model="other", dimensions=2)
```

- [ ] **Step 2: Run index tests and verify failure**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_index.py -v
```

Expected: fail because index does not exist.

- [ ] **Step 3: Implement index**

Create `services/ml/src/photo_search/index.py`:

```python
"""Persisted KNN index for product photo search."""

from __future__ import annotations

from dataclasses import dataclass, asdict
import pickle
from pathlib import Path

import numpy as np
from sklearn.neighbors import NearestNeighbors


@dataclass(slots=True)
class PhotoProductMeta:
    product_id: str
    name: str
    brand_name: str
    category_name: str
    subcategory_name: str
    image_url: str


@dataclass(slots=True)
class PhotoSearchResult:
    product_id: str
    score: float
    meta: PhotoProductMeta


class PhotoProductIndex:
    def __init__(self) -> None:
        self.knn: NearestNeighbors | None = None
        self.product_ids: list[str] = []
        self.metas: dict[str, PhotoProductMeta] = {}
        self.vectors: np.ndarray | None = None
        self.provider = ""
        self.model = ""
        self.dimensions = 0

    def build(self, metas: list[PhotoProductMeta], vectors: np.ndarray, provider: str, model: str, dimensions: int) -> None:
        self.product_ids = [meta.product_id for meta in metas]
        self.metas = {meta.product_id: meta for meta in metas}
        self.vectors = normalize_rows(np.asarray(vectors, dtype=np.float32))
        self.provider = provider
        self.model = model
        self.dimensions = dimensions
        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)

    def query(self, query_vector: np.ndarray, top_k: int, min_score: float) -> list[PhotoSearchResult]:
        if self.knn is None or self.vectors is None or not self.product_ids:
            return []
        top_k = max(1, top_k)
        vector = normalize_rows(np.asarray(query_vector, dtype=np.float32).reshape(1, -1))
        n_neighbors = min(max(top_k * 4, top_k), len(self.product_ids))
        distances, indices = self.knn.kneighbors(vector, n_neighbors=n_neighbors)
        results: list[PhotoSearchResult] = []
        for distance, idx in zip(distances[0], indices[0]):
            product_id = self.product_ids[int(idx)]
            score = 1.0 - float(distance)
            if score < min_score:
                continue
            results.append(PhotoSearchResult(product_id=product_id, score=score, meta=self.metas[product_id]))
        results.sort(key=lambda item: item.score, reverse=True)
        return results[:top_k]

    def save(self, path: str) -> None:
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "provider": self.provider,
            "model": self.model,
            "dimensions": self.dimensions,
            "product_ids": self.product_ids,
            "metas": {pid: asdict(meta) for pid, meta in self.metas.items()},
            "vectors": self.vectors,
        }
        target.write_bytes(pickle.dumps(payload))

    def load(self, path: str, provider: str, model: str, dimensions: int) -> bool:
        source = Path(path)
        if not source.exists():
            return False
        data = pickle.loads(source.read_bytes())
        if data.get("provider") != provider or data.get("model") != model or data.get("dimensions") != dimensions:
            return False
        self.provider = data["provider"]
        self.model = data["model"]
        self.dimensions = data["dimensions"]
        self.product_ids = data["product_ids"]
        self.metas = {pid: PhotoProductMeta(**raw) for pid, raw in data["metas"].items()}
        self.vectors = data["vectors"]
        if self.vectors is None or not self.product_ids:
            self.knn = None
            return True
        self.knn = NearestNeighbors(metric="cosine", algorithm="brute")
        self.knn.fit(self.vectors)
        return True

    def product_metas(self) -> list[PhotoProductMeta]:
        return [self.metas[pid] for pid in self.product_ids]


def normalize_rows(vectors: np.ndarray) -> np.ndarray:
    norms = np.linalg.norm(vectors, axis=1, keepdims=True)
    norms[norms == 0] = 1.0
    return (vectors / norms).astype(np.float32)
```

- [ ] **Step 4: Run index tests**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_index.py -v
```

Expected: pass.

- [ ] **Step 5: Commit index**

Run:

```bash
git add services/ml/src/photo_search/index.py services/ml/tests/test_photo_index.py
git commit -m "feat: add photo product index"
```

---

### Task 7: Implement ML Embedding Providers And Rebuild Command

**Files:**
- Create: `services/ml/src/photo_search/embeddings.py`
- Create: `services/ml/src/photo_search/rebuild_index.py`
- Create: `services/ml/tests/test_photo_embeddings.py`
- Create: `services/ml/tests/test_photo_rebuild.py`
- Modify: `services/ml/Makefile`

- [ ] **Step 1: Write rebuild tests with fake provider**

Create `services/ml/tests/test_photo_rebuild.py`:

```python
from __future__ import annotations

import numpy as np

from src.data_loader import ProductData
from src.photo_search.index import PhotoProductIndex
from src.photo_search.rebuild_index import build_photo_index


class FakeProvider:
    provider_name = "fake"
    model = "fake-model"
    dimensions = 2

    def embed_products(self, texts: list[str]) -> np.ndarray:
        return np.array([[1.0, 0.0] if "молоко" in text.lower() else [0.0, 1.0] for text in texts], dtype=np.float32)

    def embed_query(self, image: bytes, mime_type: str, text: str) -> np.ndarray:
        return np.array([1.0, 0.0], dtype=np.float32)


def products() -> list[ProductData]:
    return [
        ProductData(
            product_id="p1",
            name="Молоко 3.2%",
            description="",
            composition="",
            category_id="c1",
            subcategory_id="s1",
            brand_id="b1",
            category_name="Молочные продукты",
            subcategory_name="Молоко",
            brand_name="Простоквашино",
            image_url="",
            weight="950 мл",
            calories=60,
            protein=3,
            fat=3.2,
            carbohydrates=4.7,
            offers={},
            min_price_kopecks=0,
        )
    ]


def test_build_photo_index_writes_loadable_index(tmp_path) -> None:
    path = tmp_path / "photo.pkl"
    build_photo_index(products(), FakeProvider(), str(path), batch_size=10)

    index = PhotoProductIndex()
    assert index.load(str(path), provider="fake", model="fake-model", dimensions=2)
    assert index.product_ids == ["p1"]
```

- [ ] **Step 2: Run rebuild tests and verify failure**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_rebuild.py -v
```

Expected: fail because embeddings/rebuild modules do not exist.

- [ ] **Step 3: Implement provider interfaces**

Create `services/ml/src/photo_search/embeddings.py`:

```python
"""Embedding providers for photo product search."""

from __future__ import annotations

import base64
import json
import urllib.error
import urllib.request
from typing import Protocol

import numpy as np


class EmbeddingProvider(Protocol):
    provider_name: str
    model: str
    dimensions: int

    def embed_products(self, texts: list[str]) -> np.ndarray: ...
    def embed_query(self, image: bytes, mime_type: str, text: str) -> np.ndarray: ...


class ProviderNotConfiguredError(RuntimeError):
    pass


class GeminiAPIEmbeddingProvider:
    provider_name = "gemini_api_key"

    def __init__(self, api_key: str, model: str, dimensions: int) -> None:
        if not api_key:
            raise ProviderNotConfiguredError("GEMINI_API_KEY is required for photo search")
        self.api_key = api_key
        self.model = model
        self.dimensions = dimensions

    def embed_products(self, texts: list[str]) -> np.ndarray:
        return np.array([self._embed_text(text) for text in texts], dtype=np.float32)

    def embed_query(self, image: bytes, mime_type: str, text: str) -> np.ndarray:
        return self._embed_multimodal(image=image, mime_type=mime_type, text=text)

    def _embed_text(self, text: str) -> list[float]:
        payload = {
            "model": f"models/{self.model}",
            "content": {"parts": [{"text": text}]},
            "output_dimensionality": self.dimensions,
        }
        data = self._post_json(f"https://generativelanguage.googleapis.com/v1beta/models/{self.model}:embedContent", payload)
        return data["embedding"]["values"] if "embedding" in data else data["embeddings"][0]["values"]

    def _embed_multimodal(self, image: bytes, mime_type: str, text: str) -> list[float]:
        payload = {
            "model": f"models/{self.model}",
            "content": {
                "parts": [
                    {"text": text},
                    {"inline_data": {"mime_type": mime_type, "data": base64.b64encode(image).decode("ascii")}},
                ]
            },
            "output_dimensionality": self.dimensions,
        }
        data = self._post_json(f"https://generativelanguage.googleapis.com/v1beta/models/{self.model}:embedContent", payload)
        return data["embedding"]["values"] if "embedding" in data else data["embeddings"][0]["values"]

    def _post_json(self, url: str, payload: dict[str, object]) -> dict[str, object]:
        req = urllib.request.Request(
            url,
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "x-goog-api-key": self.api_key},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:  # noqa: S310 - Google API URL is fixed above
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.URLError as exc:
            raise RuntimeError(f"gemini embedding request failed: {exc}") from exc


class VertexAIEmbeddingProvider:
    provider_name = "vertex_ai"

    def __init__(self, project_id: str, location: str, model: str, dimensions: int) -> None:
        if not project_id:
            raise ProviderNotConfiguredError("VERTEX_PROJECT_ID is required for vertex_ai photo search")
        self.project_id = project_id
        self.location = location
        self.model = model
        self.dimensions = dimensions

    def embed_products(self, texts: list[str]) -> np.ndarray:
        raise ProviderNotConfiguredError("vertex_ai photo search provider is not implemented yet")

    def embed_query(self, image: bytes, mime_type: str, text: str) -> np.ndarray:
        raise ProviderNotConfiguredError("vertex_ai photo search provider is not implemented yet")
```

- [ ] **Step 4: Implement rebuild command**

Create `services/ml/src/photo_search/rebuild_index.py`:

```python
"""CLI for explicit photo-search index rebuild."""

from __future__ import annotations

import logging

import numpy as np

from src.config import Config
from src.data_loader import DataLoader, ProductData
from src.photo_search.embeddings import GeminiAPIEmbeddingProvider, VertexAIEmbeddingProvider, EmbeddingProvider
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta

logger = logging.getLogger(__name__)


def product_text(product: ProductData) -> str:
    parts = [
        product.brand_name,
        product.name,
        product.category_name,
        product.subcategory_name,
        product.composition,
        product.weight,
    ]
    return " ".join(part for part in parts if part).strip()


def meta_from_product(product: ProductData) -> PhotoProductMeta:
    return PhotoProductMeta(
        product_id=product.product_id,
        name=product.name,
        brand_name=product.brand_name,
        category_name=product.category_name,
        subcategory_name=product.subcategory_name,
        image_url=product.image_url,
    )


def build_photo_index(products: list[ProductData], provider: EmbeddingProvider, index_path: str, batch_size: int) -> None:
    metas = [meta_from_product(product) for product in products]
    vectors: list[np.ndarray] = []
    texts = [product_text(product) for product in products]
    for start in range(0, len(texts), batch_size):
        batch = texts[start : start + batch_size]
        vectors.append(provider.embed_products(batch))
    matrix = np.vstack(vectors) if vectors else np.zeros((0, provider.dimensions), dtype=np.float32)
    index = PhotoProductIndex()
    index.build(metas, matrix, provider=provider.provider_name, model=provider.model, dimensions=provider.dimensions)
    index.save(index_path)


def provider_from_config(config: Config) -> EmbeddingProvider:
    if config.PHOTO_SEARCH_PROVIDER == "gemini_api_key":
        return GeminiAPIEmbeddingProvider(config.GEMINI_API_KEY, config.PHOTO_SEARCH_MODEL, config.PHOTO_SEARCH_DIMENSIONS)
    if config.PHOTO_SEARCH_PROVIDER == "vertex_ai":
        return VertexAIEmbeddingProvider(config.VERTEX_PROJECT_ID, config.VERTEX_LOCATION, config.PHOTO_SEARCH_MODEL, config.PHOTO_SEARCH_DIMENSIONS)
    raise ValueError(f"unknown PHOTO_SEARCH_PROVIDER {config.PHOTO_SEARCH_PROVIDER!r}")


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    config = Config()
    loader = DataLoader(config.CORE_GRPC_ADDR)
    products = loader.load_products()
    logger.info("loaded %d products for photo-search index", len(products))
    build_photo_index(products, provider_from_config(config), config.PHOTO_SEARCH_INDEX_PATH, config.PHOTO_SEARCH_BATCH_SIZE)
    logger.info("photo-search index saved to %s", config.PHOTO_SEARCH_INDEX_PATH)


if __name__ == "__main__":
    main()
```

- [ ] **Step 5: Add Makefile targets**

Modify `services/ml/Makefile`:

```make
.PHONY: install proto test run probe-weights rebuild-photo-index

rebuild-photo-index:
	$(PYTHON) -m src.photo_search.rebuild_index
```

- [ ] **Step 6: Add provider request-shape tests**

Create `services/ml/tests/test_photo_embeddings.py`:

```python
from __future__ import annotations

import numpy as np
import pytest

from src.photo_search.embeddings import GeminiAPIEmbeddingProvider, ProviderNotConfiguredError


def test_gemini_provider_requires_api_key() -> None:
    with pytest.raises(ProviderNotConfiguredError):
        GeminiAPIEmbeddingProvider("", "gemini-embedding-2", 768)


def test_gemini_query_payload_contains_text_and_inline_image(monkeypatch) -> None:
    provider = GeminiAPIEmbeddingProvider("key", "gemini-embedding-2", 2)
    captured: dict[str, object] = {}

    def fake_post_json(url: str, payload: dict[str, object]) -> dict[str, object]:
        captured["url"] = url
        captured["payload"] = payload
        return {"embedding": {"values": [1.0, 0.0]}}

    monkeypatch.setattr(provider, "_post_json", fake_post_json)

    vector = provider.embed_query(b"img", "image/jpeg", "milk")

    assert np.allclose(vector, np.array([1.0, 0.0], dtype=np.float32))
    assert "gemini-embedding-2:embedContent" in str(captured["url"])
    payload = captured["payload"]
    assert payload["content"]["parts"][0]["text"] == "milk"
    assert payload["content"]["parts"][1]["inline_data"]["mime_type"] == "image/jpeg"
```

- [ ] **Step 7: Run rebuild and provider tests**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_rebuild.py tests/test_photo_embeddings.py -v
```

Expected: pass with fake provider.

- [ ] **Step 8: Commit provider/rebuild command**

Run:

```bash
git add services/ml/src/photo_search/embeddings.py services/ml/src/photo_search/rebuild_index.py services/ml/tests/test_photo_embeddings.py services/ml/tests/test_photo_rebuild.py services/ml/Makefile
git commit -m "feat: add photo search index rebuild"
```

---

### Task 8: Implement ML SearchByPhoto Service

**Files:**
- Create: `services/ml/src/photo_search/service.py`
- Modify: `services/ml/src/service.py`
- Modify: `services/ml/src/main.py`
- Create: `services/ml/tests/test_photo_service.py`

- [ ] **Step 1: Write photo service tests**

Create `services/ml/tests/test_photo_service.py`:

```python
from __future__ import annotations

import numpy as np
from src.config import Config
from src.photo_search.service import PhotoSearchIndexNotReady
from src.photo_search.index import PhotoProductIndex, PhotoProductMeta
from src.photo_search.service import PhotoSearchEngine


class FakeProvider:
    provider_name = "fake"
    model = "fake-model"
    dimensions = 2

    def embed_products(self, texts: list[str]) -> np.ndarray:
        return np.zeros((len(texts), 2), dtype=np.float32)

    def embed_query(self, image: bytes, mime_type: str, text: str) -> np.ndarray:
        return np.array([1.0, 0.0], dtype=np.float32)


def test_search_returns_ranked_candidates() -> None:
    index = PhotoProductIndex()
    index.build(
        [
            PhotoProductMeta("p1", "Молоко 3.2% 950 мл", "Простоквашино", "Молочные продукты", "Молоко", ""),
            PhotoProductMeta("p2", "Кефир 2.5%", "Домик в деревне", "Молочные продукты", "Кефир", ""),
        ],
        np.array([[1.0, 0.0], [0.0, 1.0]], dtype=np.float32),
        provider="fake",
        model="fake-model",
        dimensions=2,
    )
    config = Config()
    config.PHOTO_SEARCH_MIN_SCORE = 0.0
    engine = PhotoSearchEngine(index, FakeProvider(), config)

    result = engine.search(b"image", "image/jpeg", "простоквашино молоко 3,2 950 мл", 5)

    assert result.matched_brand == "Простоквашино"
    assert result.matched_name == "Молоко 3.2% 950 мл"
    assert result.candidates[0].product_id == "p1"


def test_search_missing_index_raises_failed_precondition() -> None:
    config = Config()
    engine = PhotoSearchEngine(PhotoProductIndex(), FakeProvider(), config)

    try:
        engine.search(b"image", "image/jpeg", "milk", 5)
    except PhotoSearchIndexNotReady as exc:
        assert "photo search index is not built" in str(exc)
    else:
        raise AssertionError("expected PhotoSearchIndexNotReady")
```

- [ ] **Step 2: Run service tests and verify failure**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_service.py -v
```

Expected: fail because service does not exist.

- [ ] **Step 3: Implement photo service engine**

Create `services/ml/src/photo_search/service.py`:

```python
"""Photo search orchestration for ml-service."""

from __future__ import annotations

from dataclasses import dataclass

from src.config import Config
from src.photo_search.embeddings import EmbeddingProvider
from src.photo_search.index import PhotoProductIndex, PhotoSearchResult
from src.photo_search.parser import OCRProductTextParser, ProductTextMeta


@dataclass(slots=True)
class PhotoCandidate:
    product_id: str
    score: float


@dataclass(slots=True)
class PhotoSearchResultDTO:
    matched_name: str
    matched_brand: str
    candidates: list[PhotoCandidate]


class PhotoSearchIndexNotReady(RuntimeError):
    pass


class PhotoSearchEngine:
    def __init__(self, index: PhotoProductIndex, provider: EmbeddingProvider, config: Config) -> None:
        self.index = index
        self.provider = provider
        self.config = config

    def search(self, image: bytes, mime_type: str, ocr_text: str, top_k: int) -> PhotoSearchResultDTO:
        if self.index.knn is None:
            raise PhotoSearchIndexNotReady("photo search index is not built")
        parser = OCRProductTextParser([
            ProductTextMeta(product_id=meta.product_id, name=meta.name, brand_name=meta.brand_name)
            for meta in self.index.product_metas()
        ])
        parsed = parser.parse(ocr_text)
        query_text = " ".join(part for part in [parsed.matched_brand, parsed.matched_name, parsed.normalized_ocr] if part)
        vector = self.provider.embed_query(image, mime_type, query_text)
        raw_results = self.index.query(vector, top_k=top_k, min_score=self.config.PHOTO_SEARCH_MIN_SCORE)
        candidates = [PhotoCandidate(result.product_id, adjusted_score(result, parsed.matched_brand, parsed.matched_name)) for result in raw_results]
        candidates.sort(key=lambda item: item.score, reverse=True)
        return PhotoSearchResultDTO(parsed.matched_name, parsed.matched_brand, candidates[:top_k])


def adjusted_score(result: PhotoSearchResult, matched_brand: str, matched_name: str) -> float:
    score = result.score
    if matched_brand and result.meta.brand_name == matched_brand:
        score += 0.03
    elif matched_brand and result.meta.brand_name and result.meta.brand_name != matched_brand:
        score -= 0.05
    if matched_name and result.meta.name == matched_name:
        score += 0.03
    return max(0.0, min(1.0, score))
```

- [ ] **Step 4: Add `SearchByPhoto` to `AnalogServicer`**

Modify `services/ml/src/service.py`:

```python
from src.photo_search.service import PhotoSearchEngine, PhotoSearchIndexNotReady


class AnalogServicer(analogs_pb2_grpc.AnalogServiceServicer):
    def __init__(self, index: AnalogIndex, config: Config, photo_search: PhotoSearchEngine | None = None) -> None:
        self.index = index
        self.config = config
        self.photo_search = photo_search
```

Add method:

```python
def SearchByPhoto(self, request, context):  # noqa: N802
    if self.photo_search is None:
        context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
        context.set_details("photo search is disabled")
        return analogs_pb2.SearchByPhotoResponse()
    if not request.image:
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details("image is required")
        return analogs_pb2.SearchByPhotoResponse()
    if request.image_mime_type not in {"image/jpeg", "image/png"}:
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details("unsupported image_mime_type")
        return analogs_pb2.SearchByPhotoResponse()
    if not request.ocr_text.strip():
        context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
        context.set_details("ocr_text is required")
        return analogs_pb2.SearchByPhotoResponse()

    try:
        result = self.photo_search.search(
            image=request.image,
            mime_type=request.image_mime_type,
            ocr_text=request.ocr_text,
            top_k=request.top_k if request.top_k > 0 else 5,
        )
    except PhotoSearchIndexNotReady:
        context.set_code(grpc.StatusCode.FAILED_PRECONDITION)
        context.set_details("photo search index is not built")
        return analogs_pb2.SearchByPhotoResponse()
    except Exception:  # noqa: BLE001
        context.set_code(grpc.StatusCode.UNAVAILABLE)
        context.set_details("photo search provider unavailable")
        return analogs_pb2.SearchByPhotoResponse()

    return analogs_pb2.SearchByPhotoResponse(
        matched_name=result.matched_name,
        matched_brand=result.matched_brand,
        candidates=[
            analogs_pb2.PhotoSearchCandidate(product_id=item.product_id, score=item.score)
            for item in result.candidates
        ],
    )
```

- [ ] **Step 5: Wire photo search in `main.py`**

Modify `services/ml/src/main.py` with helper functions:

```python
from src.photo_search.embeddings import GeminiAPIEmbeddingProvider, VertexAIEmbeddingProvider, ProviderNotConfiguredError
from src.photo_search.index import PhotoProductIndex
from src.photo_search.service import PhotoSearchEngine
```

Add:

```python
def build_photo_search(config: Config) -> PhotoSearchEngine | None:
    if not config.PHOTO_SEARCH_ENABLED:
        return None
    index = PhotoProductIndex()
    loaded = index.load(
        config.PHOTO_SEARCH_INDEX_PATH,
        provider=config.PHOTO_SEARCH_PROVIDER,
        model=config.PHOTO_SEARCH_MODEL,
        dimensions=config.PHOTO_SEARCH_DIMENSIONS,
    )
    if not loaded:
        logger.warning("photo-search index is missing or incompatible: %s", config.PHOTO_SEARCH_INDEX_PATH)
    try:
        if config.PHOTO_SEARCH_PROVIDER == "gemini_api_key":
            provider = GeminiAPIEmbeddingProvider(config.GEMINI_API_KEY, config.PHOTO_SEARCH_MODEL, config.PHOTO_SEARCH_DIMENSIONS)
        elif config.PHOTO_SEARCH_PROVIDER == "vertex_ai":
            provider = VertexAIEmbeddingProvider(config.VERTEX_PROJECT_ID, config.VERTEX_LOCATION, config.PHOTO_SEARCH_MODEL, config.PHOTO_SEARCH_DIMENSIONS)
        else:
            logger.warning("unknown photo-search provider: %s", config.PHOTO_SEARCH_PROVIDER)
            return None
    except ProviderNotConfiguredError as exc:
        logger.warning("photo-search provider is not configured: %s", exc)
        return None
    return PhotoSearchEngine(index, provider, config)
```

In `serve()`:

```python
photo_search = build_photo_search(config)
analogs_pb2_grpc.add_AnalogServiceServicer_to_server(AnalogServicer(index, config, photo_search), server)
```

- [ ] **Step 6: Run ML photo tests**

Run:

```bash
cd services/ml && .venv/bin/python -m pytest tests/test_photo_parser.py tests/test_photo_index.py tests/test_photo_embeddings.py tests/test_photo_rebuild.py tests/test_photo_service.py -v
```

Expected: pass.

- [ ] **Step 7: Commit ML SearchByPhoto**

Run:

```bash
git add services/ml/src/photo_search/service.py services/ml/src/service.py services/ml/src/main.py services/ml/tests/test_photo_service.py
git commit -m "feat: add ml photo search rpc"
```

---

### Task 9: Implement Core Photo Search Module

**Files:**
- Create: `services/core/internal/modules/photo_search/architecture-notes.md`
- Create: `services/core/internal/modules/photo_search/domain/photo_search.go`
- Create: `services/core/internal/modules/photo_search/usecase/search_by_photo.go`
- Create: `services/core/internal/modules/photo_search/usecase/search_by_photo_test.go`
- Create: `services/core/internal/modules/photo_search/grpc/ml_client.go`
- Create: `services/core/internal/modules/photo_search/grpc/ml_client_test.go`
- Create: `services/core/internal/modules/photo_search/handler/dto.go`
- Create: `services/core/internal/modules/photo_search/handler/handler.go`
- Create: `services/core/internal/modules/photo_search/handler/handler_test.go`
- Create: `services/core/internal/modules/photo_search/module.go`

- [ ] **Step 1: Write usecase tests**

Create `services/core/internal/modules/photo_search/usecase/search_by_photo_test.go`:

```go
package usecase_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/modules/photo_search/usecase"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type mockML struct{ mock.Mock }

func (m *mockML) SearchByPhoto(ctx context.Context, req domain.MLPhotoSearchRequest) (*domain.MLPhotoSearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MLPhotoSearchResult), args.Error(1)
}

type mockProducts struct{ mock.Mock }

func (m *mockProducts) Execute(ctx context.Context, id uuid.UUID) (*catalogdomain.ProductDetail, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*catalogdomain.ProductDetail), args.Error(1)
}

func TestSearchByPhoto_PreservesScoreOrderAndSkipsStaleProducts(t *testing.T) {
	first := uuid.New()
	stale := uuid.New()
	second := uuid.New()
	ml := new(mockML)
	products := new(mockProducts)
	uc := usecase.NewSearchByPhoto(ml, products, 8*1024*1024, 5, 10*time.Second, slog.Default())

	ml.On("SearchByPhoto", mock.Anything, mock.Anything).Return(&domain.MLPhotoSearchResult{
		MatchedName:  "молоко",
		MatchedBrand: "простоквашино",
		Candidates: []domain.MLPhotoCandidate{
			{ProductID: first.String(), Score: 0.9},
			{ProductID: stale.String(), Score: 0.8},
			{ProductID: second.String(), Score: 0.7},
		},
	}, nil)
	products.On("Execute", mock.Anything, first).Return(&catalogdomain.ProductDetail{Product: catalogdomain.Product{ID: first, Name: "first", InStock: true}}, nil)
	products.On("Execute", mock.Anything, stale).Return(nil, sherrors.ErrNotFound)
	products.On("Execute", mock.Anything, second).Return(&catalogdomain.ProductDetail{Product: catalogdomain.Product{ID: second, Name: "second", InStock: true}}, nil)

	result, err := uc.Execute(context.Background(), domain.SearchRequest{
		Image:     []byte("jpeg"),
		MimeType:  "image/jpeg",
		OCRText:   "молоко",
		TopK:      5,
	})

	require.NoError(t, err)
	assert.Equal(t, "молоко", result.MatchedName)
	require.Len(t, result.Candidates, 2)
	assert.Equal(t, first, result.Candidates[0].Product.ID)
	assert.Equal(t, second, result.Candidates[1].Product.ID)
}
```

- [ ] **Step 2: Run usecase tests and verify failure**

Run:

```bash
cd services/core && go test ./internal/modules/photo_search/usecase -count=1
```

Expected: fail because module does not exist.

- [ ] **Step 3: Implement domain types**

Create `services/core/internal/modules/photo_search/domain/photo_search.go`:

```go
package domain

import (
	"context"

	"github.com/google/uuid"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
)

type SearchRequest struct {
	Image    []byte
	MimeType string
	OCRText  string
	TopK     int
}

type SearchResult struct {
	MatchedName  string
	MatchedBrand string
	Candidates   []Candidate
}

type Candidate struct {
	Product catalogdomain.ProductDetail
	Score   float64
	Source  string
}

type MLPhotoSearchRequest struct {
	Image    []byte
	MimeType string
	OCRText  string
	TopK     int
}

type MLPhotoSearchResult struct {
	MatchedName  string
	MatchedBrand string
	Candidates   []MLPhotoCandidate
}

type MLPhotoCandidate struct {
	ProductID string
	Score     float64
}

type MLClient interface {
	SearchByPhoto(ctx context.Context, req MLPhotoSearchRequest) (*MLPhotoSearchResult, error)
}

type ProductLoader interface {
	Execute(ctx context.Context, id uuid.UUID) (*catalogdomain.ProductDetail, error)
}
```

- [ ] **Step 4: Implement usecase**

Create `services/core/internal/modules/photo_search/usecase/search_by_photo.go`:

```go
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

const SourceImageOCR = "image_ocr"

type SearchByPhoto struct {
	ml            domain.MLClient
	products      domain.ProductLoader
	maxImageBytes int64
	defaultTopK   int
	timeout       time.Duration
	log           *slog.Logger
}

func NewSearchByPhoto(ml domain.MLClient, products domain.ProductLoader, maxImageBytes int64, defaultTopK int, timeout time.Duration, log *slog.Logger) *SearchByPhoto {
	return &SearchByPhoto{ml: ml, products: products, maxImageBytes: maxImageBytes, defaultTopK: defaultTopK, timeout: timeout, log: log}
}

func (uc *SearchByPhoto) Execute(ctx context.Context, req domain.SearchRequest) (*domain.SearchResult, error) {
	if err := uc.validate(req); err != nil {
		return nil, err
	}
	topK := req.TopK
	if topK == 0 {
		topK = uc.defaultTopK
	}
	mlCtx := ctx
	cancel := func() {}
	if uc.timeout > 0 {
		mlCtx, cancel = context.WithTimeout(ctx, uc.timeout)
	}
	defer cancel()

	mlResult, err := uc.ml.SearchByPhoto(mlCtx, domain.MLPhotoSearchRequest{
		Image: req.Image, MimeType: req.MimeType, OCRText: strings.TrimSpace(req.OCRText), TopK: topK,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: photo search unavailable", sherrors.ErrUnavailable)
	}

	result := &domain.SearchResult{MatchedName: mlResult.MatchedName, MatchedBrand: mlResult.MatchedBrand}
	for _, item := range mlResult.Candidates {
		id, parseErr := uuid.Parse(item.ProductID)
		if parseErr != nil {
			uc.log.WarnContext(ctx, "ml returned invalid photo-search product id", "product_id", item.ProductID, "error", parseErr)
			continue
		}
		product, loadErr := uc.products.Execute(ctx, id)
		if loadErr != nil {
			if errors.Is(loadErr, sherrors.ErrNotFound) {
				uc.log.WarnContext(ctx, "ml returned stale photo-search product id", "product_id", item.ProductID)
				continue
			}
			return nil, fmt.Errorf("loading photo-search product %s: %w", id, loadErr)
		}
		result.Candidates = append(result.Candidates, domain.Candidate{Product: *product, Score: item.Score, Source: SourceImageOCR})
	}
	return result, nil
}

func (uc *SearchByPhoto) validate(req domain.SearchRequest) error {
	if len(req.Image) == 0 {
		return sherrors.ErrInvalidInput
	}
	if int64(len(req.Image)) > uc.maxImageBytes {
		return &sherrors.ValidationError{Field: "image", Message: "image is too large"}
	}
	if req.MimeType != "image/jpeg" && req.MimeType != "image/png" {
		return &sherrors.ValidationError{Field: "image", Message: "unsupported image mime type"}
	}
	ocr := strings.TrimSpace(req.OCRText)
	if len([]rune(ocr)) < 3 || len([]rune(ocr)) > 4000 {
		return &sherrors.ValidationError{Field: "ocr_text", Message: "length must be between 3 and 4000 characters"}
	}
	if req.TopK < 0 || req.TopK > 10 {
		return &sherrors.ValidationError{Field: "top_k", Message: "must be between 1 and 10"}
	}
	return nil
}
```

- [ ] **Step 5: Implement gRPC adapter**

Create `services/core/internal/modules/photo_search/grpc/ml_client.go`:

```go
package grpc

import (
	"context"
	"fmt"

	"github.com/foodsea/core/internal/modules/photo_search/domain"
	pbml "github.com/foodsea/proto/ml"
)

type MLClient struct {
	client pbml.AnalogServiceClient
}

func NewMLClient(client pbml.AnalogServiceClient) *MLClient {
	return &MLClient{client: client}
}

func (c *MLClient) SearchByPhoto(ctx context.Context, req domain.MLPhotoSearchRequest) (*domain.MLPhotoSearchResult, error) {
	resp, err := c.client.SearchByPhoto(ctx, &pbml.SearchByPhotoRequest{
		Image:         req.Image,
		ImageMimeType: req.MimeType,
		OcrText:       req.OCRText,
		TopK:          int32(req.TopK),
	})
	if err != nil {
		return nil, fmt.Errorf("ml SearchByPhoto: %w", err)
	}
	result := &domain.MLPhotoSearchResult{
		MatchedName:  resp.GetMatchedName(),
		MatchedBrand: resp.GetMatchedBrand(),
		Candidates:   make([]domain.MLPhotoCandidate, 0, len(resp.GetCandidates())),
	}
	for _, candidate := range resp.GetCandidates() {
		result.Candidates = append(result.Candidates, domain.MLPhotoCandidate{
			ProductID: candidate.GetProductId(),
			Score:     candidate.GetScore(),
		})
	}
	return result, nil
}
```

- [ ] **Step 6: Implement handler DTOs**

Create `services/core/internal/modules/photo_search/handler/dto.go`:

```go
package handler

import (
	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	catalogdto "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
)

type Response struct {
	MatchedName  string      `json:"matched_name"`
	MatchedBrand string      `json:"matched_brand"`
	Candidates   []Candidate `json:"candidates"`
}

type Candidate struct {
	Product catalogdto.ProductDetailResponse `json:"product"`
	Score   float64                          `json:"score"`
	Source  string                           `json:"source"`
}

func toResponse(result *domain.SearchResult, render func(*catalogdomain.ProductDetail) catalogdto.ProductDetailResponse) Response {
	resp := Response{MatchedName: result.MatchedName, MatchedBrand: result.MatchedBrand, Candidates: make([]Candidate, 0, len(result.Candidates))}
	for _, candidate := range result.Candidates {
		product := candidate.Product
		resp.Candidates = append(resp.Candidates, Candidate{
			Product: render(&product),
			Score:   candidate.Score,
			Source:  candidate.Source,
		})
	}
	return resp
}
```

- [ ] **Step 7: Implement handler**

Create `services/core/internal/modules/photo_search/handler/handler.go`:

```go
package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	catalogdto "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/platform/httputil"
	sherrors "github.com/foodsea/core/internal/shared/errors"
)

type searchExecutor interface {
	Execute(ctx context.Context, req domain.SearchRequest) (*domain.SearchResult, error)
}

type Handler struct {
	search        searchExecutor
	maxImageBytes int64
	render        func(*catalogdomain.ProductDetail) catalogdto.ProductDetailResponse
}

func New(search searchExecutor, maxImageBytes int64, render func(*catalogdomain.ProductDetail) catalogdto.ProductDetailResponse) *Handler {
	return &Handler{search: search, maxImageBytes: maxImageBytes, render: render}
}

// SearchByPhoto godoc
// @Summary      Найти товар по фото
// @Description  Принимает фото товара и OCR-текст с iOS, возвращает похожие товары каталога
// @Tags         PhotoSearch
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        image    formData file   true  "JPEG/PNG фото товара"
// @Param        ocr_text formData string true  "OCR-текст, считанный iOS"
// @Param        top_k    formData int    false "Количество кандидатов, default=5, max=10"
// @Success      200 {object} httputil.Response{data=Response}
// @Failure      400 {object} httputil.Response{error=string}
// @Failure      401 {object} httputil.Response{error=string}
// @Failure      413 {object} httputil.Response{error=string}
// @Failure      415 {object} httputil.Response{error=string}
// @Failure      422 {object} httputil.Response{error=string}
// @Failure      503 {object} httputil.Response{error=string}
// @Router       /products/photo-search [post]
func (h *Handler) SearchByPhoto(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		httputil.BadRequest(c, "image is required")
		return
	}
	defer file.Close()
	if header.Size > h.maxImageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, httputil.Response{Error: "image is too large"})
		return
	}
	image, err := io.ReadAll(io.LimitReader(file, h.maxImageBytes+1))
	if err != nil {
		httputil.BadRequest(c, "failed to read image")
		return
	}
	if int64(len(image)) > h.maxImageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, httputil.Response{Error: "image is too large"})
		return
	}
	mimeType := http.DetectContentType(image)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		c.JSON(http.StatusUnsupportedMediaType, httputil.Response{Error: "unsupported image mime type"})
		return
	}
	ocrText := strings.TrimSpace(c.PostForm("ocr_text"))
	if ocrText == "" {
		httputil.BadRequest(c, "ocr_text is required")
		return
	}
	topK := 0
	if raw := c.PostForm("top_k"); raw != "" {
		topK, err = strconv.Atoi(raw)
		if err != nil {
			httputil.HandleError(c, &sherrors.ValidationError{Field: "top_k", Message: "must be an integer"})
			return
		}
	}
	result, err := h.search.Execute(c.Request.Context(), domain.SearchRequest{
		Image: image, MimeType: mimeType, OCRText: ocrText, TopK: topK,
	})
	if err != nil {
		httputil.HandleError(c, err)
		return
	}
	httputil.OK(c, toResponse(result, h.render))
}
```

- [ ] **Step 8: Implement module**

Create `services/core/internal/modules/photo_search/module.go`:

```go
package photo_search

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	cataloghandler "github.com/foodsea/core/internal/modules/catalog/handler"
	photodomain "github.com/foodsea/core/internal/modules/photo_search/domain"
	photogrpc "github.com/foodsea/core/internal/modules/photo_search/grpc"
	"github.com/foodsea/core/internal/modules/photo_search/handler"
	"github.com/foodsea/core/internal/modules/photo_search/usecase"
	pbml "github.com/foodsea/proto/ml"
)

type Deps struct {
	MLClient      pbml.AnalogServiceClient
	ProductLoader photodomain.ProductLoader
	MaxImageBytes int64
	Timeout       time.Duration
	Log           *slog.Logger
}

type Module struct {
	h *handler.Handler
}

func NewModule(deps Deps) *Module {
	ml := photogrpc.NewMLClient(deps.MLClient)
	search := usecase.NewSearchByPhoto(ml, deps.ProductLoader, deps.MaxImageBytes, 5, deps.Timeout, deps.Log)
	render := func(d *catalogdomain.ProductDetail) cataloghandler.ProductDetailResponse {
		return cataloghandler.ToProductDetailResponse(d)
	}
	return &Module{h: handler.New(search, deps.MaxImageBytes, render)}
}

func (m *Module) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/products/photo-search", m.h.SearchByPhoto)
}
```

- [ ] **Step 9: Write handler tests**

Create `services/core/internal/modules/photo_search/handler/handler_test.go`:

```go
package handler_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/foodsea/core/internal/modules/catalog/domain"
	catalogdto "github.com/foodsea/core/internal/modules/catalog/handler"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	"github.com/foodsea/core/internal/modules/photo_search/handler"
)

type mockSearch struct{ mock.Mock }

func (m *mockSearch) Execute(ctx context.Context, req domain.SearchRequest) (*domain.SearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SearchResult), args.Error(1)
}

func TestSearchByPhoto_ReturnsCandidates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	search := new(mockSearch)
	productID := uuid.New()
	search.On("Execute", mock.Anything, mock.MatchedBy(func(req domain.SearchRequest) bool {
		return req.MimeType == "image/jpeg" && req.OCRText == "milk" && req.TopK == 5
	})).Return(&domain.SearchResult{
		MatchedName: "milk",
		Candidates: []domain.Candidate{{
			Product: catalogdomain.ProductDetail{Product: catalogdomain.Product{ID: productID, Name: "Milk", InStock: true}},
			Score: 0.9,
			Source: "image_ocr",
		}},
	}, nil)

	r := gin.New()
	h := handler.New(search, 8*1024*1024, catalogdto.ToProductDetailResponse)
	r.POST("/products/photo-search", h.SearchByPhoto)

	body, contentType := multipartBody(t, map[string]string{"ocr_text": "milk", "top_k": "5"}, "image", "milk.jpg", minimalJPEGBytes())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/products/photo-search", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), productID.String())
	search.AssertExpectations(t)
}

func TestSearchByPhoto_UnsupportedMime_Returns415(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.New(new(mockSearch), 8*1024*1024, catalogdto.ToProductDetailResponse)
	r.POST("/products/photo-search", h.SearchByPhoto)

	body, contentType := multipartBody(t, map[string]string{"ocr_text": "milk"}, "image", "milk.txt", []byte("not an image"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/products/photo-search", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func minimalJPEGBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00, 0xff, 0xd9}
}

func multipartBody(t *testing.T, fields map[string]string, fileField string, fileName string, fileBytes []byte) (io.Reader, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	require.NoError(t, err)
	_, err = part.Write(fileBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}
```

- [ ] **Step 10: Write gRPC adapter tests**

Create `services/core/internal/modules/photo_search/grpc/ml_client_test.go` with a fake `pbml.AnalogServiceClient`:

```go
package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	googlegrpc "google.golang.org/grpc"

	photogrpc "github.com/foodsea/core/internal/modules/photo_search/grpc"
	"github.com/foodsea/core/internal/modules/photo_search/domain"
	pbml "github.com/foodsea/proto/ml"
)

type fakeAnalogClient struct{}

func (c *fakeAnalogClient) GetAnalogs(ctx context.Context, in *pbml.GetAnalogsRequest, opts ...googlegrpc.CallOption) (*pbml.GetAnalogsResponse, error) {
	return &pbml.GetAnalogsResponse{}, nil
}

func (c *fakeAnalogClient) GetBatchAnalogs(ctx context.Context, in *pbml.GetBatchAnalogsRequest, opts ...googlegrpc.CallOption) (*pbml.GetBatchAnalogsResponse, error) {
	return &pbml.GetBatchAnalogsResponse{}, nil
}

func (c *fakeAnalogClient) SearchByPhoto(ctx context.Context, in *pbml.SearchByPhotoRequest, opts ...googlegrpc.CallOption) (*pbml.SearchByPhotoResponse, error) {
	return &pbml.SearchByPhotoResponse{
		MatchedName:  "milk",
		MatchedBrand: "brand",
		Candidates: []*pbml.PhotoSearchCandidate{{ProductId: "p1", Score: 0.91}},
	}, nil
}

func TestMLClient_SearchByPhoto(t *testing.T) {
	client := photogrpc.NewMLClient(&fakeAnalogClient{})

	result, err := client.SearchByPhoto(context.Background(), domain.MLPhotoSearchRequest{
		Image: []byte("img"), MimeType: "image/jpeg", OCRText: "milk", TopK: 5,
	})

	require.NoError(t, err)
	require.Equal(t, "milk", result.MatchedName)
	require.Equal(t, "brand", result.MatchedBrand)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "p1", result.Candidates[0].ProductID)
}
```

- [ ] **Step 11: Write architecture notes**

Create `services/core/internal/modules/photo_search/architecture-notes.md`:

```markdown
# Architecture Notes - core-service / photo_search

## Purpose

Protected catalog discovery endpoint for iOS product lookup by product photo plus OCR text.

## Boundaries

- Owns HTTP multipart validation and catalog response shaping.
- Calls internal `ml.AnalogService.SearchByPhoto` through a small gRPC adapter.
- Loads product details through catalog `GetProduct`.
- Does not store uploaded photos.
- Does not call Google directly.

## Route

- `POST /api/v1/products/photo-search`, protected by JWT.

## Error Semantics

- Missing/invalid multipart maps to 400.
- Oversized image maps to 413.
- Unsupported image MIME maps to 415.
- Field validation maps to 422.
- ML/provider/index failures map to 503.
```

- [ ] **Step 12: Run photo_search tests**

Run:

```bash
cd services/core && go test ./internal/modules/photo_search/... -count=1
```

Expected: pass after correcting any compile issues from package aliases.

- [ ] **Step 13: Commit core module**

Run:

```bash
git add services/core/internal/modules/photo_search
git commit -m "feat: add core photo search module"
```

---

### Task 10: Wire Core API, Docs, Makefile, And Swagger

**Files:**
- Modify: `services/core/cmd/api/main.go`
- Modify: `services/core/cmd/api/architecture-notes.md`
- Modify: `services/core/internal/platform/architecture-notes.md`
- Modify: `services/ml/architecture-notes.md`
- Modify: `Makefile`
- Modify: `AGENTS.md`
- Generated: `services/core/docs/swagger/*`

- [ ] **Step 1: Wire ML dialer and photo_search module in `main.go`**

Modify imports:

```go
photo_search "github.com/foodsea/core/internal/modules/photo_search"
coregrpcclient "github.com/foodsea/core/internal/platform/grpcclient"
```

After S3 initialization:

```go
mlClients, err := coregrpcclient.DialML(ctx, cfg.ML.GRPCAddr, log)
if err != nil {
	log.Error("failed to connect to ml grpc", "error", err)
	os.Exit(1)
}
defer mlClients.Close()
```

After `imagesModule`:

```go
photoSearchModule := photo_search.NewModule(photo_search.Deps{
	MLClient:      mlClients.Analog,
	ProductLoader: catalogModule.ProductLoader(),
	MaxImageBytes: cfg.PhotoSearch.MaxImageBytes,
	Timeout:       cfg.PhotoSearch.Timeout,
	Log:           log,
})
```

Route registration:

```go
photoSearchModule.RegisterRoutes(protected)
```

- [ ] **Step 2: Add root Makefile target**

Modify root `Makefile`:

```make
.PHONY: rebuild-photo-search-index

rebuild-photo-search-index:
	cd services/ml && $(MAKE) rebuild-photo-index
```

- [ ] **Step 3: Update architecture notes and AGENTS**

Update notes with these bullets:

```markdown
- `core-service` now has one outbound internal gRPC client: `ml.AnalogService` for protected photo search.
- `POST /api/v1/products/photo-search` is protected and accepts multipart `image`, `ocr_text`, optional `top_k`.
- Uploaded images are processed in memory and are not persisted.
```

Update `services/ml/architecture-notes.md`:

```markdown
## Photo Product Search

- Additional RPC: `SearchByPhoto`.
- Loads persisted `PHOTO_SEARCH_INDEX_PATH` on startup.
- Rebuild is explicit through `python -m src.photo_search.rebuild_index`.
- Query path embeds user image plus OCR text using configured provider.
- Catalog product embeddings are not recomputed on service restart.
```

Update `AGENTS.md` env/command sections after implementation:

```markdown
make rebuild-photo-search-index

Core-only:
- `ML_GRPC_ADDR`
- `PHOTO_SEARCH_MAX_IMAGE_BYTES`
- `PHOTO_SEARCH_TIMEOUT`

ML-only:
- `PHOTO_SEARCH_ENABLED`
- `PHOTO_SEARCH_INDEX_PATH`
- `PHOTO_SEARCH_PROVIDER`
- `GEMINI_API_KEY`
- `VERTEX_PROJECT_ID`
- `VERTEX_LOCATION`
- `PHOTO_SEARCH_MODEL`
- `PHOTO_SEARCH_DIMENSIONS`
- `PHOTO_SEARCH_MIN_SCORE`
- `PHOTO_SEARCH_BATCH_SIZE`
```

- [ ] **Step 4: Generate Swagger**

Run:

```bash
make swagger
```

Expected: core swagger files include `/products/photo-search`.

- [ ] **Step 5: Run targeted compile/tests**

Run:

```bash
cd services/core && go test ./cmd/api ./internal/modules/photo_search/... ./internal/platform/... ./internal/modules/catalog/... -count=1
```

Expected: pass.

- [ ] **Step 6: Commit wiring and docs**

Run:

```bash
git add services/core/cmd/api/main.go services/core/cmd/api/architecture-notes.md services/core/internal/platform/architecture-notes.md services/ml/architecture-notes.md Makefile AGENTS.md services/core/docs/swagger/docs.go services/core/docs/swagger/swagger.json services/core/docs/swagger/swagger.yaml
git commit -m "feat: wire photo search api"
```

---

### Task 11: Add Core E2E Coverage

**Files:**
- Modify: `services/core/test/e2e/suite_test.go`
- Create or modify: `services/core/test/e2e/photo_search_test.go`

- [ ] **Step 1: Add fake ML client to e2e suite**

In `services/core/test/e2e/suite_test.go`, import the new core module and ML proto:

```go
photo_search "github.com/foodsea/core/internal/modules/photo_search"
pbml "github.com/foodsea/proto/ml"
googlegrpc "google.golang.org/grpc"
```

Add a fake generated client implementation:

```go
type fakeMLClient struct{}

func (c *fakeMLClient) GetAnalogs(ctx context.Context, in *pbml.GetAnalogsRequest, opts ...googlegrpc.CallOption) (*pbml.GetAnalogsResponse, error) {
	return &pbml.GetAnalogsResponse{}, nil
}

func (c *fakeMLClient) GetBatchAnalogs(ctx context.Context, in *pbml.GetBatchAnalogsRequest, opts ...googlegrpc.CallOption) (*pbml.GetBatchAnalogsResponse, error) {
	return &pbml.GetBatchAnalogsResponse{}, nil
}

func (c *fakeMLClient) SearchByPhoto(ctx context.Context, req *pbml.SearchByPhotoRequest, opts ...googlegrpc.CallOption) (*pbml.SearchByPhotoResponse, error) {
	return &pbml.SearchByPhotoResponse{
		MatchedName:  "молоко",
		MatchedBrand: "простоквашино",
		Candidates: []*pbml.PhotoSearchCandidate{{ProductId: seededProductID, Score: 0.91}},
	}, nil
}
```

Wire it after `barcodeMod`:

```go
photoSearchMod := photo_search.NewModule(photo_search.Deps{
	MLClient:      &fakeMLClient{},
	ProductLoader: catalogMod.ProductLoader(),
	MaxImageBytes: 8 * 1024 * 1024,
	Timeout:       10 * time.Second,
	Log:           log,
})
```

Register the route after cart routes:

```go
photoSearchMod.RegisterRoutes(protected)
```

- [ ] **Step 2: Add authenticated multipart test**

Create `services/core/test/e2e/photo_search_test.go`:

```go
package e2e

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhotoSearch_ReturnsCandidates(t *testing.T) {
	token := registerUser(t, "photo-search@foodsea.test", "SuperSecret1!")
	body, contentType := multipartBody(t, map[string]string{
		"ocr_text": "простоквашино молоко 3.2 950 мл",
		"top_k": "5",
	}, "image", "milk.jpg", minimalJPEGBytes())

	resp, raw := postMultipartAuth(t, testBaseURL+"/api/v1/products/photo-search", token, body, contentType)

	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	require.Contains(t, raw, "простоквашино")
	require.Contains(t, raw, "candidates")
}

func minimalJPEGBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00, 0xff, 0xd9}
}

func multipartBody(t *testing.T, fields map[string]string, fileField string, fileName string, fileBytes []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	require.NoError(t, err)
	_, err = part.Write(fileBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func postMultipartAuth(t *testing.T, url string, token string, body io.Reader, contentType string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, string(raw)
}
```

- [ ] **Step 3: Run core e2e**

Run:

```bash
make test-e2e-core
```

Expected: pass.

- [ ] **Step 4: Commit e2e coverage**

Run:

```bash
git add services/core/test/e2e/suite_test.go services/core/test/e2e/photo_search_test.go
git commit -m "test: cover core photo search e2e"
```

---

### Task 12: Final Verification

**Files:**
- No new files unless fixes are required.

- [ ] **Step 1: Run ML tests**

Run:

```bash
make test-ml
```

Expected: all ML tests pass.

- [ ] **Step 2: Run core targeted tests**

Run:

```bash
cd services/core && go test ./internal/modules/photo_search/... ./internal/modules/catalog/... ./internal/platform/... -count=1
```

Expected: pass.

- [ ] **Step 3: Run proto generation check**

Run:

```bash
make proto
cd services/ml && make proto
```

Expected: no unintended generated diff after regeneration.

- [ ] **Step 4: Run Swagger generation check**

Run:

```bash
make swagger
```

Expected: no unintended swagger diff beyond the photo-search endpoint.

- [ ] **Step 5: Run broader Go tests**

Run:

```bash
make test-go-all
```

Expected: pass. Infrastructure-dependent failures must be recorded with the failing command and exact error output before completion.

- [ ] **Step 6: Inspect final git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only intended files are modified. Do not clean unrelated dirty files already present in the worktree.
