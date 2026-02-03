# Currency Quotes Service

## Описание
- **Асинхронное обновление котировок**: возможность запрашивать обновление курсов валютных пар без блокировки выполнения.
- **Фоновая обработка**: использование воркеров для получения данных от внешних провайдеров с поддержкой повторных попыток.
- **Кэширование**: использование выделенного экземпляра Redis для быстрого доступа к последним полученным котировкам (read-through cache). Очередь задач и кэш изолированы в разных Redis-инстансах.
- **Дедупликация**: предотвращение избыточных запросов для одной и той же валютной пары в режиме реального времени.
- **Наблюдаемость**: структурированное логирование в формате JSON и отслеживание запросов с помощью Correlation ID.
- **Документация API**: автоматически генерируемая спецификация Swagger/OpenAPI.

## Быстрый старт

### 1. Клонирование и настройка
```bash
git clone https://github.com/Alex-ttt/exchange-rate-quote-service.git
cd exchange-rate-quote-service
cp .env.example .env
```

Отредактируйте [`.env`](.env), указав как минимум `QUOTESVC_EXTERNAL_API_KEY` — ключ для доступа к API exchangerate.host.

Обязательные переменные:

| Переменная | Описание | Значение по умолчанию |
|------------|----------|-----------------------|
| `QUOTESVC_EXTERNAL_API_KEY` | API-ключ провайдера котировок | — (обязательно задать) |
| `QUOTESVC_REDIS_ASYNQ_ADDR` | Адрес Redis для очереди задач | `redis_asynq:6380` |
| `QUOTESVC_REDIS_CACHE_ADDR` | Адрес Redis для кэша | `redis_cache:6381` |

Значения по умолчанию для Redis рассчитаны на запуск через Docker Compose. При локальном запуске необходимо переопределить их на `localhost` (см. ниже).

Механизмы конфигурации (по убыванию приоритета):
1. **Переменные окружения** (префикс `QUOTESVC_`).
2. **Файл [`.env`](.env)**: загружается при старте приложения.
3. **Файл [config.yaml](internal/config/config.yaml)**: содержит значения по умолчанию.

### 2. Запуск в Docker (рекомендуется)
Самый простой способ — Docker Compose поднимает всю инфраструктуру одной командой:

```bash
docker compose up --build
```

API-ключ можно передать и без файла [`.env`](.env):
```bash
QUOTESVC_EXTERNAL_API_KEY=your_key_here docker compose up --build
```

Docker Compose запустит четыре сервиса:

| Сервис | Образ | Порт | Назначение |
|--------|-------|------|------------|
| **app** | (собирается из [Dockerfile](Dockerfile)) | 8080 | HTTP API + фоновый воркер |
| **db** | `postgres:18.1-alpine` | 5432 | PostgreSQL |
| **redis_asynq** | `redis:8.4.0-alpine` | 6380 | Очередь задач Asynq (persistence, без eviction) |
| **redis_cache** | `redis:8.4.0-alpine` | 6381 | Кэш котировок (без persistence, LRU eviction) |

### 3. Локальный запуск (Go)
Требуется Go 1.25.6+ и запущенные зависимости.

```bash
# Запустить только зависимости через Docker
docker compose up db redis_asynq redis_cache -d

# Установить зависимости Go
go mod download

# Переопределить адреса на localhost и запустить
QUOTESVC_DATABASE_HOST=localhost \
QUOTESVC_REDIS_ASYNQ_ADDR=localhost:6380 \
QUOTESVC_REDIS_CACHE_ADDR=localhost:6381 \
go run ./cmd/app
```

Либо пропишите эти адреса в [`.env`](.env), чтобы не указывать их при каждом запуске.

