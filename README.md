# PromoGo

Backend-платформа программы лояльности для ритейла. PromoGo принимает события о покупках из 1С/POS, начисляет и списывает баллы, хранит баланс и историю операций, а также предоставляет API для мобильных клиентов и административных систем.

> Репозиторий содержит backend на Go. Мобильное приложение и web-конфигуратор описаны в продуктовых документах, но не входят в текущую кодовую базу.

## Возможности

- начисление и списание баллов с защитой от повторной обработки транзакций;
- настраиваемые правила программы лояльности для магазина;
- интеграция с 1С/POS по store-scoped API-ключам и разрешениям;
- регистрация клиентов по номеру телефона, OTP и refresh-сессии;
- клиентские методы для получения профиля, баланса и истории операций;
- OIDC-аутентификация сотрудников;
- роли и права для организаций, магазинов и platform admin;
- управление сотрудниками, API-ключами и аудит административных действий;
- автоматические SQL-миграции при запуске;
- health/readiness endpoints и graceful shutdown.

Сейчас SMS-коды и уведомления о начислении выводятся в лог. Интерфейсы для внешних SMS- и push-провайдеров уже выделены, но реальные интеграции ещё не подключены.

## Стек

- Go 1.25.12
- PostgreSQL 16
- Redis 7
- `net/http`
- `pgx`
- `go-redis`
- Goose
- Viper
- Docker / Docker Compose

## Архитектура

```mermaid
flowchart LR
    POS["1С / POS"] --> HTTP["HTTP API"]
    Mobile["Мобильный клиент"] --> HTTP
    Admin["Административный клиент"] --> HTTP

    HTTP --> Auth["Auth и middleware"]
    HTTP --> Services["Сервисы лояльности"]
    Services --> Mechanics["Механика баллов"]
    Auth --> Redis[(Redis)]
    Auth --> Postgres[(PostgreSQL)]
    Services --> Postgres
    Services --> Notifications["SMS / push interfaces"]
```

Код разделён на HTTP-слой, прикладные сервисы, доменную модель и PostgreSQL-репозитории. Точка сборки зависимостей находится в `internal/app`.

## Быстрый запуск через Docker Compose

Понадобятся Docker и Docker Compose.

```bash
cp .env.example .env
docker compose -f deployments/docker-compose.yml up --build
```

В PowerShell вместо `cp` можно использовать `Copy-Item .env.example .env`.

После запуска:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Оба запроса должны вернуть HTTP `200`. При старте приложение самостоятельно применяет недостающие миграции.

Остановить окружение:

```bash
docker compose -f deployments/docker-compose.yml down
```

Чтобы удалить также локальный том PostgreSQL:

```bash
docker compose -f deployments/docker-compose.yml down -v
```

## Локальная разработка

Понадобятся:

- Go 1.25.12 (версия toolchain из `go.mod`);
- PostgreSQL и Redis — локально либо через Docker;
- `make` — необязательно, все команды можно запускать напрямую.

Поднимите инфраструктуру:

```bash
make docker-up
```

Создайте локальный файл окружения:

```bash
cp .env.example .env
```

Запустите приложение:

```bash
make run
```

Эквивалент без `make`:

```bash
docker compose -f deployments/docker-compose.yml up -d postgres redis
go run ./cmd/promogo
```

По умолчанию сервер слушает `http://localhost:8080`.

## Конфигурация

Базовая конфигурация хранится в [`configs/config.yaml`](configs/config.yaml). Путь к другому YAML-файлу можно передать через:

```bash
PROMOGO_CONFIG_FILE=/path/to/config.yaml
```

Любой параметр переопределяется переменной окружения с префиксом `PROMOGO_`. Вложенные ключи разделяются символом `_`:

| YAML | Переменная окружения |
|---|---|
| `http.port` | `PROMOGO_HTTP_PORT` |
| `postgres.host` | `PROMOGO_POSTGRES_HOST` |
| `postgres.password` | `PROMOGO_POSTGRES_PASSWORD` |
| `redis.addr` | `PROMOGO_REDIS_ADDR` |
| `auth.access_token_secret` | `PROMOGO_AUTH_ACCESS_TOKEN_SECRET` |
| `oidc.issuer_url` | `PROMOGO_OIDC_ISSUER_URL` |
| `fcm.credentials_json` | `PROMOGO_FCM_CREDENTIALS_JSON` |

Локальный `.env` загружается автоматически и не должен попадать в Git.

