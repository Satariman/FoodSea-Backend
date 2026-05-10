# Photo Product Search Design

## Context

FoodSea needs an iOS-first backend feature for searching catalog products by a photo of the product packaging. The iOS client sends two signals:

- the product photo;
- OCR text extracted on-device by iOS.

The backend must extract a likely product name and brand, vectorize the query together with the image, and return similar products from the existing catalog.

Current relevant implementation:

- `core-service` owns public catalog/search/barcode HTTP APIs and product data in `core_db`.
- `ml-service` owns semantic analog search over an in-memory KNN index loaded from `core.CatalogService.ListProductsForML`.
- `optimization-service` already consumes `ml.AnalogService` for analogs, but photo search is a catalog discovery feature and should live in core.
- Google Gemini Embedding 2 is described by Google as a multimodal embedding model for text/images and is available through Gemini API and Vertex AI. Exact model IDs and SDK calls must be verified against official Google docs at implementation time.

## Goals

- Add a protected core HTTP endpoint for product search by photo.
- Keep uploaded photos in memory only; do not persist them in S3, DB, or logs.
- Keep ML-specific embedding and vector search logic inside `ml-service`.
- Support Gemini API key first for testing, with a provider abstraction that can switch to Vertex AI service-account auth without changing photo-search use cases.
- Avoid spending money on catalog re-embedding during frequent restarts by rebuilding the photo-search index only through an explicit command.
- Cover the feature with unit, gRPC, CLI, and e2e tests using fake embedding providers in CI.

## Non-Goals

- No iOS implementation.
- No new public endpoint in `ml-service`.
- No automatic catalog embedding updates from Kafka or DB hooks in the first version.
- No storage of user-uploaded photos for debug or analytics.
- No separate LLM call for OCR-to-structured-data extraction.

## Recommended Architecture

Use `core-service` as the public API owner and `ml-service` as the internal photo-search engine.

Runtime request flow:

1. iOS calls protected `POST /api/v1/products/photo-search` with `multipart/form-data`.
2. `core-service` validates JWT, file size, MIME type, OCR text, and `top_k`.
3. `core-service` calls `ml.AnalogService.SearchByPhoto` over internal gRPC.
4. `ml-service` parses OCR text, builds a multimodal query embedding from image plus text, searches a persisted local photo-search index, and returns product IDs with scores.
5. `core-service` loads product details from `core_db`, skips stale or invalid IDs, preserves ML ranking order, and returns catalog candidates.

This keeps catalog-facing API behavior in core, while keeping Google provider logic, vector math, index persistence, and OCR heuristics in ML.

## HTTP API Contract

Endpoint:

```http
POST /api/v1/products/photo-search
Authorization: Bearer <jwt>
Content-Type: multipart/form-data
```

Form fields:

- `image`: required file, `image/jpeg` or `image/png`.
- `ocr_text`: required string, trimmed length `3..4000`.
- `top_k`: optional integer, default `5`, maximum `10`.

Default max image size: `8 MiB`.

Successful response:

```json
{
  "data": {
    "matched_name": "молоко 3.2%",
    "matched_brand": "простоквашино",
    "candidates": [
      {
        "product": {
          "id": "00000000-0000-0000-0000-000000000000",
          "name": "Молоко Простоквашино 3.2%",
          "image_url": "https://example.test/product.png",
          "min_price_kopecks": 12990,
          "max_discount_percent": 12,
          "best_offer": null
        },
        "score": 0.9123,
        "source": "image_ocr"
      }
    ]
  }
}
```

Response semantics:

- `matched_name` and `matched_brand` come from `ml-service` OCR parsing.
- `product` should reuse the existing catalog product response shape rather than inventing a separate product DTO.
- Empty result sets return `200` with an empty `candidates` array.
- Product candidates are sorted by ML score descending.

HTTP error mapping:

- `401`: missing or invalid JWT.
- `400`: missing `image`, missing `ocr_text`, or invalid multipart.
- `413`: image exceeds configured size.
- `415`: unsupported image MIME type.
- `422`: invalid `top_k` or OCR text outside allowed length.
- `503`: ML service unavailable, embedding provider unavailable, or photo-search index not ready.

## Proto Contract

Extend `proto/ml/analogs.proto` by adding one RPC to the existing `ml.AnalogService`:

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