### 4. Запуск из GoLand
В проекте настроены Run Configurations (каталог [`.run`](.run/):
- **Docker Dependencies Only** — запускает db + redis_asynq + redis_cache.
- **Run App** — запускает приложение на хосте с переменными окружения для `localhost`.

Рекомендуемый порядок: сначала запустить зависимости, затем приложение.

## Компоненты приложения

### API и Swagger UI
Приложение предоставляет REST API для работы с котировками.
- **Swagger UI** доступен по адресу: `http://localhost:8080/swagger/index.html` (если включено в конфиге).
- **Основные эндпоинты**:
    - `POST /quotes/update` — создание асинхронной задачи на обновление.
    - `GET /quotes/{update_id}` — получение статуса и результата обновления.
    - `GET /quotes/latest` — получение последней кэшированной котировки.

### Асинхронная обработка
Обновление котировок происходит асинхронно, чтобы не блокировать клиентские запросы. При вызове `/quotes/update` задача ставится в очередь, а клиент сразу получает `update_id`. Это позволяет масштабировать обработку внешних запросов независимо от API.

### Background worker (asynq)
Фоновый воркер использует библиотеку **Asynq** и **Redis** в качестве брокера сообщений.
- **Два экземпляра Redis**: сервис использует отдельные Redis-инстансы для задач и кэша:
  - `redis_asynq` — хранение очереди задач Asynq. Persistence включена (AOF + RDB), eviction отключён (`noeviction`), что гарантирует сохранность задач.
  - `redis_cache` — кэширование последних котировок. Persistence отключена, включена политика `allkeys-lru` с лимитом 256MB, что позволяет Redis автоматически вытеснять старые ключи.
- **Зачем два Redis**:
  - **Изоляция отказов (Single Responsibility)**: очередь задач и кэш — принципиально разные нагрузки с противоположными требованиями к хранению. Очереди нужна durability и запрет на eviction, кэшу — ограничение памяти и автоматическое вытеснение. Совмещение в одном инстансе заставляет идти на компромисс: либо eviction-политика кэша рискует удалить ключи очереди, либо отключение eviction приводит к OOM при росте кэша.
  - **Независимое масштабирование**: каждый инстанс можно масштабировать и настраивать под свою нагрузку — увеличить `maxmemory` кэша без влияния на очередь, или перенести очередь на более надёжный узел с быстрыми дисками.
  - **Слабая связность (Low Coupling)**: перезапуск, обновление или сбой одного Redis не затрагивает другой. Потеря кэша не останавливает обработку задач, а проблемы с очередью не инвалидируют кэш.
- **Функции воркера**: получение задач из очереди, выполнение HTTP-запросов к провайдеру, обновление данных в БД и обновление кэша.
- **Запуск**: воркер запускается в том же процессе, что и API (в текущей конфигурации Docker Compose).

## Конфигурация (справочник)

| Переменная | Описание | Значение по умолчанию |
|------------|----------|-----------------------|
| `QUOTESVC_SERVER_PORT` | Порт HTTP сервера | `8080` |
| `QUOTESVC_DATABASE_HOST` | Хост PostgreSQL | `db` |
| `QUOTESVC_DATABASE_PORT` | Порт PostgreSQL | `5432` |
| `QUOTESVC_DATABASE_USER` | Пользователь БД | `postgres` |
| `QUOTESVC_DATABASE_PASSWORD` | Пароль БД | `postgres` |
| `QUOTESVC_DATABASE_NAME` | Имя базы данных | `quotesdb` |
| `QUOTESVC_REDIS_ASYNQ_ADDR` | Адрес Redis для очереди задач | `redis_asynq:6380` |
| `QUOTESVC_REDIS_CACHE_ADDR` | Адрес Redis для кэша | `redis_cache:6381` |
| `REDIS_ASYNQ_PORT` | Публикуемый порт Redis Asynq на хосте (только docker-compose) | `6380` |
| `REDIS_CACHE_PORT` | Публикуемый порт Redis Cache на хосте (только docker-compose) | `6381` |
| `QUOTESVC_EXTERNAL_API_KEY` | API-ключ провайдера | (пусто) |
| `QUOTESVC_EXTERNAL_TIMEOUT_SEC` | Таймаут запроса к провайдеру | `5` |
| `QUOTESVC_CACHE_TTL_SEC` | Время жизни кэша котировок | `3600` |

## Healthcheck и Readiness

### Проверки на уровне контейнеров
В [`docker-compose.yml`](docker-compose.yml) настроены следующие проверки:
- **app**: `wget -qO- http://localhost:8080/healthz`
- **db**: `pg_isready`
- **redis_asynq**: `redis-cli -p 6380 PING`
- **redis_cache**: `redis-cli -p 6381 PING`

### Эндпоинты приложения
- `GET /healthz` (Liveness): возвращает `200 OK`, если процесс запущен.
- `GET /readyz` (Readiness): возвращает `200 OK`, если есть соединение с PostgreSQL и Redis (cache). В случае ошибки возвращает `503 Service Unavailable`. Подключение к Redis (asynq) проверяется библиотекой Asynq самостоятельно.

## Конфигурация Redis

Сервис использует два отдельных экземпляра Redis с разными стратегиями хранения. Конфигурация каждого задаётся в файлах, которые монтируются в контейнеры через Docker Compose.

### `redis-asynq.conf` — очередь задач (durable)
Настроен для надёжного хранения задач Asynq. Данные не должны теряться при перезапуске.
- **Persistence**: включена (AOF `appendonly yes` + RDB-снимки через `save`).
- **Eviction**: отключён (`maxmemory-policy noeviction`, `maxmemory 0`). Ключи очереди не могут быть вытеснены.
- **Порт**: `6380`.
- **Том**: `redis_asynq_data:/data` — данные сохраняются между перезапусками.

### `redis-cache.conf` — кэш котировок (ephemeral)
Настроен как чистый кэш. Потеря данных при перезапуске допустима.
- **Persistence**: отключена (`save ""`, `appendonly no`).
- **Eviction**: включён (`maxmemory-policy allkeys-lru`, `maxmemory 256mb`). При нехватке памяти Redis автоматически удаляет наименее используемые ключи.
- **Порт**: `6381`.
- **Том**: отсутствует — данные живут только в памяти.

### Проверка конфигурации Redis

После запуска `docker compose up --build` можно убедиться, что настройки применились корректно:

```bash
# redis_asynq: persistence включена, eviction отключен
docker compose exec redis_asynq redis-cli -p 6380 CONFIG GET appendonly
# → "yes"
docker compose exec redis_asynq redis-cli -p 6380 CONFIG GET maxmemory-policy
# → "noeviction"
docker compose exec redis_asynq redis-cli -p 6380 CONFIG GET save
# → "900 1 300 10 60 10000"

# redis_cache: persistence отключена, LRU eviction включён
docker compose exec redis_cache redis-cli -p 6381 CONFIG GET appendonly
# → "no"
docker compose exec redis_cache redis-cli -p 6381 CONFIG GET maxmemory
# → "268435456" (256 MB)
docker compose exec redis_cache redis-cli -p 6381 CONFIG GET maxmemory-policy
# → "allkeys-lru"
```

### Настройка адресов Redis для разных способов запуска

Значения по умолчанию (`redis_asynq:6380` и `redis_cache:6381`) рассчитаны на запуск через Docker Compose, где сервисы доступны по имени контейнера.

**Docker Compose** — работает без дополнительных настроек:
```bash
docker compose up --build
```

**Локальный запуск** — зависимости запускаются через Docker, приложение — на хосте:
```bash
# 1. Запустить зависимости
docker compose up db redis_asynq redis_cache -d

# 2. Переопределить адреса для localhost
export QUOTESVC_REDIS_ASYNQ_ADDR=localhost:6380
export QUOTESVC_REDIS_CACHE_ADDR=localhost:6381
export QUOTESVC_DATABASE_HOST=localhost
go run ./cmd/app
```

**GoLand** — адреса задаются в Run Configuration `"Run App"` (переменные окружения `QUOTESVC_REDIS_ASYNQ_ADDR=localhost:6380` и `QUOTESVC_REDIS_CACHE_ADDR=localhost:6381`).

**Смена публикуемых портов на хосте** — если порты 6380/6381 заняты:
```bash
REDIS_ASYNQ_PORT=16380 REDIS_CACHE_PORT=16381 docker compose up -d
```
При этом адреса приложения (`QUOTESVC_REDIS_*_ADDR`) менять не нужно — внутри Docker-сети порты остаются прежними.

## Тестирование
Проект включает в себя модульные (unit) и интеграционные тесты.

### Юнит-тесты
Проверяют изолированную логику компонентов без внешних зависимостей.
- **Handler & Middleware**: проверка корректности обработки HTTP-запросов, валидации входных данных и работы Correlation ID.
- **Service & Validator**: проверка бизнес-логики расчёта котировок и правил валидации валютных пар.
- **Mocks**: использование моков для изоляции зависимостей (например, внешних API).

Запуск:
```bash
go test -v ./internal/...
```

### Интеграционные тесты
Проверяют взаимодействие компонентов с реальными инфраструктурными сервисами.
- **База данных (PostgreSQL)**: проверка миграций, корректности сохранения и получения данных, работы уникальных индексов и дедупликации.
- **Кэш (Redis)**: проверка стратегии кэширования (read-through) и инвалидации данных.
- **Testcontainers**: тесты автоматически запускают необходимые контейнеры (Postgres, Redis) перед началом работы, что гарантирует чистоту окружения.

Для запуска интеграционных тестов требуется установленный и запущенный **Docker**.

Запуск:
```bash
go test -v -tags=integration ./internal/integration/...
```

## Docker и CI/CD 
- **Docker**: проект содержит многоэтапный [`Dockerfile`](Dockerfile) для сборки легковесного образа на базе Alpine.
- **CI/CD**: настроен через GitHub Actions ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)).
    - **Lint**: статическая проверка кода с использованием `golangci-lint` для обеспечения качества и стиля кода.
    - **Unit & Integration Tests**: автоматический запуск всех тестов в изолированном окружении с использованием Docker-сервисов для базы данных и кэша.
    - **Security Scan**: проверка кода на наличие уязвимостей с помощью `gosec`.
    - **API E2E Tests**: сценарий полной проверки работоспособности (End-to-End):
        - Сборка Docker-образа приложения.
        - Запуск всей инфраструктуры через `docker compose`.
        - Проверка доступности (Readiness check).
        - Тестирование реальных API-эндпоинтов с помощью `curl` (проверка валидации, создания задач и получения результатов).
    - **Swagger**: автоматическая проверка актуальности сгенерированной документации API.

## Возможные улучшения
- **Transactional Outbox**: использование паттерна Outbox для обеспечения гарантии доставки событий между базой данных и асинхронными задачами.
- **Выделение сервиса поддерживаемых валют**: вынос логики управления списком поддерживаемых валют в отдельный компонент на уровне БД.
- **Разделение на микросервисы**: разделение API и воркера на два независимых сервиса для их индивидуального масштабирования.
- **Несколько провайдеров**: реализация поддержки нескольких провайдеров котировок для повышения отказоустойчивости.
- **Мониторинг**: добавление Grafana/Prometheus для расширенного мониторинга метрик воркера.
- **Rate Limiting**: ограничение частоты запросов для конкретных пользователей/IP.
