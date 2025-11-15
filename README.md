# PR Reviewer Assignment Service

Микросервис для автоматического назначения ревьюверов на Pull Request'ы.

## Описание

Сервис автоматически назначает до двух активных ревьюверов из команды автора при создании PR, позволяет выполнять переназначение ревьюверов и получать список PR'ов, назначенных конкретному пользователю.

## Технологический стек

- **Язык**: Go 1.21
- **База данных**: PostgreSQL 15
- **HTTP фреймворк**: Chi v5
- **Миграции**: golang-migrate
- **Контейнеризация**: Docker, Docker Compose

## Быстрый старт

### Вариант 1: Запуск через Docker Compose (рекомендуется)

Это самый простой способ запуска - не требует установки Go и PostgreSQL.

**Требования:**
- Docker
- Docker Compose

**Шаги:**

1. Скачайте и распакуйте проект (если скачали как zip):
   ```bash
   unzip pr-reviewer-service.zip
   cd pr-reviewer-service
   ```

2. Запустите сервис:
   ```bash
   docker-compose up --build
   ```

3. Сервис будет доступен на `http://localhost:8080`

4. Для остановки нажмите `Ctrl+C`, затем:
   ```bash
   docker-compose down
   ```

**Команды Makefile:**
```bash
make docker-up        # Запуск через docker-compose
make docker-down      # Остановка контейнеров
make docker-clean     # Остановка и удаление volumes
```

### Вариант 2: Локальный запуск без Docker

Этот вариант подходит для разработки.

