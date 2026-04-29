# FoodSea Backend

Backend-платформа FoodSea для iOS-приложения: каталог продуктов, сравнение цен по магазинам, оптимизация корзины и оформление заказа.

Проект организован как монорепо с несколькими сервисами и общими protobuf-контрактами.

## 1) Архитектура проекта

### Сервисы и зоны ответственности

| Сервис | Технология | HTTP | gRPC | Роль |
|---|---|---:|---:|---|
| `core-service` | Go | `:8081` | `:9091` | Пользователи, каталог, офферы, корзина, поиск, barcode, изображения |
| `optimization-service` | Go | `:8082` | `:9092`* | Оптимизация корзины, работа с аналогами, хранение optimization snapshots |
| `ordering-service` | Go | `:8083` | `:9093` | Заказы и Saga-оркестрация |
| `ml-service` | Python | — | `:50051` | Поиск аналогов по векторной близости |

`*` В `make dev-all` локально `optimization gRPC` запускается на `:9094` (чтобы не конфликтовать с Kafka на `:9092`).

### Межсервисное взаимодействие

- iOS клиент работает с backend по HTTP (через `core`/`optimization`/`ordering`).
- Внутри backend сервисы общаются по gRPC (`core` ↔ `optimization` ↔ `ordering` ↔ `ml`).
- Kafka используется для событий и оркестрации long-running сценариев:
  - `cart.events` (инвалидация результатов оптимизации при изменении корзины),
  - `optimization.events`, `order.events`,
  - `saga.commands`, `saga.replies`.
- Каждый Go-сервис владеет своей БД (Database-per-Service): `core_db`, `optimization_db`, `ordering_db`.

### Архитектурные принципы

- Clean Architecture внутри Go-модулей.
- Domain-слой не зависит от Gin/Ent/Kafka/gRPC.
- Деньги только в `int64` (копейки), без `float`.
- Sentinel errors + `errors.Is` для межслойной классификации ошибок.
- Cache-aside: Redis не источник истины.
- At-least-once обработка Kafka сообщений, идемпотентные обработчики.

## 2) Как работает ML-модель аналогов

`ml-service` строит и обслуживает in-memory KNN-индекс аналогов товаров.

### Источник данных

- Данные берутся из `core-service` по gRPC (`CatalogService.ListProductsForML`).
- Прямого доступа к SQL у `ml-service` нет.

### Признаки (feature vector)

Для каждого товара строится объединённый вектор из:
- текстового эмбеддинга (`sentence-transformers`, модель `all-MiniLM-L6-v2`) по `name + description + composition`;
- нормализованных nutrition-признаков (`calories/protein/fat/carbohydrates`);
- one-hot категории;
- нормализованного веса/объёма (парсинг строк вроде `500 мл`, `1 кг`);
- нормализованного минимального price-признака.

Все компоненты взвешиваются через env-параметры (`TEXT_WEIGHT`, `CATEGORY_WEIGHT`, `NUTRITION_WEIGHT`, `PRICE_WEIGHT`).

### Поиск и ранжирование

- Базовый поиск: `NearestNeighbors(metric="cosine", algorithm="brute")`.
- Режим `price_aware=true` корректирует score относительно цены исходного товара:
  - более дешёвые аналоги получают бонус,
  - более дорогие — штраф.
- `filter_store_ids` ограничивает кандидатов товарами, доступными в выбранных магазинах.
- Результаты ниже `MIN_SCORE_THRESHOLD` отбрасываются.

### Жизненный цикл индекса

- При старте `ml-service` пытается загрузить сериализованный индекс (`INDEX_PATH`).
- Если файла нет, сервис загружает каталог из `core`, строит индекс и сохраняет его на диск.
- Если `core` временно недоступен и индекса нет, сервис стартует с пустым индексом (возвращает пустые результаты, не падает).

## 3) Как работает алгоритм оптимизации корзины

`optimization-service` реализует алгоритм в `optimizer/algorithm`.

### Основная схема

1. Подготовка входа:
- цены по магазинам,
- условия доставки,
- кандидаты аналогов.

2. Сокращение пространства поиска:
- используются только магазины, где есть товары корзины;
- если магазинов слишком много, выбираются top-15 по покрытию корзины.

3. Полный перебор подмножеств магазинов:
- для каждого subset выполняется жадное назначение товара в минимальную цену внутри subset;
- неподходящие subset (не покрывают всю корзину) отбрасываются.

4. Multi-move consolidation:
- система пробует переносить позиции между магазинами,
- цель: достичь `free delivery` там, где это выгоднее, и уменьшить итоговую сумму `товары + доставка`.

5. Этап аналогов:
- поверх оптимального распределения предлагаются замены (`substitutions`),
- учитывается и ценовая дельта, и изменение доставки (включая cross-store сценарии).

6. Timeout fallback:
- при `context deadline` возвращается лучший найденный результат с `is_approximate=true`.

### Результат оптимизации

В snapshot сохраняются:
- `assignments` (что и в каком магазине брать),
- `delivery_kopecks`, `total_kopecks`, `savings_kopecks`,
- `substitutions` (выгодные аналоги),
- статус (`active`, `locked`, `expired`) для сценариев ordering Saga.

