# Task Scheduler - Планировщик задач

Микросервисная система планировщика задач с автоматическим построением графиков работ и статистикой.

## Структура проекта

```
deadnav/
├── cmd/
│   └── api/              # Точка входа приложения
│       └── main.go
├── internal/             # Внутренний код приложения (не экспортируется)
│   ├── config/          # Конфигурация
│   ├── database/        # Подключение к БД
│   ├── handlers/        # HTTP обработчики
│   ├── models/          # Модели данных
│   └── services/        # Бизнес-логика
├── pkg/                 # Публичные пакеты (можно экспортировать)
│   ├── logger/         # Логгер
│   └── middleware/     # Middleware компоненты
├── scripts/            # SQL скрипты
├── docker-compose.yml  # Docker Compose конфигурация
├── Dockerfile          # Docker образ
└── go.mod             # Go модуль
```

## Быстрый старт

### С помощью Docker Compose (рекомендуется)

```bash
# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f api

# Остановка
docker-compose down
```

### Локальная разработка

1. Установите MySQL и создайте базу данных:
```bash
mysql -u root -p < scripts/init.sql
```

2. Скопируйте файл окружения:
```bash
cp .env.example .env
```

3. Установите зависимости:
```bash
go mod tidy
```

4. Запустите приложение:
```bash
go run cmd/api/main.go
```

## API Endpoints

### Tasks
- `POST /api/v1/tasks` - Создать задачу
- `GET /api/v1/tasks` - Получить все задачи
- `GET /api/v1/tasks/:id` - Получить задачу по ID
- `PUT /api/v1/tasks/:id` - Обновить задачу
- `DELETE /api/v1/tasks/:id` - Удалить задачу

### Statistics
- `GET /api/v1/statistics` - Получить статистику

### Health Check
- `GET /health` - Проверка здоровья сервиса

## Примеры запросов

### Создать задачу
```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Новая задача",
    "description": "Описание задачи",
    "status": "pending",
    "priority": 2,
    "start_date": "2024-01-01T00:00:00Z",
    "end_date": "2024-01-10T00:00:00Z"
  }'
```

### Получить статистику
```bash
curl http://localhost:8080/api/v1/statistics
```

## Добавление нового функционала

### 1. Новый микросервис

Для добавления нового микросервиса (например, для уведомлений):

```
cmd/
└── notification-service/
    └── main.go
```

Создайте отдельный entry point в `cmd/` для каждого микросервиса.

### 2. Новая модель данных

1. Добавьте модель в `internal/models/`:
```go
// internal/models/notification.go
type Notification struct {
    ID      int64     `json:"id"`
    UserID  int64     `json:"user_id"`
    Message string    `json:"message"`
    CreatedAt time.Time `json:"created_at"`
}
```

2. Обновите миграцию в `scripts/init.sql`

### 3. Новый сервис (бизнес-логика)

Создайте сервис в `internal/services/`:
```go
// internal/services/notification_service.go
type NotificationService struct {
    db *sql.DB
}

func (s *NotificationService) SendNotification(userID int64, message string) error {
    // логика отправки уведомления
}
```

### 4. Новый обработчик (handler)

Создайте handler в `internal/handlers/`:
```go
// internal/handlers/notification_handler.go
type NotificationHandler struct {
    service *services.NotificationService
}

func (h *NotificationHandler) SendNotification(c *gin.Context) {
    // обработка запроса
}
```

### 5. Регистрация маршрутов

В `cmd/api/main.go` добавьте новые routes:
```go
notificationGroup := r.Group("/api/v1/notifications")
{
    notificationGroup.POST("", notificationHandler.SendNotification)
    notificationGroup.GET("/:id", notificationHandler.GetNotification)
}
```

### 6. Разделение на микросервисы

Для масштабирования можно выделить сервисы:

- **Task Service** (`cmd/task-service`) - управление задачами
- **Schedule Service** (`cmd/schedule-service`) - построение графиков
- **Statistics Service** (`cmd/statistics-service`) - аналитика
- **Notification Service** (`cmd/notification-service`) - уведомления
- **Auth Service** (`cmd/auth-service`) - аутентификация

Межсервисное взаимодействие через:
- gRPC для синхронных вызовов
- RabbitMQ/Kafka для асинхронных событий

## Масштабирование

### Горизонтальное масштабирование

```bash
# Запуск нескольких инстансов API
docker-compose up -d --scale api=3
```

### Добавление кэширования

Интегрируйте Redis для кэширования часто запрашиваемых данных:
- Статистика
- Списки задач

### Очереди задач

Для фоновых вычислений (построение графиков, отчеты):
- RabbitMQ
- Apache Kafka
- NATS

## Технологии

- **Go 1.21+** - язык программирования
- **Gin** - HTTP фреймворк
- **MySQL 8.0** - база данных
- **Docker & Docker Compose** - контейнеризация
- **Zap** - логирование
- **godotenv** - управление переменными окружения

## Лицензия

MIT