**Требования:**
- Go 1.21 или новее ([скачать](https://go.dev/dl/))
- PostgreSQL 15 или новее ([скачать](https://www.postgresql.org/download/))

**Шаги:**

1. Скачайте проект из GitHub:
   ```bash
   git clone https://github.com/khushdil1201/pr-reviewer-service.git
   cd pr-reviewer-service
   ```

2. Установите PostgreSQL и создайте базу данных:
   ```bash
   # Linux/Mac с psql:
   createdb pr_reviewer
   
   # Или через SQL:
   psql -U postgres -c "CREATE DATABASE pr_reviewer;"
   ```

3. Установите зависимости Go:
   ```bash
   go mod download
   ```

4. Установите переменную окружения с подключением к БД:
   ```bash
   # Linux/Mac:
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable"
   
   # Windows (PowerShell):
   $env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable"
   
   # Windows (CMD):
   set DATABASE_URL=postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable
   ```
   
   > **Важно:** Замените `postgres:postgres` на ваши реальные логин:пароль PostgreSQL

5. Запустите сервис:
   ```bash
   # Через Makefile:
   make run
   
   # Или напрямую:
   go run ./cmd/server
   ```

6. Сервис будет доступен на `http://localhost:8080`

### Вариант 3: Сборка бинарного файла

   ```bash
   # Linux/Mac:
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable"
   
   # Windows (PowerShell):
   $env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable"
   
   # Windows (CMD):
   set DATABASE_URL=postgres://postgres:postgres@localhost:5432/pr_reviewer?sslmode=disable
   ```
   
   > **Важно:** Замените `postgres:postgres` на ваши реальные логин:пароль PostgreSQL

## API Endpoints

### Teams

- `POST /team/add` - Создать команду с участниками
- `GET /team/get?team_name=<name>` - Получить команду
- `POST /team/deactivateAll` - Деактивировать всех пользователей в команде  ← НОВЫЙ

### Users

- `POST /users/setIsActive` - Установить флаг активности пользователя
- `GET /users/getReview?user_id=<id>` - Получить PR'ы пользователя

### Pull Requests

- `POST /pullRequest/create` - Создать PR с автоматическим назначением ревьюверов
- `POST /pullRequest/merge` - Пометить PR как MERGED (идемпотентно)
- `POST /pullRequest/reassign` - Переназначить ревьювера

### Statistics

- `GET /statistics` - Получить общую статистику по системе

Полная спецификация API доступна в файле `openapi.yaml`.

## Примеры использования

### Создание команды

```bash
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true},
      {"user_id": "u3", "username": "Charlie", "is_active": true}
    ]
  }'
```

### Создание PR

```bash
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001",
    "pull_request_name": "Add search feature",
    "author_id": "u1"
  }'
```

## Бизнес-логика

### Назначение ревьюверов

1. При создании PR автоматически назначаются до 2 активных ревьюверов из команды автора
2. Автор PR не может быть назначен ревьювером
3. Назначаются только пользователи с `is_active = true`
4. Если доступных кандидатов меньше двух, назначается доступное количество (0/1)

### Переназначение

1. Заменяет одного ревьювера на случайного активного участника из команды заменяемого ревьювера
2. Новый ревьювер не должен быть автором PR
3. Новый ревьювер не должен быть уже назначен на этот PR
4. После merge PR изменение состава ревьюверов запрещено

### Идемпотентность

Операция merge является идемпотентной - повторный вызов не приводит к ошибке и возвращает актуальное состояние PR.

## Структура проекта

```
.
├── cmd/
│   └── server/
│       └── main.go             # Точка входа приложения
├── internal/
│   ├── handler/                # HTTP-обработчики
│   ├── models/                 # Модели данных
│   ├── repository/             # Работа с БД
│   ├── service/                # Бизнес-логика
│   └── testutil/               # Вспомогательные утилиты для тестирования
├── migrations/                 # Миграции базы данных
├── docker-compose.yml          # Docker Compose конфигурация
├── Dockerfile                  # Dockerfile для сборки
├── Makefile                    # Команды для сборки и запуска
├── openapi.yaml                # OpenAPI спецификация
├── go.mod                      # Модуль Go
├── go.sum                      # Суммы зависимостей
├── .gitignore                  # Игнорируемые файлы
├── .golangci.yml               # Конфигурация линтера
├── README.md                   # Документация (вы здесь!)
├── LOAD_TEST_RESULTS.md        # Результаты нагрузочного тестирования
├── loadtest.go                 # Код нагрузочного теста
├── test_requests.ps1           # Скрипт для тестовых запросов (Windows)
└── test_requests.sh            # Скрипт для тестовых запросов (Linux/macOS)
```

## Требования

- Docker и Docker Compose
- Или Go 1.21+ и PostgreSQL 15+ (для локальной разработки)

## Переменные окружения

- `DATABASE_URL` - URL подключения к PostgreSQL (формат: `postgres://user:password@host:port/dbname?sslmode=disable`)
- `PORT` - Порт для HTTP сервера (по умолчанию: 8080)

## Разработка

### Качество кода

Проект использует **golangci-lint** для поддержания качества кода:

```bash
# Установка (требуется Go ≥ 1.17)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Проверка кода линтером
make lint

# Автоматическое исправление проблем форматирования
make lint-fix

# Запуск тестов
make test
```

**Конфигурация линтера:** `.golangci.yml`

**Подключенные проверки:**
- Проверка необработанных ошибок (errcheck)
- Статический анализ (staticcheck, govet)
- Форматирование кода (gofmt, goimports)
- Проверка стиля (revive)
- Проверка безопасности (gosec)

## Принятые решения и допущения

1. **Выбор случайных ревьюверов**: Используется алгоритм перемешивания (Fisher-Yates shuffle) для случайного выбора ревьюверов из доступных кандидатов.

2. **Переназначение**: При переназначении новый ревьювер выбирается из команды заменяемого ревьювера (а не из команды автора).

3. **Идемпотентность merge**: Повторный вызов merge на уже merged PR возвращает успешный ответ с актуальным состоянием, а не ошибку.

4. **База данных**: Используется PostgreSQL с нормализованной схемой (отдельные таблицы для команд, пользователей, PR и связи PR-ревьюверы).

5. **Миграции**: Применяются автоматически при старте приложения через golang-migrate.
