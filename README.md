# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Сборка с информацией о версии

Информация о версии, дате и коммите подставляется через `-ldflags`:

```bash
make build
```

Или вручную:

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "N/A")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "N/A")

go build -ldflags "\
  -X main.buildVersion=$VERSION \
  -X main.buildDate=$DATE \
  -X main.buildCommit=$COMMIT" \
  -o bin/shortener ./cmd/shortener
```

При старте приложение выводит:

```text
Build version: 1.0.0
Build date: 2025-01-15T10:30:00Z
Build commit: abc1234
```

Если переменные не заданы — выводится `N/A`.

## Конфигурация через JSON

Поддерживается загрузка настроек из JSON-файла:

```bash
./shortener -c config.json
# или
./shortener --config config.json
# или
CONFIG=config.json ./shortener
```

Формат файла (см. `config.example.json`):

```json
{
    "server_address": "localhost:8080",
    "base_url": "http://localhost",
    "file_storage_path": "/path/to/file.db",
    "database_dsn": "",
    "enable_https": true,
    "tls_cert_file": "./certs/cert.pem",
    "tls_key_file": "./certs/key.pem",
    "audit_file": "/path/to/audit.log",
    "audit_url": "http://audit.example.com/events",
    "delete_worker_count": 5,
    "delete_queue_size": 1024,
    "delete_buffer_size": 100,
    "delete_flush_interval": 1000000000,
    "delete_enqueue_timeout": 100000000
}
```

Все поля опциональны, незаданные берутся из env, флагов или дефолтов.
Неизвестные поля игнорируются.

### Поля JSON-файла

| JSON-поле | Env | Флаг | По умолчанию |
|-----------|-----|------|--------------|
| `server_address` | `SERVER_ADDRESS` | `-a` | `localhost:8080` |
| `base_url` | `BASE_URL` | `-b` | `http://localhost:8080` |
| `file_storage_path` | `FILE_STORAGE_PATH` | `-f` | `data/storage.json` |
| `database_dsn` | `DATABASE_DSN` | `-d` | `""` |
| `enable_https` | `ENABLE_HTTPS` | `-s` | `false` |
| `tls_cert_file` | `TLS_CERT_FILE` | `-tls-cert` | `./certs/cert.pem` |
| `tls_key_file` | `TLS_KEY_FILE` | `-tls-key` | `./certs/key.pem` |
| `audit_file` | `AUDIT_FILE` | `-audit-file` | `""` |
| `audit_url` | `AUDIT_URL` | `-audit-url` | `""` |
| `delete_worker_count` | `DELETE_WORKER_COUNT` | — | `5` |
| `delete_queue_size` | `DELETE_QUEUE_SIZE` | — | `1024` |
| `delete_buffer_size` | `DELETE_BUFFER_SIZE` | — | `100` |
| `delete_flush_interval` | `DELETE_FLUSH_INTERVAL` | — | `1s` |
| `delete_enqueue_timeout` | `DELETE_ENQUEUE_TIMEOUT` | — | `100ms` |
| `trusted_subnet` | `TRUSTED_SUBNET` | `-t` / `--trusted-subnet` | `""` |
| `grpc_address` | `GRPC_ADDRESS` | `-g` | `localhost:9090` |

> **Наносекунды:** поля `delete_flush_interval` и `delete_enqueue_timeout` — числа
> в наносекундах (`1000000000` = 1 секунда, `100000000` = 100 миллисекунд).

### Приоритет источников

1. Флаги командной строки (высший)
2. Переменные окружения
3. JSON-файл
4. Дефолты (низший)

Пример: `-a localhost:9090` перекроет `server_address` из файла,
а `SERVER_ADDRESS=...` перекроет его при незаданном флаге.

Флаги имеют абсолютный приоритет, включая явное выключение:
`-s=false` выключает HTTPS, даже если задан `ENABLE_HTTPS=true`
или `"enable_https": true` в JSON-файле.

### Ошибки

- Файл не найден → сервис не стартует с понятной ошибкой
- Невалидный JSON → сервис не стартует с понятной ошибкой

```text
не удалось прочитать файл конфигурации "config.json": open config.json: no such file or directory
невалидный JSON в файле "config.json": invalid character 'x' looking for beginning of value
```

## HTTPS

### Генерация самоподписанного сертификата

```bash
make certs
```

Или вручную:

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 \
  -keyout certs/key.pem \
  -out certs/cert.pem \
  -days 365 \
  -nodes \
  -subj "/CN=localhost"
chmod 600 certs/key.pem
```

### Запуск с HTTPS

```bash
# Через флаг
./shortener -s

# Через переменную окружения
ENABLE_HTTPS=true ./shortener