## 4) Почему выбран именно этот стек

### Go-сервисы

| Инструмент | Почему выбран |
|---|---|
| `gin` | Быстрый HTTP-слой с простым middleware pipeline и удобной интеграцией со Swagger |
| `ent` | Type-safe ORM + строгие схемы + предсказуемая генерация кода |
| `atlas` | Версионируемые SQL-миграции, удобная связка с Ent |
| `grpc` + `protobuf` | Строгие межсервисные контракты и компактный транспорт |
| `kafka-go` | Простой и стабильный Kafka-клиент в Go без лишней инфраструктуры |
| `go-redis/v9` | Стандартный де-факто клиент Redis для cache-aside сценариев |
| `golang-jwt/jwt` | Прозрачная и контролируемая JWT-аутентификация |
| `swaggo` | Генерация OpenAPI/Swagger из кода handler-ов |

### ML-сервис

| Инструмент | Почему выбран |
|---|---|
| `sentence-transformers` | Качественные готовые эмбеддинги для семантической близости товаров |
| `scikit-learn` (`NearestNeighbors`) | Надёжный и понятный KNN для небольшой/средней размерности данных |
| `numpy` | Быстрая векторная математика для сборки и нормализации признаков |
| `grpcio` | Нативная и стабильная интеграция Python-сервиса в gRPC-контур Go-сервисов |
| `pytest` | Быстрые модульные проверки feature-builder/index/service логики |

### Инфраструктура

| Компонент | Почему выбран |
|---|---|
| PostgreSQL 16 | Надёжная транзакционная БД для core/ordering/optimization |
| Redis 7 | Быстрый кэш и хранение TTL-данных |
| Kafka (Confluent) | Событийная шина и поддержка Saga-аудита |
| MinIO (S3-compatible) | Локальный и предсказуемый storage для изображений |
| Docker Compose | Быстрый локальный запуск зависимостей без k8s-overhead |
| Testcontainers | Интеграционные/e2e тесты в условиях, близких к production runtime |

## 5) Структура репозитория

```text
services/
  core/            # API каталога/корзины/партнёров + gRPC Cart/Offer/Catalog
  optimization/    # API оптимизации + gRPC OptimizationService + cart.events consumer
  ordering/        # API заказов + Saga orchestration
  ml/              # Python gRPC analog service
proto/             # protobuf контракты (общий Go-модуль)
deploy/            # docker-compose, init topics, k8s manifests
docs/api/          # экспортированные API-артефакты
scripts/           # вспомогательные скрипты (в т.ч. swagger regression)
```

## 6) Быстрый старт (локально)

### 0. Зависимости

```bash
make tools
```

### 1. Поднять инфраструктуру

```bash
make dev-infra-up
```

### 2. Применить миграции

```bash
make migrate-core
make migrate-optimization
make migrate-ordering
```

### 3. Генерация контрактов и документации

```bash
make proto
make swagger
```

### 4. Запуск сервисов

Вариант A (всё сразу, в фоне, включая `ml-service`):

```bash
make dev-all
```

Вариант B (по отдельности):

```bash
cd services/core && go run ./cmd/api
cd services/optimization && go run ./cmd/api
cd services/ordering && go run ./cmd/api
cd services/ml && make run
```

### 5. Проверка

- Core health: `http://localhost:8081/health`
- Optimization health: `http://localhost:8082/health`
- Ordering health: `http://localhost:8083/health`

Swagger UI:
- `http://localhost:8081/swagger/index.html`
- `http://localhost:8082/swagger/index.html`
- `http://localhost:8083/swagger/index.html`

## 7) Тестирование

```bash
make test-unit
make test-integration
make test-e2e-core
make test-e2e-ordering
make test-e2e-optimization
make test-ml
```

Полный прогон:

```bash
make test-all
```

## 8) Важные env-переменные

Общие:
- `ENV`, `SERVER_PORT`, `GRPC_PORT`
- `DB_URL`, `REDIS_URL`, `KAFKA_BROKERS`
- `JWT_SECRET`

Optimization-specific:
- `CORE_GRPC_ADDR`, `ML_GRPC_ADDR`
- `OPTIMIZATION_TIMEOUT`, `RESULT_TTL`

Ordering-specific:
- `CORE_GRPC_ADDR`, `OPTIMIZATION_GRPC_ADDR`
- `SAGA_STEP_TIMEOUT`, `SAGA_MAX_COMPENSATION_ATTEMPTS`

ML-specific:
- `CORE_GRPC_ADDR`, `GRPC_PORT`, `INDEX_PATH`
- `TEXT_MODEL`, `TEXT_WEIGHT`, `CATEGORY_WEIGHT`, `NUTRITION_WEIGHT`, `PRICE_WEIGHT`, `PRICE_PENALTY`, `MIN_SCORE_THRESHOLD`

## 9) Полезные команды

```bash
make build
make lint
make seed-core
make stop-all
make dev-infra-down
```

---

Если меняется API-контракт (`proto/*.proto`), обязательно запускай `make proto` и проверяй совместимость вызовов между сервисами.
