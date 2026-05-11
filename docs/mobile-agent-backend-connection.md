# Mobile Agent: подключение к FoodSea Backend

Короткая спецификация для мобильного агента (iOS/Android) по интеграции с backend.

## 1) Base URL и окружения

- Все REST-эндпоинты имеют префикс: `/api/v1`
- Текущий `dev`:
  - `http://dev.111.88.159.219.nip.io`
- `prod`:
  - отдельный домен (задаётся инфраструктурой), формат такой же: `https://<prod-host>`

Пример:
- `GET http://dev.111.88.159.219.nip.io/api/v1/categories`

## 2) Авторизация (JWT)

### Вход/регистрация
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh` с телом:
  - `{ "refresh_token": "<token>" }`

### Защищённые запросы
- Заголовок:
  - `Authorization: Bearer <access_token>`

### Выход
- `POST /api/v1/auth/logout` (требует Bearer токен)

## 3) Контракты ответа

Обычно success:
- `{ "data": ... }`

Обычно ошибка:
- `{ "error": "..." }`

Типовые коды:
- `200/201` success
- `400` валидация
- `401` неавторизован
- `404` не найдено
- `409` конфликт статуса/состояния

## 4) Основные REST-группы

### Публичные
- `/auth/register`, `/auth/login`, `/auth/refresh`
- `/categories`, `/brands`, `/products`, `/products/:id`
- `/stores`, `/products/:id/offers`
- `/search`
- `/barcode/:code`

### Защищённые (нужен Bearer)
- `/users/me`, `/users/me/onboarding`
- `/cart`, `/cart/items`, `/cart/items/:product_id`
- `/optimize`, `/optimize/:id`, `/analogs/:product_id`
- `/orders`, `/orders/:id`, `/orders/:id/status`, `/orders/:id/saga`

## 5) Важные нюансы для мобильного клиента

1. Через ingress `/health` может быть недоступен (404) — это нормально.  
   Для проверки доступности API используй:
   - `GET /api/v1/categories` (ожидается `200`)
2. Если каталог пустой, оптимизация заказа (`/optimize`) вернёт ошибку наподобие `cart is empty`.
3. Для order-flow нужен `optimization_result_id` из ответа `/optimize`.
4. Все цены приходят в копейках (`int64`), не `float`.

## 6) Рекомендуемая последовательность в приложении

1. `login/register` -> сохранить `access_token` + `refresh_token`.
2. Загрузить справочники: `categories`, `products`, `stores`.
3. Работать с корзиной (`cart/items`).
4. Вызвать `POST /optimize`.
5. Создать заказ `POST /orders` с `optimization_result_id`.
6. При `401` делать `POST /auth/refresh`, обновлять токены и повторять запрос.
