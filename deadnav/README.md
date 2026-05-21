# Deadnav — Планировщик задач с календарём

> **Deadnav** — это REST API на Go для управления задачами с интеллектуальным автоматическим планированием. Система находит свободные временные слоты в рабочем календаре пользователя и вставляет задачи туда, учитывая рабочие часы, рабочие дни, часовой пояс и приоритет.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql)](https://www.mysql.com/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

---

## Содержание

- [Особенности](#особенности)
- [Быстрый старт](#быстрый-старт)
  - [Docker (рекомендуется)](#docker-рекомендуется)
  - [Локальная разработка](#локальная-разработка)
- [Переменные окружения](#переменные-окружения)
- [API Reference](#api-reference)
  - [Аутентификация](#аутентификация)
  - [Задачи](#задачи)
  - [Расписание / Календарь](#расписание--календарь)
  - [Настройки планировщика](#настройки-планировщика)
  - [Статистика](#статистика)
  - [Healthcheck](#healthcheck)
- [Алгоритм планирования](#алгоритм-планирования)
- [Статистика: описание полей](#статистика-описание-полей)
- [Структура проекта](#структура-проекта)
- [База данных](#база-данных)
- [Продакшн-деплой](#продакшн-деплой)
  - [Docker Compose (prod)](#docker-compose-prod)
  - [Nginx reverse proxy](#nginx-reverse-proxy)
  - [Makefile](#makefile)
- [Расширение API](#расширение-api)

---

## Особенности

- **JWT-аутентификация** — регистрация, логин по username/email, интеграция с Telegram Bot
- **CRUD задач** — создание, чтение, обновление, удаление с полным жизненным циклом статусов
- **Автопланирование** — при создании/обновлении задачи система автоматически ищет первый подходящий слот в рабочем расписании
- **Управление предпочтениями** — рабочие часы, рабочие дни, минимальная длина слота, часовой пояс (IANA)
- **Статистика** — детальная аналитика по задачам, включая `productivity_score` от 0 до 100
- **Многопользовательский режим** — полная изоляция данных между пользователями
- **Docker-ready** — отдельные конфиги для разработки и продакшна
- **Non-root контейнер** — API запускается от unprivileged пользователя
- **Gin + Zap** — высокопроизводительный HTTP-роутер и структурированный логгер

---

## Быстрый старт

### Docker (рекомендуется)

#### Разработка

```deadnav/deadnav/docker-compose.yml#L1-5
version: '3.8'
# ...
```

Запуск одной командой:

```deadnav/deadnav/Makefile#L1-4
.PHONY: help dev prod build clean logs stop restart
```

```/dev/null/shell.sh#L1-2
git clone https://github.com/your-org/deadnav.git
cd deadnav
make dev
```

После старта API доступен на `http://localhost:8080`.

#### Проверка работы

```/dev/null/shell.sh#L1-1
curl http://localhost:8080/health
```

Ожидаемый ответ:

```/dev/null/response.json#L1-3
{
  "status": "ok"
}
```

---

### Локальная разработка

**Требования:**
- Go 1.25+
- MySQL 8.0+
- `git`

**1. Клонирование и установка зависимостей**

```/dev/null/shell.sh#L1-3
git clone https://github.com/your-org/deadnav.git
cd deadnav
go mod download
```

**2. Настройка базы данных**

Выполните SQL-схему из `scripts/init.sql`:

```/dev/null/shell.sh#L1-1
mysql -u root -p < scripts/init.sql
```

**3. Переменные окружения**

Создайте файл `.env` в корне проекта (или задайте переменные в shell):

```/dev/null/.env.example#L1-9
SERVER_PORT=8080
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password
DB_NAME=deadnav
JWT_SECRET=your-super-secret-key-at-least-32-chars
JWT_EXPIRATION_HOURS=24
TELEGRAM_BOT_TOKEN=
```

**4. Запуск**

```/dev/null/shell.sh#L1-1
go run ./cmd/api
```

**5. Запуск тестов**

```/dev/null/shell.sh#L1-1
go test ./...
```

---

## Переменные окружения

| Переменная | По умолчанию | Обязательная | Описание |
|---|---|:---:|---|
| `SERVER_PORT` | `8080` | — | TCP-порт HTTP-сервера |
| `DB_HOST` | `localhost` | — | Хост MySQL |
| `DB_PORT` | `3306` | — | Порт MySQL |
| `DB_USER` | `root` | — | Пользователь MySQL |
| `DB_PASSWORD` | `password` | — | Пароль MySQL |
| `DB_NAME` | `deadnav` | — | Имя базы данных |
| `JWT_SECRET` | — | **да** | Секрет для подписи JWT (мин. 32 символа) |
| `JWT_EXPIRATION_HOURS` | `24` | — | Время жизни токена в часах |
| `TELEGRAM_BOT_TOKEN` | — | — | Токен Telegram-бота (для `/auth/telegram`) |

> **Безопасность:** никогда не коммитьте реальные значения `JWT_SECRET`, `DB_PASSWORD` и `TELEGRAM_BOT_TOKEN` в репозиторий.

---

## API Reference

Базовый URL: `http://localhost:8080`

### Соглашения

| Элемент | Описание |
|---|---|
| Все даты | RFC 3339 (`2024-06-15T09:00:00Z`) |
| Аутентификация | `Authorization: Bearer <token>` |
| Формат тела запроса | `application/json` |
| Формат ответа | `application/json` |
| Коды ошибок | `400` Bad Request, `401` Unauthorized, `403` Forbidden, `404` Not Found, `500` Internal Server Error |

---

### Аутентификация

Все эндпоинты аутентификации публичны (не требуют токена), **кроме** `GET /api/v1/auth/me`.

---

#### `POST /api/v1/auth/register` — Регистрация

Создаёт нового пользователя с локальной аутентификацией и возвращает JWT.

**Тело запроса:**

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `username` | string | **да** | Уникальное имя пользователя |
| `email` | string | **да** | Уникальный e-mail |
| `password` | string | **да** | Пароль (хранится bcrypt-хэшем) |

```/dev/null/curl-register.sh#L1-8
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "email": "alice@example.com",
    "password": "S3cur3P@ssw0rd"
  }'
```

**Ответ `201 Created`:**

```/dev/null/response-register.json#L1-14
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "auth_provider": "local",
    "created_at": "2024-06-15T09:00:00Z"
  }
}
```

---

#### `POST /api/v1/auth/login` — Вход

Аутентифицирует существующего пользователя по username **или** email.

**Тело запроса:**

| Поле | Тип | Описание |
|---|---|---|
| `username_or_email` | string | Username или e-mail |
| `password` | string | Пароль |

```/dev/null/curl-login.sh#L1-8
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username_or_email": "alice@example.com",
    "password": "S3cur3P@ssw0rd"
  }'
```

**Ответ `200 OK`:**

```/dev/null/response-login.json#L1-12
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "auth_provider": "local",
    "created_at": "2024-06-15T09:00:00Z"
  }
}
```

---

#### `POST /api/v1/auth/telegram` — Вход через Telegram

Создаёт или аутентифицирует пользователя через данные Telegram Login Widget. Сервер верифицирует подпись `hash` с помощью `TELEGRAM_BOT_TOKEN`.

**Тело запроса:**

| Поле | Тип | Описание |
|---|---|---|
| `telegram_id` | int64 | Числовой ID пользователя Telegram |
| `username` | string | Telegram username (может быть пустым) |
| `first_name` | string | Имя |
| `last_name` | string | Фамилия (опционально) |
| `auth_date` | int64 | Unix-время авторизации (от Telegram) |
| `hash` | string | HMAC-SHA256 подпись от Telegram |

```/dev/null/curl-telegram.sh#L1-13
curl -X POST http://localhost:8080/api/v1/auth/telegram \
  -H "Content-Type: application/json" \
  -d '{
    "telegram_id": 123456789,
    "username": "alice_tg",
    "first_name": "Alice",
    "last_name": "Smith",
    "auth_date": 1718445600,
    "hash": "a3f9d1c2e4b5678901abcdef1234567890abcdef1234567890abcdef12345678"
  }'
```

**Ответ `200 OK`:**

```/dev/null/response-telegram.json#L1-12
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 2,
    "username": "alice_tg",
    "email": "",
    "auth_provider": "telegram",
    "created_at": "2024-06-15T10:30:00Z"
  }
}
```

> Если `TELEGRAM_BOT_TOKEN` не задан, эндпоинт возвращает `503 Service Unavailable`.

---

#### `GET /api/v1/auth/me` — Текущий пользователь

Возвращает профиль авторизованного пользователя. **Требует Bearer-токен.**

```/dev/null/curl-me.sh#L1-3
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Ответ `200 OK`:**

```/dev/null/response-me.json#L1-9
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "auth_provider": "local",
  "created_at": "2024-06-15T09:00:00Z"
}
```

---

### Задачи

Все эндпоинты требуют заголовка `Authorization: Bearer <token>`.

---

#### `POST /api/v1/tasks` — Создать задачу

Создаёт задачу и **автоматически** запускает планировщик для поиска свободного слота в рабочем календаре.

**Тело запроса:**

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `title` | string | **да** | Название задачи |
| `description` | string | — | Описание |
| `status` | string | — | `pending` (по умолчанию), `in_progress`, `completed`, `cancelled` |
| `priority` | int | — | 1 (низкий) — 5 (высокий), по умолчанию `1` |
| `duration_minutes` | int | — | Длительность в минутах; `0` = вычислить из дат (ограничено 30–480 мин) |
| `start_date` | string | **да** | Начало окна планирования (RFC 3339) |
| `end_date` | string | **да** | Дедлайн задачи (RFC 3339) |

```/dev/null/curl-create-task.sh#L1-14
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Подготовить квартальный отчёт",
    "description": "Сводная таблица продаж Q2 2024",
    "status": "pending",
    "priority": 4,
    "duration_minutes": 120,
    "start_date": "2024-06-17T00:00:00Z",
    "end_date": "2024-06-21T18:00:00Z"
  }'
```

**Ответ `201 Created`:**

```/dev/null/response-create-task.json#L1-29
{
  "task": {
    "id": 42,
    "user_id": 1,
    "title": "Подготовить квартальный отчёт",
    "description": "Сводная таблица продаж Q2 2024",
    "status": "pending",
    "priority": 4,
    "duration_minutes": 120,
    "start_date": "2024-06-17T00:00:00Z",
    "end_date": "2024-06-21T18:00:00Z",
    "created_at": "2024-06-15T11:00:00Z",
    "updated_at": "2024-06-15T11:00:00Z"
  },
  "schedule": {
    "id": 17,
    "task_id": 42,
    "user_id": 1,
    "start_time": "2024-06-17T09:00:00Z",
    "end_time": "2024-06-17T11:00:00Z",
    "created_at": "2024-06-15T11:00:00Z"
  },
  "schedule_warning": ""
}
```

> Если алгоритм не нашёл подходящего слота, задача **всё равно сохраняется**, а поле `schedule_warning` содержит объяснение. Поле `schedule` при этом будет `null`.

---

#### `GET /api/v1/tasks` — Список задач

Возвращает все задачи авторизованного пользователя.

```/dev/null/curl-list-tasks.sh#L1-3
curl http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-list-tasks.json#L1-20
[
  {
    "id": 42,
    "user_id": 1,
    "title": "Подготовить квартальный отчёт",
    "status": "pending",
    "priority": 4,
    "duration_minutes": 120,
    "start_date": "2024-06-17T00:00:00Z",
    "end_date": "2024-06-21T18:00:00Z",
    "created_at": "2024-06-15T11:00:00Z",
    "updated_at": "2024-06-15T11:00:00Z"
  }
]
```

---

#### `GET /api/v1/tasks/:id` — Получить задачу

```/dev/null/curl-get-task.sh#L1-3
curl http://localhost:8080/api/v1/tasks/42 \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:** объект задачи (аналогично полю `task` в ответе создания).

**Ошибки:** `404` если задача не найдена или принадлежит другому пользователю.

---

#### `PUT /api/v1/tasks/:id` — Обновить задачу

Обновляет поля задачи и **перезапускает планировщик** (reschedule). Тело запроса идентично `POST /api/v1/tasks`.

```/dev/null/curl-update-task.sh#L1-14
curl -X PUT http://localhost:8080/api/v1/tasks/42 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Подготовить квартальный отчёт (финал)",
    "description": "Добавить раздел по расходам",
    "status": "in_progress",
    "priority": 5,
    "duration_minutes": 180,
    "start_date": "2024-06-17T00:00:00Z",
    "end_date": "2024-06-20T18:00:00Z"
  }'
```

**Ответ `200 OK`:**

```/dev/null/response-update-task.json#L1-13
{
  "message": "task updated successfully",
  "schedule": {
    "id": 17,
    "task_id": 42,
    "user_id": 1,
    "start_time": "2024-06-17T09:00:00Z",
    "end_time": "2024-06-17T12:00:00Z",
    "created_at": "2024-06-15T11:00:00Z"
  },
  "schedule_warning": ""
}
```

---

#### `DELETE /api/v1/tasks/:id` — Удалить задачу

Удаляет задачу **и её запись в расписании** (каскадное удаление).

```/dev/null/curl-delete-task.sh#L1-3
curl -X DELETE http://localhost:8080/api/v1/tasks/42 \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-delete-task.json#L1-3
{
  "message": "task deleted successfully"
}
```

---

### Расписание / Календарь

Все эндпоинты требуют заголовка `Authorization: Bearer <token>`.

---

#### `GET /api/v1/schedule` — Весь календарь

Возвращает все запланированные слоты текущего пользователя.

```/dev/null/curl-get-schedule.sh#L1-3
curl http://localhost:8080/api/v1/schedule \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-get-schedule.json#L1-14
[
  {
    "id": 17,
    "task_id": 42,
    "user_id": 1,
    "start_time": "2024-06-17T09:00:00Z",
    "end_time": "2024-06-17T11:00:00Z",
    "created_at": "2024-06-15T11:00:00Z"
  }
]
```

---

#### `GET /api/v1/schedule/free-slots` — Свободные слоты

Возвращает список незанятых временных окон в заданном диапазоне с учётом рабочих часов и рабочих дней пользователя.

**Query-параметры:**

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `from` | RFC 3339 | **да** | Начало диапазона поиска |
| `to` | RFC 3339 | **да** | Конец диапазона поиска |
| `duration` | int | — | Минимальная длина слота в минутах (по умолчанию `60`) |

```/dev/null/curl-free-slots.sh#L1-5
curl "http://localhost:8080/api/v1/schedule/free-slots\
?from=2024-06-17T00%3A00%3A00Z\
&to=2024-06-21T23%3A59%3A59Z\
&duration=90" \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-free-slots.json#L1-16
[
  {
    "start": "2024-06-17T11:00:00Z",
    "end": "2024-06-17T18:00:00Z"
  },
  {
    "start": "2024-06-18T09:00:00Z",
    "end": "2024-06-18T18:00:00Z"
  },
  {
    "start": "2024-06-19T09:00:00Z",
    "end": "2024-06-19T18:00:00Z"
  }
]
```

---

#### `GET /api/v1/schedule/task/:id` — Расписание задачи

Возвращает запись расписания для конкретной задачи.

```/dev/null/curl-get-task-schedule.sh#L1-3
curl http://localhost:8080/api/v1/schedule/task/42 \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:** объект расписания (аналогично полю `schedule` в ответе создания задачи).

**Ошибки:** `404` если расписание не найдено (задача не была запланирована).

---

#### `POST /api/v1/schedule/task/:id/reschedule` — Повторное планирование

Запускает алгоритм автопланирования заново для уже существующей задачи. Полезно, если появились свободные слоты после удаления других задач.

```/dev/null/curl-reschedule.sh#L1-3
curl -X POST http://localhost:8080/api/v1/schedule/task/42/reschedule \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-reschedule.json#L1-12
{
  "schedule": {
    "id": 17,
    "task_id": 42,
    "user_id": 1,
    "start_time": "2024-06-18T09:00:00Z",
    "end_time": "2024-06-18T11:00:00Z",
    "created_at": "2024-06-15T11:00:00Z"
  },
  "schedule_warning": ""
}
```

---

#### `DELETE /api/v1/schedule/task/:id` — Снять задачу с расписания

Удаляет запись в расписании. **Сама задача остаётся** — только исключается из календаря.

```/dev/null/curl-delete-schedule.sh#L1-3
curl -X DELETE http://localhost:8080/api/v1/schedule/task/42 \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-delete-schedule.json#L1-3
{
  "message": "schedule removed successfully"
}
```

---

### Настройки планировщика

Все эндпоинты требуют заголовка `Authorization: Bearer <token>`.

Настройки создаются автоматически при первом вызове `PUT`. `GET` всегда возвращает актуальные значения (или системные дефолты).

---

#### `GET /api/v1/preferences` — Получить настройки

```/dev/null/curl-get-prefs.sh#L1-3
curl http://localhost:8080/api/v1/preferences \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-get-prefs.json#L1-9
{
  "user_id": 1,
  "work_start_hour": 9,
  "work_end_hour": 18,
  "work_days": "Mon,Tue,Wed,Thu,Fri",
  "min_slot_minutes": 30,
  "timezone": "UTC"
}
```

**Поля ответа:**

| Поле | Тип | Описание |
|---|---|---|
| `work_start_hour` | int | Начало рабочего дня (0–23) |
| `work_end_hour` | int | Конец рабочего дня (0–23) |
| `work_days` | string | Рабочие дни через запятую: `Mon`, `Tue`, `Wed`, `Thu`, `Fri`, `Sat`, `Sun` |
| `min_slot_minutes` | int | Минимальная длина временного слота |
| `timezone` | string | Часовой пояс в формате IANA (`Europe/Moscow`, `America/New_York`, и т.д.) |

---

#### `PUT /api/v1/preferences` — Обновить настройки

```/dev/null/curl-put-prefs.sh#L1-12
curl -X PUT http://localhost:8080/api/v1/preferences \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "work_start_hour": 10,
    "work_end_hour": 19,
    "work_days": "Mon,Tue,Wed,Thu,Fri",
    "min_slot_minutes": 30,
    "timezone": "Europe/Moscow"
  }'
```

**Ответ `200 OK`:** обновлённый объект настроек (аналогично `GET`).

> **Важно:** после изменения настроек существующие расписания **не пересчитываются** автоматически. Используйте `POST /api/v1/schedule/task/:id/reschedule` для обновления конкретных задач.

---

### Статистика

Требует заголовка `Authorization: Bearer <token>`.

---

#### `GET /api/v1/statistics` — Статистика пользователя

Возвращает полную аналитику по всем задачам авторизованного пользователя.

```/dev/null/curl-stats.sh#L1-3
curl http://localhost:8080/api/v1/statistics \
  -H "Authorization: Bearer <token>"
```

**Ответ `200 OK`:**

```/dev/null/response-stats.json#L1-21
{
  "total_tasks": 15,
  "completed_tasks": 8,
  "pending_tasks": 4,
  "in_progress_tasks": 2,
  "cancelled_tasks": 1,
  "overdue_tasks": 2,
  "upcoming_deadlines": 3,
  "avg_delay_hours": 1.5,
  "on_time_completion_rate": 0.875,
  "avg_duration_hours": 2.3,
  "median_duration_hours": 2.0,
  "min_duration_hours": 0.5,
  "max_duration_hours": 8.0,
  "tasks_created_this_week": 5,
  "tasks_completed_this_week": 3,
  "completion_trend": "improving",
  "productivity_score": 72
}
```

Подробное описание полей — в разделе [Статистика: описание полей](#статистика-описание-полей).

---

### Healthcheck

#### `GET /health` — Статус сервиса

Публичный эндпоинт, не требует аутентификации. Используется Docker, Kubernetes и балансировщиками нагрузки для проверки живости сервиса.

```/dev/null/curl-health.sh#L1-1
curl http://localhost:8080/health
```

**Ответ `200 OK`:**

```/dev/null/response-health.json#L1-3
{
  "status": "ok"
}
```

---

## Алгоритм планирования

Алгоритм запускается автоматически при **создании** (`POST /api/v1/tasks`) и **обновлении** (`PUT /api/v1/tasks/:id`) задачи, а также вручную через `POST /api/v1/schedule/task/:id/reschedule`.

```/dev/null/algorithm.txt#L1-14
1. Загрузка предпочтений пользователя
   └─ рабочие часы, рабочие дни, часовой пояс

2. Вычисление длительности задачи
   ├─ если duration_minutes > 0  →  использовать как есть
   └─ иначе                      →  end_date − start_date, ограничено [30, 480] мин

3. Загрузка всех существующих расписаний
   └─ диапазон: сейчас .. дедлайн задачи (end_date)

4. Итерация по рабочим дням
   ├─ начало: max(now, start_date)
   ├─ конец:  end_date
   ├─ пропустить нерабочие дни (по work_days)
   ├─ ограничить поиск рамками work_start_hour .. work_end_hour
   ├─ при нахождении пересечения с существующим слотом →
   │    переместить курсор на конец блокирующего события
   └─ при нахождении свободного промежутка ≥ duration → записать слот

5. Сохранение расписания
   └─ INSERT ... ON DUPLICATE KEY UPDATE (upsert по task_id)

6. Результат
   ├─ слот найден     →  schedule заполнен, schedule_warning = ""
   └─ слот не найден  →  задача сохранена, schedule = null,
                          schedule_warning содержит описание причины
```

**Пример:** задача на 2 часа, дедлайн — пятница. Если понедельник–вторник заняты другими задачами, алгоритм найдёт первый двухчасовой просвет начиная со среды в рамках рабочего времени.

---

## Статистика: описание полей

| Поле | Тип | Описание |
|---|---|---|
| `total_tasks` | int | Всего задач |
| `completed_tasks` | int | Завершённые (`completed`) |
| `pending_tasks` | int | Ожидающие (`pending`) |
| `in_progress_tasks` | int | В процессе (`in_progress`) |
| `cancelled_tasks` | int | Отменённые (`cancelled`) |
| `overdue_tasks` | int | Просроченные: `end_date` в прошлом и статус не `completed` |
| `upcoming_deadlines` | int | Дедлайн в ближайшие 24 часа |
| `avg_delay_hours` | float | Среднее опоздание выполнения (часов) |
| `on_time_completion_rate` | float | Доля задач, выполненных в срок (0.0–1.0) |
| `avg_duration_hours` | float | Средняя длительность задачи (часов) |
| `median_duration_hours` | float | Медианная длительность (часов) |
| `min_duration_hours` | float | Минимальная длительность (часов) |
| `max_duration_hours` | float | Максимальная длительность (часов) |
| `tasks_created_this_week` | int | Задач создано за текущую неделю |
| `tasks_completed_this_week` | int | Задач завершено за текущую неделю |
| `completion_trend` | string | Тренд: `improving`, `stable`, `declining` |
| `productivity_score` | int | Интегральная оценка продуктивности 0–100 |

### Формула `productivity_score`

```/dev/null/score.txt#L1-7
productivity_score (0–100) =
  40 pts  × completion_rate          (доля завершённых задач)
  25 pts  × on_time_completion_rate  (доля выполненных в срок)
  15 pts  × no_overdue_bonus         (15 если overdue_tasks = 0, иначе 0)
  10 pts  × trend_bonus              (10 если trend = "improving", 5 если "stable")
  10 pts  × high_priority_completion (доля завершённых задач с priority ≥ 4)
```

---

## Структура проекта

```deadnav/deadnav/cmd/api/main.go#L1-1
// Entry point
```

```/dev/null/tree.txt#L1-32
deadnav/
├── cmd/
│   └── api/
│       └── main.go                   # Точка входа: инициализация, маршрутизация, запуск сервера
├── internal/
│   ├── config/
│   │   └── config.go                 # Загрузка конфигурации из переменных окружения
│   ├── database/
│   │   └── mysql.go                  # Подключение к MySQL, пул соединений
│   ├── handlers/                     # HTTP-обработчики (Gin)
│   │   ├── auth_handler.go           # /api/v1/auth/*
│   │   ├── common.go                 # Вспомогательные типы, хелперы ответов
│   │   ├── preferences_handler.go    # /api/v1/preferences
│   │   ├── schedule_handler.go       # /api/v1/schedule/*
│   │   ├── statistics_handler.go     # /api/v1/statistics
│   │   └── task_handler.go           # /api/v1/tasks/*
│   ├── models/                       # Структуры данных (модели БД)
│   │   ├── schedule.go
│   │   ├── statistics.go
│   │   ├── task.go
│   │   └── user.go
│   └── services/                     # Бизнес-логика
│       ├── preferences_service.go    # CRUD предпочтений
│       ├── schedule_service.go       # Алгоритм автопланирования
│       ├── statistics_service.go     # Вычисление аналитики
│       ├── task_service.go           # CRUD задач
│       └── user_service.go           # Аутентификация, JWT, Telegram
├── pkg/
│   ├── logger/
│   │   └── logger.go                 # Настройка Zap (structured logging)
│   └── middleware/
│       ├── auth.go                   # JWT middleware (Bearer token)
│       └── middleware.go             # CORS, request logging, panic recovery
├── scripts/
│   └── init.sql                      # DDL-схема: таблицы users, tasks, schedules, preferences
├── Dockerfile                        # Multi-stage build (Go builder + Alpine runtime)
├── docker-compose.yml                # Окружение разработки
├── docker-compose.prod.yml           # Продакшн-окружение
├── nginx.conf                        # Пример конфигурации Nginx reverse proxy
├── Makefile                          # Команды управления
├── go.mod
└── go.sum
```

### Зависимости

| Пакет | Версия | Назначение |
|---|---|---|
| `github.com/gin-gonic/gin` | v1.9.1 | HTTP-роутер и фреймворк |
| `github.com/go-sql-driver/mysql` | v1.7.1 | MySQL-драйвер |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT генерация и верификация |
| `github.com/joho/godotenv` | v1.5.1 | Загрузка `.env`-файлов |
| `go.uber.org/zap` | v1.26.0 | Структурированное логирование |

---

## База данных

Схема инициализируется автоматически при первом запуске контейнера через `scripts/init.sql`.

### Таблицы

#### `users`

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT PK | Автоинкремент |
| `username` | VARCHAR(100) UNIQUE | Уникальное имя |
| `email` | VARCHAR(255) UNIQUE | Уникальный e-mail |
| `password_hash` | VARCHAR(255) | bcrypt-хэш (NULL для Telegram-пользователей) |
| `telegram_id` | BIGINT UNIQUE | ID в Telegram (NULL для local) |
| `auth_provider` | ENUM | `local` или `telegram` |
| `created_at` | TIMESTAMP | Время создания |

#### `user_preferences`

| Колонка | Тип | По умолчанию | Описание |
|---|---|---|---|
| `user_id` | BIGINT PK FK | — | Ссылка на `users.id` (CASCADE DELETE) |
| `work_start_hour` | TINYINT | `9` | Начало рабочего дня |
| `work_end_hour` | TINYINT | `18` | Конец рабочего дня |
| `work_days` | VARCHAR(27) | `Mon,Tue,Wed,Thu,Fri` | Рабочие дни |
| `min_slot_minutes` | INT | `30` | Мин. длина слота |
| `timezone` | VARCHAR(50) | `UTC` | Часовой пояс (IANA) |

#### `tasks`

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT PK | Автоинкремент |
| `user_id` | BIGINT FK | Владелец задачи (CASCADE DELETE) |
| `title` | VARCHAR(255) | Название |
| `description` | TEXT | Описание |
| `status` | ENUM | `pending`, `in_progress`, `completed`, `cancelled` |
| `priority` | TINYINT | 1–5 (CHECK constraint) |
| `duration_minutes` | INT UNSIGNED | 0 = авто-вычисление из дат |
| `start_date` | DATETIME | Начало окна планирования |
| `end_date` | DATETIME | Дедлайн |
| `created_at` / `updated_at` | TIMESTAMP | Метки времени |

#### `schedules`

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT PK | Автоинкремент |
| `task_id` | BIGINT FK UNIQUE | Ровно одна запись на задачу (CASCADE DELETE) |
| `user_id` | BIGINT FK | Денормализованный для быстрых запросов |
| `start_time` | DATETIME | Начало запланированного слота |
| `end_time` | DATETIME | Конец запланированного слота |
| `created_at` | TIMESTAMP | Время создания записи |

#### `task_statistics` (VIEW)

Удобное представление для агрегации задач по пользователям: `total_tasks`, `completed_tasks`, `active_tasks`, `avg_duration_hours`.

---

## Продакшн-деплой

### Docker Compose (prod)

**1. Создайте файл `.env`:**

```/dev/null/.env.prod#L1-8
DB_ROOT_PASSWORD=VeryStr0ngRootPass!
DB_USER=deadnav_user
DB_PASSWORD=VeryStr0ngDbPass!
DB_NAME=deadnav
JWT_SECRET=your-super-secret-jwt-key-minimum-32-characters-long
JWT_EXPIRATION_HOURS=24
TELEGRAM_BOT_TOKEN=1234567890:ABCdef...
```

**2. Запустите продакшн-стек:**

```/dev/null/shell.sh#L1-1
make prod
```

Или напрямую:

```/dev/null/shell.sh#L1-1
docker compose -f docker-compose.prod.yml up -d
```

**3. Проверьте статус:**

```/dev/null/shell.sh#L1-2
docker compose -f docker-compose.prod.yml ps
make health
```

**4. Просмотр логов:**

```/dev/null/shell.sh#L1-2
make prod-logs          # все сервисы
make prod-logs-api      # только API
```

#### Особенности продакшн-конфигурации

- MySQL слушает только на `127.0.0.1:3306` (не публичный порт)
- Оба контейнера изолированы в сети `deadnav_internal` (bridge)
- API-контейнер запускается от непривилегированного пользователя (`uid=1001`)
- Resource limits: MySQL — 1 ГБ RAM / 2 CPU; API — 512 МБ RAM / 1 CPU
- Логи ротируются: API — 5 файлов × 50 МБ, MySQL — 3 файла × 10 МБ
- Встроенный healthcheck: `/health` каждые 15 с

---

### Nginx reverse proxy

Пример конфигурации находится в `nginx.conf`. Для активации:

```/dev/null/shell.sh#L1-4
sudo cp nginx.conf /etc/nginx/sites-available/deadnav
# Отредактируйте server_name и пути к SSL-сертификатам
sudo ln -s /etc/nginx/sites-available/deadnav /etc/nginx/sites-enabled/deadnav
sudo nginx -t && sudo systemctl reload nginx
```

**Ключевые особенности конфигурации:**

- Автоматический редирект HTTP → HTTPS
- TLS 1.2 / 1.3, современные шифры
- Rate limiting: 30 req/s с буфером 50 (per IP)
- Security headers: `X-Frame-Options`, `X-Content-Type-Options`, `HSTS`
- Gzip-сжатие JSON-ответов
- `keepalive 32` для upstream-соединений
- `/health` без rate limit и без access log

**SSL-сертификаты (Let's Encrypt):**

```/dev/null/shell.sh#L1-3
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
sudo systemctl enable certbot.timer
```

---

### Makefile

Доступные команды:

```/dev/null/make-help.txt#L1-20
dev              Запустить окружение разработки
dev-logs         Логи разработки
prod             Запустить продакшн (требует .env)
prod-logs        Логи продакшна
prod-logs-api    Логи только API-сервиса
logs             Псевдоним для dev-logs
stop             Остановить все контейнеры
restart          Перезапустить продакшн
clean            Остановить + удалить контейнеры и volumes
build            Пересобрать Docker-образы (без кэша)
db-shell         Открыть MySQL shell
db-backup        Создать дамп базы → backups/deadnav_YYYYMMDD_HHMMSS.sql
db-restore       Восстановить из дампа: make db-restore FILE=backups/...sql
health           Проверить /health
```

Пример резервного копирования и восстановления:

```/dev/null/shell.sh#L1-4
# Создать бэкап
make db-backup

# Восстановить конкретный бэкап
make db-restore FILE=backups/deadnav_20240615_120000.sql
```

---

## Расширение API

Проект построен по принципу **разделения слоёв**: handler → service → database. Добавление нового ресурса занимает четыре шага.

### Шаг 1 — Модель

Создайте файл `internal/models/notification.go`:

```deadnav/deadnav/internal/models/task.go#L1-5
// По аналогии с существующими моделями:
// type Task struct { ... }
// Добавьте новый тип:
// type Notification struct { ... }
```

```/dev/null/notification-model.go#L1-12
package models

import "time"

type Notification struct {
    ID        int64     `json:"id"`
    UserID    int64     `json:"user_id"`
    TaskID    int64     `json:"task_id"`
    Message   string    `json:"message"`
    SentAt    time.Time `json:"sent_at"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Шаг 2 — Сервис

Создайте файл `internal/services/notification_service.go` с методами `Create`, `ListByUser`, `Delete`. Инъектируйте `*sql.DB` через конструктор.

### Шаг 3 — Обработчик

Создайте файл `internal/handlers/notification_handler.go`:

```/dev/null/notification-handler.go#L1-18
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type NotificationHandler struct {
    service *services.NotificationService
}

func (h *NotificationHandler) List(c *gin.Context) {
    userID := c.GetInt64("user_id") // устанавливается JWT middleware
    items, err := h.service.ListByUser(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, items)
}
```

### Шаг 4 — Маршруты

Зарегистрируйте маршруты в `cmd/api/main.go` рядом с существующими группами:

```/dev/null/routes.go#L1-7
notifHandler := handlers.NewNotificationHandler(db)

authorized := router.Group("/api/v1")
authorized.Use(middleware.Auth(cfg.JWTSecret))
{
    authorized.GET("/notifications", notifHandler.List)
}
```

### Рекомендации при расширении

- **Валидация** — используйте теги `binding:"required"` в структурах запроса (Gin использует `go-playground/validator`)
- **Ошибки** — возвращайте стандартизированный `gin.H{"error": "..."}` через хелперы из `internal/handlers/common.go`
- **Логирование** — используйте `logger.Info(...)` / `logger.Error(...)` из `pkg/logger`
- **Транзакции** — для операций, затрагивающих несколько таблиц, оборачивайте в `db.Begin()` / `tx.Commit()`
- **Тестирование** — интерфейсы сервисов позволяют подменять реализацию mock-объектами в unit-тестах
- **Миграции** — при изменении схемы добавляйте `ALTER TABLE` в `scripts/init.sql` и ведите нумерованные миграции

---

## Лицензия

MIT License. Подробнее в файле [LICENSE](LICENSE).

---

*Разработано с использованием [Go](https://go.dev/), [Gin](https://github.com/gin-gonic/gin), [MySQL 8](https://www.mysql.com/) и [Zap](https://github.com/uber-go/zap).*
