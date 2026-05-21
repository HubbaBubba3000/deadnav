# Deadnav — Code Review Report

**Дата:** 2026-05-21  
**Версия:** текущая (на основе исходного кода)  
**Рецензент:** Автоматизированное код-ревью

---

## Содержание

1. [Краткое резюме](#краткое-резюме)
2. [Критические баги (P0)](#критические-баги-p0)
3. [Высокий приоритет (P1)](#высокий-приоритет-p1)
4. [Средний приоритет (P2)](#средний-приоритет-p2)
5. [Низкий приоритет / Code Quality (P3)](#низкий-приоритет--code-quality-p3)
6. [Проблемы безопасности](#проблемы-безопасности)
7. [Проблемы инфраструктуры / Docker](#проблемы-инфраструктуры--docker)
8. [Рекомендации по архитектуре](#рекомендации-по-архитектуре)

---

## Краткое резюме

Проект Deadnav — планировщик задач с календарём на Go (Gin + MySQL + JWT). Архитектура хорошо продумана (чистое разделение handlers/services/models), код в целом читаемый. Однако при детальном анализе обнаружено **несколько критических багов**, которые приводят к **потере данных**, **нерабочей функциональности** и **потенциальной невозможности компиляции**. Ниже — полный разбор.

---

## Критические баги (P0)

### ~~🔴 P0-1. `GetUserByID` никогда не заполняет `TelegramID`~~ ✅ ИСПРАВЛЕНО

### ~~🔴 P0-2. `LoginWithTelegram` — порча уникального имени при конфликте~~ ✅ ИСПРАВЛЕНО

### ~~🔴 P0-3. Проект не скомпилируется: отсутствует `golang.org/x/crypto`~~ ✅ ИСПРАВЛЕНО

### ~~🔴 P0-4. Race condition при авто-планировании задач~~ ✅ ИСПРАВЛЕНО

### ~~🔴 P0-5. `mustUserID` паникует при отсутствии middleware~~ ✅ ИСПРАВЛЕНО

---

### ~~🟠 P1-1. `LoginWithTelegram` — telegram_id никогда не сканируется при существующем пользователе~~ ✅ ИСПРАВЛЕНО

---

### ~~🟠 P1-2. `cmd/server/main.go` — мёртвый код, конфликт портов~~ ✅ ИСПРАВЛЕНО (файл удалён)

---

### ~~🟠 P1-3. `Dockerfile` — `git describe` не сработает в Docker-сборке~~ ✅ ИСПРАВЛЕНО

---

### ~~🟠 P1-4. Нет graceful shutdown~~ ✅ ИСПРАВЛЕНО

---

### ~~🟠 P1-5. Нет `logger.Sync()` при завершении~~ ✅ ИСПРАВЛЕНО

---

### ~~🟠 P1-6. Error-сообщения раскрывают внутренности БД~~ ✅ ИСПРАВЛЕНО

---

## Средний приоритет (P2)

### 🟡 P2-1. Пароли БД в открытом виде в Makefile

**Файл:** `Makefile`

```makefile
db-shell:
    docker exec -it deadnav_db mysql -u deadnav_user -pdeadnav_password deadnav

db-backup:
    docker exec deadnav_db mysqldump -u deadnav_user -pdeadnav_password deadnav > ...
```

**Проблемы:**
1. Пароль `deadnav_password` захардкожен и виден в истории команд и списке процессов
2. Имя контейнера `deadnav_db` не совпадает с реальным (`mysql` в docker-compose): команды не заработают
3. MySQL покажет warning: *"Using a password on the command line interface can be insecure"*

**Исправление:** Использовать переменные окружения:
```makefile
db-shell:
    docker exec -it $$(docker compose ps -q mysql) mysql -u $(DB_USER) -p$(DB_PASSWORD) $(DB_NAME)
```
Либо использовать `docker compose exec`.

---

### 🟡 P2-2. `Makefile` ссылается на несуществующие файлы/контейнеры

**Файл:** `Makefile`

- `COMPOSE_PROD=docker-compose.prod.yml` — файл **отсутствует** в проекте, `make prod` упадёт
- `deadnav_db` — имя контейнера, которого нет в `docker-compose.yml` (там сервис называется `mysql`)

---

### 🟡 P2-3. Модель `Task` имеет два поля для длительности

**Файл:** `internal/models/task.go`

```go
type Task struct {
    DurationMinutes int `json:"duration_minutes"` // используется планировщиком
    EstimatedTime   int `json:"estimated_time"`   // используется в API/UI
}
```

**Проблема:** Два поля с пересекающейся семантикой. Из кода видно, что `DurationMinutes` — основной параметр для планировщика, а `EstimatedTime` вычисляется из complexity/urgency/importance и кажется экспериментальным. Это сбивает с толку и пользователей API, и разработчиков.

**Рекомендация:** Унифицировать до одного поля.

---

### 🟡 P2-4. Отсутствует валидация полей задач

**Файл:** `internal/handlers/task_handler.go`

- Поле `Title` не валидируется (можно создать задачу с пустым названием)
- Поле `Status` не валидируется — клиент может отправить `"invalid_status"`, и БД вернёт ошибку ENUM, но сообщение будет неинформативным
- Поля `Priority`, `Complexity`, `Urgency`, `Importance` не проверяются на диапазон 1–5
- Нет ограничения длины `Title` и `Description`

**Исправление:** Добавить валидацию в handler или service (использовать `binding` теги Gin: `binding:"required,min=1,max=255"` и т.д.)

---

### 🟡 P2-5. `EstimatedTime` пересчитывается при каждом Update

**Файл:** `internal/handlers/task_handler.go`

```go
if task.EstimatedTime == 0 {
    task.EstimatedTime = calculateEstimatedTime(task.Complexity, task.Urgency, task.Importance)
}
```

Это в `CreateTask`, но не в `UpdateTask`. При обновлении задачи estimated_time остаётся прежним, даже если complexity/urgency/importance изменились.

---

### 🟡 P2-6. `Login` — `sql.ErrNoRows` не оборачивается, проверка прямая

**Файл:** `internal/services/user_service.go`, метод `Login`

```go
if err == sql.ErrNoRows {
    return nil, errors.New("invalid credentials")
}
```

Это работает корректно (QueryRow возвращает чистый `sql.ErrNoRows`), но неконсистентно с `task_service.go` где используется `errors.Is(err, sql.ErrNoRows)`.

---

### 🟡 P2-7. Нет таймаута на запросы к БД

Нигде не используется `context.Context` для запросов к БД. Все вызовы `QueryRow`/`Query`/`Exec` используют бесконечный дефолтный контекст. При зависании БД запросы никогда не отменятся.

**Исправление:** Передавать `context.Context` из Gin (`c.Request.Context()`) в сервисы и использовать `QueryRowContext`, `QueryContext`, `ExecContext`.

---

### 🟡 P2-8. `sql.ErrNoRows` не консистентно обрабатывается в `schedule_service`

**Файл:** `internal/services/schedule_service.go`, метод `GetTaskSchedule`

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, fmt.Errorf("GetTaskSchedule: no schedule for task %d: %w", taskID, sql.ErrNoRows)
}
```

Ошибка **оборачивается** с `%w`, сохраняя `sql.ErrNoRows` в цепочке. Это правильно. Но в `user_service.go` метод `GetUserByID` возвращает `errors.New("user not found")` без `%w`, теряя `sql.ErrNoRows`. Нужна консистентность.

---

### 🟡 P2-9. `database.NewMySQLConnection` не настраивает `ConnMaxLifetime`

**Файл:** `internal/database/mysql.go`

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
// Отсутствуют:
// db.SetConnMaxLifetime(time.Duration)
// db.SetConnMaxIdleTime(time.Duration)
```

**Проблема:** MySQL по умолчанию закрывает неактивные соединения через `wait_timeout` (обычно 8 часов). Без `SetConnMaxLifetime` возможны ошибки `invalid connection` при попытке использовать просроченное соединение.

**Исправление:**
```go
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(2 * time.Minute)
```

---

## Низкий приоритет / Code Quality (P3)

### 🔵 P3-1. `OptionalAuth` middleware не используется

**Файл:** `pkg/middleware/auth.go`

Определён, но нигде не применяется. Мёртвый код.

---

### 🔵 P3-2. `calculateEstimatedTime` делает лишние float-преобразования

**Файл:** `internal/handlers/task_handler.go`

```go
func calculateEstimatedTime(complexity, urgency, importance int) int {
    baseValue := float64(complexity + urgency + importance)  // преобразование в float
    estimatedMinutes := baseValue * 15
    // клиппинг с float64 ...
    return int(math.Round(estimatedMinutes))  // обратно в int
}
```

Все входные параметры — целые, умножение на 15 даёт целое. Преобразование в `float64` и обратно избыточно. Можно полностью в целых числах: `estimatedMinutes := (complexity + urgency + importance) * 15`.

---

### 🔵 P3-3. Дедкод в `CreateTask` — проверка на конфликт без действий

**Файл:** `internal/handlers/task_handler.go`

```go
if containsScheduleConflict(schedErr.Error()) {
    // Here we would notify the user that adjustments might be needed
    // For now, we'll return the warning and let the user handle it
}
```

Ветка не делает ничего сверх того, что уже делает `scheduleWarning`. Пустой if-блок — либо добавить реальную логику, либо убрать.

---

### 🔵 P3-4. `schedErrMsg` — избыточная обёртка

**Файл:** `internal/handlers/task_handler.go`

```go
func schedErrMsg(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
```

Это просто `err.Error()` с nil-проверкой. Можно инлайнить.

---

### 🔵 P3-5. Mixed receiver naming

В сервисах используются разные имена reciever'а: `s *Service` vs `s *SomeService`. В `TaskService` — `s`, в `UserService` — `s`, в `StatisticsService` — `s`, в `ScheduleService` — `s`. Это консистентно, но некоторые методы в том же сервисе используют `s`, что нормально. Ок.

---

### 🔵 P3-6. `scanSchedules` дублирует паттерн из `task_service.go`

`scanTasks` и `scanSchedules` — почти идентичные функции, отличающиеся только структурой. Можно сделать generic-функцию-сканер, но для Go <1.18 это нормально.

---

### 🔵 P3-7. `logger.Init()` не потокобезопасен

**Файл:** `pkg/logger/logger.go`

```go
var Logger *zap.Logger

func Init() error {
    config := zap.NewProductionConfig()
    // ...
    Logger, err = config.Build()  // ← гонка данных при конкурентном вызове
    // ...
}
```

В текущем использовании это не проблема (Init вызывается один раз в main), но пакет заявляет себя как переиспользуемый.

---

## Проблемы безопасности

### 🔒 SEC-1. JWT Secret по умолчанию

**Файл:** `internal/config/config.go`

```go
JWTSecret: getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
```

Если забудут установить `JWT_SECRET`, любой сможет подделать токен. Рекомендуется:
- Генерировать случайный секрет при старте с громким WARNING в логах
- Или падать с ошибкой, если используется значение по умолчанию в production-окружении

### 🔒 SEC-2. Нет rate-limiting на эндпоинтах аутентификации

`POST /api/v1/auth/register` и `POST /api/v1/auth/login` не имеют ограничений по частоте запросов. Это позволяет брутфорс-атаки на пароли и массовую регистрацию.

**Исправление:** Добавить rate-limiting middleware для auth-группы.

### 🔒 SEC-3. Нет ограничения размера тела запроса

Gin по умолчанию не ограничивает размер тела. Возможна атака на исчерпание памяти (отправка JSON большого размера).

**Исправление:**
```go
r.Use(gin.MaxBytesReader()) // или r.MaxMultipartMemory = 8 << 20
```

### 🔒 SEC-4. CORS разрешает все origins

**Файл:** `pkg/middleware/middleware.go`

```go
c.Header("Access-Control-Allow-Origin", "*")
```

Для продакшна с cookie-аутентификацией `*` недопустим. С JWT это менее критично, но всё равно стоит ограничить конкретными доменами.

### 🔒 SEC-5. Пароли в docker-compose.yml в открытом виде

```yaml
MYSQL_ROOT_PASSWORD: rootpassword
MYSQL_PASSWORD: deadnav_password
```

**Исправление:** Использовать `.env` файл или Docker secrets.

### 🔒 SEC-6. Порт MySQL открыт наружу

```yaml
ports:
  - "3306:3306"
```

В продакшне база данных не должна быть доступна снаружи Docker-сети. Достаточно internal network.

---

## Проблемы инфраструктуры / Docker

### 🐳 INF-1. Dockerfile: `go mod tidy` при каждой сборке

`go mod tidy` может изменить `go.mod`/`go.sum`, что сделает сборку невоспроизводимой. Лучше разделить:
```dockerfile
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build ...
```

### 🐳 INF-2. Dockerfile: HEALTHCHECK использует `wget`, но он не установлен

В финальном образе (`alpine:3.19`) добавлены только `ca-certificates` и `tzdata`. `wget` не установлен, поэтому HEALTHCHECK не сработает.

**Исправление:** Добавить `wget` в `RUN apk --no-cache add ca-certificates tzdata wget`, либо использовать встроенный `wget` из busybox, который есть в alpine по умолчанию. На самом деле в Alpine busybox `wget` доступен, так что это может работать — но не guaranteed.

### 🐳 INF-3. docker-compose: `depends_on` с `condition: service_healthy`

`depends_on` с `condition` не работает в `docker compose up` (работает только в swarm mode или с `docker-compose` v2). Новый `docker compose` (без дефиса) эту опцию игнорирует. Нужно использовать `depends_on:` без condition + retry logic в приложении, или `healthcheck` + `start_period`.

### 🐳 INF-4. docker-compose: нет `docker-compose.prod.yml`, на который ссылается Makefile

---

## Рекомендации по архитектуре

1. **Передача `context.Context`**: Все методы сервисов, работающие с БД, должны принимать `context.Context` первым параметром. Это стандартная практика в Go для:
   - Таймаутов запросов
   - Трассировки (tracing)
   - Отмены при shutdown

2. **Интерфейсы для сервисов**: Handlers зависят от конкретных типов (`*services.TaskService`). Использование интерфейсов упростило бы тестирование (mocking).

3. **Консистентность ошибок**: Выбрать единый подход:
   - Либо везде оборачивать `sql.ErrNoRows` с `%w`
   - Либо везде возвращать кастомные ошибки типа `ErrNotFound`

4. **Миграции БД**: Вместо `init.sql` (который срабатывает только при первом создании контейнера), использовать инструмент миграций (`golang-migrate`, `goose`, и т.д.).

5. **Конфигурация через файл**: Рассмотреть добавление YAML/JSON конфига как альтернативу только ENV-переменным — это удобнее для сложных конфигураций.

6. **Тесты**: Проект не содержит ни одного теста (`.dockerignore` исключает `*_test.go`). Нужно добавить unit-тесты для сервисов и integration-тесты для handlers.

---

## Сводная таблица

| ID | Серьёзность | Файл | Проблема |
|----|-------------|------|----------|
| P0-1 | 🔴 Крит. | `user_service.go:218-240` | `GetUserByID` никогда не заполняет `TelegramID` |
| P0-2 | 🔴 Крит. | `user_service.go:169-172` | `string(rune(...))` вместо `strconv.FormatInt` |
| P0-3 | 🔴 Крит. | `go.mod` | Отсутствует `golang.org/x/crypto` — не скомпилируется |
| P0-4 | 🔴 Крит. | `schedule_service.go` | Race condition при авто-планировании |
| P0-5 | 🔴 Крит. | `common.go` | `mustUserID` молча возвращает 0 при отсутствии middleware |
| P1-1 | 🟠 Выс. | `user_service.go` | `telegramID` не сканируется → UPDATE при каждом входе |
| P1-2 | 🟠 Выс. | `cmd/server/main.go` | Мёртвый код, конфликт порта :8080 |
| P1-3 | 🟠 Выс. | `Dockerfile` | `git describe` не сработает в Docker |
| P1-4 | 🟠 Выс. | `cmd/api/main.go` | Нет graceful shutdown |
| P1-5 | 🟠 Выс. | `cmd/api/main.go` | Нет `logger.Sync()` |
| P1-6 | 🟠 Выс. | Все handlers | Сырые ошибки БД возвращаются клиенту |
| P2-1 | 🟡 Сред. | `Makefile` | Пароли БД в открытом виде |
| P2-2 | 🟡 Сред. | `Makefile` | `docker-compose.prod.yml` не существует |
| P2-3 | 🟡 Сред. | `models/task.go` | Два поля для длительности |
| P2-4 | 🟡 Сред. | `handlers/` | Нет валидации полей задач |
| P2-5 | 🟡 Сред. | `handlers/task_handler.go` | `EstimatedTime` не пересчитывается при Update |
| P2-6 | 🟡 Сред. | `user_service.go` | Неконсистентная обработка `sql.ErrNoRows` |
| P2-7 | 🟡 Сред. | Все сервисы | Нет `context.Context` для запросов к БД |
| P2-8 | 🟡 Сред. | `schedule_service.go` | `sql.ErrNoRows` обёрнут неконсистентно |
| P2-9 | 🟡 Сред. | `database/mysql.go` | Нет `ConnMaxLifetime` |
| SEC-1 | 🔒 Безоп. | `config/config.go` | JWT Secret по умолчанию |
| SEC-2 | 🔒 Безоп. | `cmd/api/main.go` | Нет rate-limiting на auth |
| SEC-3 | 🔒 Безоп. | Все handlers | Нет ограничения размера тела |
| SEC-4 | 🔒 Безоп. | `middleware/middleware.go` | CORS `*` на все origins |
| SEC-5 | 🔒 Безоп. | `docker-compose.yml` | Пароли в открытом виде |
| SEC-6 | 🔒 Безоп. | `docker-compose.yml` | Порт MySQL открыт наружу |
| INF-1 | 🐳 Инфра. | `Dockerfile` | `go mod tidy` при каждой сборке |
| INF-2 | 🐳 Инфра. | `Dockerfile` | HEALTHCHECK: `wget` может отсутствовать |
| INF-3 | 🐳 Инфра. | `docker-compose.yml` | `depends_on condition` не работает в compose v3 |
| INF-4 | 🐳 Инфра. | `Makefile` | Ссылка на несуществующий `docker-compose.prod.yml` |

---

> **Приоритет исправлений:** P0 → P1 → SEC → P2 → INF → P3
