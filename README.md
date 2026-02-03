# FoodSea Backend

Backend-часть мобильного приложения FoodSea, реализованная на Go с использованием модульной Clean Architecture.

## Архитектура

Проект организован как модульный монолит, где каждый модуль построен по принципам Clean Architecture и содержит следующие слои:

- **Domain** — бизнес-логика, сущности (entities) и интерфейсы репозиториев
- **UseCase** — сценарии использования (use cases), координирующие работу доменных сущностей
- **Interfaces** — HTTP-обработчики (handlers), DTO для запросов и ответов, валидация
- **Infrastructure** — реализации репозиториев для PostgreSQL и Redis, HTTP-клиенты для внешних API

## Структура проекта

```
foodsea-backend/
├── cmd/
│   └── api/
│       └── main.go              # Точка входа приложения
├── internal/
│   ├── modules/                 # Бизнес-модули
│   │   ├── catalog/            # Каталог товаров
│   │   │   ├── domain/          # Сущности и интерфейсы
│   │   │   ├── usecase/        # Бизнес-логика
│   │   │   ├── interfaces/     # HTTP handlers
│   │   │   └── infrastructure/ # Реализации репозиториев
│   │   ├── cart/               # Корзина покупок
│   │   ├── offers/             # Предложения магазинов
│   │   ├── search/              # Поиск и фильтрация
│   │   ├── barcode/            # Поиск по штрихкоду
│   │   ├── voice/              # Обработка голосовых запросов
│   │   ├── optimization/        # Оптимизация стоимости корзины
│   │   ├── analogs/             # Подбор аналогов товаров
│   │   └── partners/           # Магазины-партнеры
│   └── platform/               # Общая инфраструктура
│       ├── config/             # Конфигурация
│       ├── database/           # Подключение к PostgreSQL
│       ├── redis/              # Подключение к Redis
│       └── router/             # HTTP роутинг
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## Модули

### catalog
Управление каталогом товаров: товары, категории, бренды/производители.

### cart
Управление корзиной пользователя: добавление, изменение количества, удаление товаров.

### offers
Предложения магазинов-партнеров: цены, наличие, акции, условия доставки.

### search
Поиск и фильтрация товаров по текстовому запросу, категориям, брендам и ценам.

### barcode
Поиск товара по штрихкоду.

### voice
Обработка текстовых запросов из голосового ввода: извлечение названий товаров и количеств.

### optimization
Расчет оптимального распределения товаров корзины по магазинам-партнерам для минимизации стоимости.

### analogs
Подбор аналогов товаров на основе семантической близости через ML Gateway. .

### partners
Управление магазинами-партнерами и адаптеры для интеграции с их API.

## Технологии

- **Go 1.21+** — язык программирования
- **PostgreSQL 12+** — основная база данных
- **Redis 6.0+** — кэширование
- **Gin** — HTTP веб-фреймворк
- **Swagger** — документация API
- **Docker** — контейнеризация

## Требования

- Go 1.21 или выше
- Docker и docker-compose (для запуска через Docker)
- PostgreSQL 12+ (при запуске без Docker)
- Redis 6.0+ (при запуске без Docker)

## Запуск через Docker

1. Клонируйте репозиторий:
```bash
cd "Repo Backend"
```

2. Запустите все сервисы:
```bash
docker-compose up -d
```

3. Проверьте работоспособность:
```bash
curl http://localhost:8085/api/v1/health
```

4. Откройте Swagger UI:
```
http://localhost:8085/swagger/index.html
```

Или просто откройте в браузере: `http://localhost:8085/` (настроен редирект на Swagger).

## Запуск локально (без Docker)

1. Установите зависимости:
```bash
go mod download
```

2. Настройте переменные окружения (или используйте значения по умолчанию):
```bash
export SERVER_PORT=8080
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=foodsea
export REDIS_HOST=localhost
export REDIS_PORT=6379
```

3. Убедитесь, что PostgreSQL и Redis запущены и доступны.

4. Запустите приложение:
```bash
go run cmd/api/main.go
```

## Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|-----------|----------|----------------------|
| `SERVER_PORT` | Порт HTTP сервера | `8080` |
| `SERVER_HOST` | Хост HTTP сервера | `localhost` |
| `DB_HOST` | Хост PostgreSQL | `localhost` |
| `DB_PORT` | Порт PostgreSQL | `5432` |
| `DB_USER` | Пользователь PostgreSQL | `postgres` |
| `DB_PASSWORD` | Пароль PostgreSQL | `postgres` |
| `DB_NAME` | Имя базы данных | `foodsea` |
| `DB_SSLMODE` | Режим SSL для PostgreSQL | `disable` |
| `REDIS_HOST` | Хост Redis | `localhost` |
| `REDIS_PORT` | Порт Redis | `6379` |
| `REDIS_PASSWORD` | Пароль Redis | (пусто) |

## API Endpoints

### Health Check
- `GET /health` — проверка работоспособности сервера, базы данных и Redis

### Swagger
- `GET /swagger/index.html` — Swagger UI для просмотра документации API

## Разработка

### Добавление нового модуля

1. Создайте структуру папок в `internal/modules/your-module/`:
   - `domain/` — сущности и интерфейсы репозиториев
   - `usecase/` — бизнес-логика
   - `interfaces/` — HTTP handlers
   - `infrastructure/` — реализации репозиториев

2. Реализуйте интерфейсы в domain слое.

3. Реализуйте use cases в usecase слое.

4. Создайте HTTP handlers в interfaces слое.

5. Реализуйте репозитории в infrastructure слое.

6. Зарегистрируйте роуты в `internal/platform/router/router.go`.

### Генерация Swagger документации

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api/main.go
```

