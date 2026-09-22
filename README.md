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
любое другое значение — выключает. Флаг `-s` перекрывает переменную окружения.

Сертификаты не коммитятся в git (добавлены в `.gitignore`).