# С кастомными путями
./shortener -s -tls-cert ./my/cert.pem -tls-key ./my/key.pem
```

> **Важно:** пути по умолчанию (`./certs/cert.pem`, `./certs/key.pem`) — относительные,
> поэтому запускайте бинарник из корня проекта либо передавайте абсолютные пути
> через флаги `-tls-cert` / `-tls-key` или env `TLS_CERT_FILE` / `TLS_KEY_FILE`.

### Конфигурация

| Параметр | Флаг | Env | По умолчанию |
|---|---|---|---|
| Включить HTTPS | `-s` | `ENABLE_HTTPS` | `false` |
| Путь к сертификату | `-tls-cert` | `TLS_CERT_FILE` | `./certs/cert.pem` |
| Путь к ключу | `-tls-key` | `TLS_KEY_FILE` | `./certs/key.pem` |

Значения `ENABLE_HTTPS`: `true`, `1`, `yes` (регистр не важен) — включают HTTPS,
любое другое значение — выключает. Флаг `-s` перекрывает переменную окружения
и JSON-файл, включая явное выключение `-s=false` при `ENABLE_HTTPS=true`.

Сертификаты не коммитятся в git (добавлены в `.gitignore`).

## Эндпоинт /api/internal/stats

Возвращает статистику сервиса:

```json
{
  "urls": 42,
  "users": 7
}
```

- `urls` — количество сокращённых URL;
- `users` — количество уникальных пользователей, которые сокращали URL.

### Доступ

Эндпоинт защищён проверкой IP клиента по доверенной подсети (CIDR).
IP берётся из заголовка `X-Real-IP`.

| Условие | Статус |
|---|---|
| IP в доверенной подсети | `200 OK` |
| IP не в подсети | `403 Forbidden` |
| Заголовка `X-Real-IP` нет | `403 Forbidden` |
| Невалидный IP в `X-Real-IP` | `400 Bad Request` |
| `trusted_subnet` пуст | `403 Forbidden` для всех |

### Конфигурация

| Источник | Ключ |
|---|---|
| JSON | `trusted_subnet` |
| Env | `TRUSTED_SUBNET` |
| Флаг | `-t` или `--trusted-subnet` |

Пример:

```bash
./shortener -t 192.168.1.0/24

TRUSTED_SUBNET=192.168.1.0/24 ./shortener
```

Невалидный CIDR в конфиге → сервер не стартует с понятной ошибкой.

## gRPC

Кроме HTTP, сервис поднимает gRPC-сервер (`shortener.v1.ShortenerService`)
на отдельном порту — параллельно с HTTP, без cmux.

### Методы

| RPC | Аналог HTTP | Описание |
|---|---|---|
| `ShortenURL` | `POST /api/shorten` | сократить URL |
| `ExpandURL` | `GET /{id}` | оригинальный URL по короткому ID (без HTTP-редиректа) |
| `ListUserURLs` | `GET /api/user/urls` | все URL пользователя (запрос — `google.protobuf.Empty`) |

Контракт: `api/shortener/v1/shortener.proto`, сгенерированный код — `gen/pb/shortener/v1/`.

### Запуск

```bash
# Через флаг
./shortener -g localhost:9090

# Через переменную окружения
GRPC_ADDRESS=localhost:9090 ./shortener

# Отключить gRPC (явный пустой флаг)
./shortener -g ""
```

По умолчанию gRPC слушает `localhost:9090`.

### Авторизация

Идентификатор пользователя передаётся в metadata:

```text
authorization: Bearer <JWT>
```

JWT подписывается тем же секретом (`AUTH_SECRET_KEY`), что и кука HTTP,
поэтому токен из HTTP-куки подходит и для gRPC.
Без/с невалидным токеном — статус `Unauthenticated`.

### TLS

Если включён HTTPS (`-s` / `ENABLE_HTTPS=true`), gRPC поднимается с TLS
на тех же сертификатах (`-tls-cert` / `-tls-key`). HTTP без TLS → gRPC без TLS.

### Пример клиента

```go
conn, err := grpc.NewClient("localhost:9090",
    grpc.WithTransportCredentials(insecure.NewCredentials())) // или credentials.NewClientTLSFromFile
if err != nil { log.Fatal(err) }
defer conn.Close()

client := pb.NewShortenerServiceClient(conn)

ctx := metadata.AppendToOutgoingContext(context.Background(),
    "authorization", "Bearer "+token)

resp, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{
    Url: &url,
}.Build())
```

### Кодогенерация

Прото-файл: `api/shortener/v1/shortener.proto`. Регенерация кода в `gen/pb/shortener/v1/`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

protoc \
  --go_out=. --go_opt=module=github.com/b602op/shortener \
  --go_opt=default_api_level=API_OPAQUE \
  --go-grpc_out=. --go-grpc_opt=module=github.com/b602op/shortener \
  -I . \
  -I <путь к include из дистрибутива protoc> \
  api/shortener/v1/shortener.proto
```

Сгенерированный код коммитится в репозиторий (`gen/pb/shortener/v1/`).

`-I <include>` — путь к well-known типам (`google/protobuf/empty.proto`):
идёт в комплекте с protoc (папка `include/` в дистрибутиве
[protobuf releases](https://github.com/protocolbuffers/protobuf/releases)).