Extend `proto/core/catalog.proto` `ProductFeaturesProto` with backward-compatible fields:

- `brand_name`;
- `category_name`;
- `subcategory_name`;
- `image_url`.

The additional catalog fields let `ml-service` build human-readable product text for photo-search indexing without direct SQL access to `core_db`.

## Core-Service Design

Add a new module:

```text
services/core/internal/modules/photo_search/
├── architecture-notes.md
├── domain/
├── grpc/
├── handler/
├── usecase/
└── module.go
```

Responsibilities:

- `handler`: parse multipart requests, map validation errors to HTTP, render response.
- `usecase`: validate normalized request, call ML, load products, preserve ranking order.
- `grpc`: adapt generated `ml.AnalogServiceClient.SearchByPhoto` into a domain interface.
- `domain`: request/result structs and small interfaces such as `PhotoSearchClient` and `ProductLoader`.

Route registration:

- Register under protected `/api/v1`, not public routes.
- Proposed path: `/products/photo-search`.

New core config:

- `ML_GRPC_ADDR`, default `ml-service:50051`.
- `PHOTO_SEARCH_MAX_IMAGE_BYTES`, default `8388608`.
- `PHOTO_SEARCH_TIMEOUT`, default `10s`.

Architecture note change:

- `core-service` currently documents that it has no outbound gRPC clients. This feature changes that: core gets an outbound internal gRPC client only for `ml.AnalogService.SearchByPhoto`. Update `services/core/cmd/api/architecture-notes.md` and `services/core/internal/platform/architecture-notes.md`.

## ML-Service Design

Add a separate photo-search path next to the existing analog-search path. The existing analog index and RPC behavior remain unchanged.

Suggested structure:

```text
services/ml/src/photo_search/
├── __init__.py
├── embeddings.py
├── index.py
├── parser.py
├── rebuild_index.py
└── service.py
```

Main components:

- `EmbeddingProvider`: provider interface used by query and rebuild flows.
- `GeminiAPIEmbeddingProvider`: first working provider, using API key auth.
- `VertexAIEmbeddingProvider`: same interface. In the first implementation it may return a typed `ErrProviderNotConfigured`, but provider selection must not require usecase changes.
- `PhotoProductIndex`: persisted index with product IDs, normalized vectors, product metadata, and model/provider/dimension/version metadata.
- `OCRProductTextParser`: deterministic parser using known brands and product names from the loaded index.
- `SearchByPhoto` gRPC handler: validates request, parses OCR, embeds query, searches index, applies ranking adjustments, returns candidates.

Startup behavior:

- `ml-service` loads `PHOTO_SEARCH_INDEX_PATH` if present and compatible.
- If the file is missing or incompatible, the service still starts.
- `SearchByPhoto` returns gRPC `FAILED_PRECONDITION` with `photo search index is not built` until the index exists.

Rebuild behavior:

- Explicit command: `cd services/ml && python -m src.photo_search.rebuild_index`.
- The command loads products from `core.CatalogService.ListProductsForML`.
- Product embedding text includes brand name, product name, category/subcategory names, composition, and weight.
- Product images are optional. For MVP, do not download product images during rebuild to avoid coupling to MinIO/public URLs; keep index metadata fields reserved for a follow-up product-image embedding step.
- The command calls the configured embedding provider in batches, normalizes vectors, and writes `PHOTO_SEARCH_INDEX_PATH`.
- Rebuild is the only operation that recomputes catalog embeddings.

ML config:

- `PHOTO_SEARCH_ENABLED=true|false`.
- `PHOTO_SEARCH_INDEX_PATH=data/photo_search_index.pkl`.
- `PHOTO_SEARCH_PROVIDER=gemini_api_key|vertex_ai`.
- `GEMINI_API_KEY`.
- `PHOTO_SEARCH_MODEL`, exact default verified during implementation.
- `PHOTO_SEARCH_DIMENSIONS=768`.
- `PHOTO_SEARCH_MIN_SCORE=0.25`.
- `PHOTO_SEARCH_BATCH_SIZE=32`.

## OCR Parsing And Ranking

`ocr_text` is a strong signal, not a source of truth.

Parsing pipeline:

1. Normalize OCR text: lowercase, trim, collapse whitespace, keep Cyrillic, Latin, digits, `%`, and common package separators.
2. Find `matched_brand` by comparing normalized OCR with known brand names from the index. Support case folding, punctuation tolerance, and `ё`/`е` equivalence.
3. If a brand is found, restrict product-name matching to products of that brand.
4. Find `matched_name` by comparing OCR with product names from the selected brand:
   - token overlap;
   - fuzzy similarity for OCR typos;
   - stronger weight for numbers, percentages, and package size tokens such as `3.2%`, `950 мл`, `1 л`.
5. If name confidence is at least `0.55`, return that product-name match as `matched_name`.
6. If name confidence is lower, fall back to a deterministic OCR-derived name after removing brand, packaging noise, and very short tokens.
7. Build query text as `matched_brand + matched_name + normalized OCR`, capped to provider input limits.
8. Embed query using image plus query text.

Ranking:

- Base score is cosine similarity from the multimodal embedding index.
- Apply small deterministic adjustments:
  - brand exact match: `+0.03`;
  - brand mismatch when a brand was confidently found: `-0.05`;
  - product-name match overlap: up to `+0.03`.
- Clamp final score to `0..1`.
- Drop candidates below `PHOTO_SEARCH_MIN_SCORE`.

## Privacy, Security, And Cost Controls

Privacy:

- Do not save uploaded photos.
- Do not log full OCR text.
- Log only request ID, image MIME type, image size, `top_k`, result count, and error class.

Security:

- Endpoint requires JWT.
- Validate MIME type and size before gRPC.
- Keep Gemini/Vertex credentials only in backend environment or Kubernetes secrets.
- Do not expose `ml-service` externally.

Cost:

- Catalog embeddings are generated only by explicit rebuild command.
- Runtime query embeddings happen only for authenticated requests.
- `top_k` is capped at `10`.
- Persisted index contains provider, model, and dimensions metadata; incompatible files are rejected.

## Testing Strategy

Core tests:

- Handler unit tests for successful multipart request, missing image, missing OCR, unsupported MIME, oversized image, invalid `top_k`.
- Usecase unit tests for ML unavailable, stale product IDs, invalid product IDs, empty candidates, and score-order preservation.
- gRPC adapter tests using bufconn and a fake ML server.
- Core e2e test with authenticated multipart request and fake ML gRPC server returning known product IDs.

ML tests:

- Parser tests for Russian OCR, brand matching, `ё`/`е`, punctuation, weights, percentages, missing brand, and brand-first product-name matching.
- Photo index tests with deterministic vectors: build, persist, load, incompatible metadata, cosine query, score threshold.
- gRPC `SearchByPhoto` tests: invalid arguments, missing index -> `FAILED_PRECONDITION`, successful candidates, empty candidates below threshold.
- Rebuild command tests with fake catalog loader and fake embedding provider writing to a temp index path.
- Provider tests should not call real Google APIs in CI; use fakes and narrowly scoped request-shape tests.

Generation and broader checks:

- After proto changes: `make proto`.
- After Python proto changes: `cd services/ml && make proto`.
- After HTTP API changes: `make swagger`.
- Relevant tests: `make test-ml`, core unit tests, and the core e2e target.

## Documentation And Operations

Update these files during implementation:

- `services/core/cmd/api/architecture-notes.md`.
- `services/core/internal/platform/architecture-notes.md`.
- `services/core/internal/modules/photo_search/architecture-notes.md`.
- `services/ml/architecture-notes.md`.
- `AGENTS.md` env and command sections if the feature is implemented.
- `Makefile` with a target such as `make rebuild-photo-search-index`.
- Deployment secrets/config for `ML_GRPC_ADDR`, photo-search config, and Gemini/Vertex credentials.

## Rollout

1. Add proto fields/RPC and regenerate Go/Python stubs.
2. Implement ML parser, index, fake provider tests, and rebuild command.
3. Build an index locally with fake or real provider depending on environment.
4. Add core module with fake ML client tests.
5. Wire core to real `ml-service` through `ML_GRPC_ADDR`.
6. Add Swagger and e2e coverage.
7. Enable the protected endpoint for iOS testing.

## References

- Google DeepMind, Gemini Embedding 2 model page: https://deepmind.google/models/gemini/embedding/
- Google Cloud, Vertex AI Multimodal Embeddings API: https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/multimodal-embeddings-api
- Google AI for Developers, Gemini API embeddings docs: https://ai.google.dev/gemini-api/docs/embeddings
