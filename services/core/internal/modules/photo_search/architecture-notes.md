# Architecture Notes — photo_search Module

## Назначение

`photo_search` добавляет защищённый HTTP endpoint core-service для поиска товаров по фото упаковки и OCR-тексту:

- маршрут: `POST /api/v1/products/photo-search`;
- вход: multipart (`image`, `ocr_text`, optional `top_k`);
- выход: кандидаты товаров из core-каталога в порядке ранжирования ML.

Модуль не хранит фото в S3/БД и не имеет собственных Ent-сущностей.

## Границы и зависимости

- **handler**: HTTP/parsing/валидация формы, MIME-check, маппинг ошибок в HTTP.
- **usecase**: бизнес-валидация запроса, вызов ML-клиента, загрузка товаров из catalog, skip stale ID.
- **grpc adapter**: адаптирует `ml.AnalogService.SearchByPhoto` в доменный интерфейс.
- **domain**: DTO и контракты (`PhotoSearchClient`, `ProductLoader`).

Внешние зависимости:

- gRPC клиент `ml.AnalogServiceClient`;
- `catalog` product loader (`Execute(ctx, productID)`), используемый только как read-model.

## Поведение и инварианты

- Поддерживаемые MIME: `image/jpeg`, `image/png`.
- `top_k`: default `5`, диапазон `1..10`.
- `ocr_text`: trim + длина `3..4000`.
- Stale кандидаты (товар уже удалён/недоступен в core) пропускаются без падения запроса.
- Порядок оставшихся кандидатов сохраняет ранжирование, полученное от ML.
- gRPC `Unavailable`/`FailedPrecondition` маппятся в `ErrUnavailable` (HTTP 503 через `httputil`).