Для любого окружения, кроме `development`, обязательно задайте `PROMOGO_AUTH_ACCESS_TOKEN_SECRET` длиной не менее 32 байт. Для staff login также настройте OIDC issuer, audience и при необходимости JWKS URL.

## API

### Системные endpoints

| Метод | Путь | Назначение |
|---|---|---|
| `GET` | `/healthz` | Процесс запущен |
| `GET` | `/readyz` | PostgreSQL и Redis доступны |

### 1С / POS

Авторизация: `Authorization: Bearer <store-api-key>`.

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/v1/transactions` | Начислить баллы за покупку |
| `POST` | `/api/v1/transactions/redeem` | Списать баллы |
| `GET` | `/api/v1/clients/lookup?phone=...` | Найти клиента по телефону |
| `GET` | `/api/v1/clients/{id}/balance` | Получить баланс клиента |

### Мобильный клиент

OTP request/verify, refresh и logout доступны без access token. Методы `/me` и `logout-all` требуют `Authorization: Bearer <customer-access-token>`.

| Метод | Путь |
|---|---|
| `POST` | `/api/v1/auth/otp/request` |
| `POST` | `/api/v1/auth/otp/verify` |
| `POST` | `/api/v1/auth/refresh` |
| `POST` | `/api/v1/auth/logout` |
| `POST` | `/api/v1/auth/logout-all` |
| `GET` | `/api/v1/me` |
| `GET` | `/api/v1/me/balance` |
| `GET` | `/api/v1/me/transactions` |

### Staff / admin

Вход сотрудников выполняется через `POST /api/v1/staff/auth/oidc`. Административные endpoints под `/api/v1/admin/...` используют staff access token и проверяют RBAC-права на организацию или магазин.

Контракт customer/staff auth API находится в [`docs/openapi/auth.yaml`](docs/openapi/auth.yaml).

## Создание первого администратора

HTTP API не может выдать первую административную роль без уже существующего администратора. Для начальной настройки используется CLI-команда:

```bash
go run ./cmd/promogo bootstrap-admin \
  --subject "oidc-subject" \
  --email "admin@example.com" \
  --name "Admin" \
  --org-name "Default Organization"
```

Вместо `--org-name` можно передать `--org-id`. Команда безопасна для повторного запуска: она создаёт либо обновляет нужные записи и предварительно применяет миграции.

## Миграции

SQL-файлы находятся в [`migrations/sql`](migrations/sql) и встроены в бинарник. При обычном запуске приложение применяет их автоматически.

Ручное управление через Makefile:

```bash
make migrate-status
make migrate-up
make migrate-down
make migrate-validate
```

Для нестандартного подключения:

```bash
make migrate-up DATABASE_URL="postgres://user:password@localhost:5432/promogo?sslmode=disable"
```

## Проверки

```bash
make test
make lint
```

Эквивалентные Go-команды:

```bash
go test ./...
go vet ./...
```

## Основные команды

| Команда | Назначение |
|---|---|
| `make build` | Собрать `bin/promogo` |
| `make run` | Запустить backend |
| `make test` | Выполнить тесты |
| `make lint` | Запустить `go vet` |
| `make tidy` | Синхронизировать зависимости |
| `make docker-up` | Запустить PostgreSQL и Redis |
| `make docker-down` | Остановить локальную инфраструктуру |
| `make docker-logs` | Смотреть логи контейнера приложения |

## Структура репозитория

```text
cmd/promogo/                  точка входа и bootstrap-admin
configs/                      YAML-конфигурация
deployments/                  Docker Compose
docs/openapi/                 OpenAPI-контракты
internal/app/                 сборка зависимостей и lifecycle
internal/auth/                JWT, OTP, OIDC и нормализация телефонов
internal/config/              загрузка и проверка конфигурации
internal/domain/              доменные модели и интерфейсы
internal/httpserver/          маршруты, handlers и middleware
internal/mechanic/            механики программы лояльности
internal/notification/        каналы SMS и уведомлений
internal/repository/postgres/ PostgreSQL-репозитории
internal/service/             прикладные сценарии
migrations/sql/               SQL-миграции Goose
```

## Продуктовая документация

- [`Idea.md`](Idea.md) — концепция продукта;
- [`MVP-scope.md`](MVP-scope.md) — границы первого пилота;
- [`Full-scope.md`](Full-scope.md) — развитие по фазам;
- [`ClientChecklist.md`](ClientChecklist.md) — вопросы и зависимости для запуска у клиента.

## Лицензия

Лицензия пока не указана. До появления файла `LICENSE` использование и распространение проекта регулируется правообладателем репозитория.
