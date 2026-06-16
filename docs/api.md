# DeadNav API

Полная документация по HTTP API проекта `deadnav`.

Документ включает:
- все обнаруженные эндпоинты;
- обязательные заголовки;
- path/query/body параметры;
- форматы request/response body;
- типовые коды ответов.

Структура БД намеренно опущена.

---

## Обзор сервисов

В репозитории есть два HTTP-сервиса:

1. **Основной API** на Go/Gin
   - базовый URL: `http://<host>:8080`
2. **Gamification service** на Flask
   - базовый URL: `http://<host>:8083`

> Если сервисы проксируются через `nginx` или API gateway, фактический внешний URL может отличаться.

---

## Форматы и соглашения

### Content-Type
Для всех запросов с JSON body используйте:

```http
Content-Type: application/json
```

### Аутентификация
Большинство эндпоинтов основного API требуют JWT:

```http
Authorization: Bearer <token>
```

Если заголовок отсутствует или имеет неверный формат, API возвращает `401 Unauthorized`.

### Формат дат
В основном API даты и время передаются в формате `RFC3339` / `ISO 8601`, например:

```text
2026-06-10T14:30:00Z
```

Для фильтров задач код использует `RFC3339Nano`, то есть обычный `RFC3339` тоже подходит.

### Типовые ответы

#### Ошибка
```json
{
  "error": "text"
}
```

#### Сообщение об успехе
```json
{
  "message": "text"
}
```

---

## Модели данных

Ниже перечислены JSON-структуры, которые используются в API.

### `User`

```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "telegram_id": 123456789,
  "vk_id": 999,
  "auth_provider": "local",
  "created_at": "2026-06-10T12:00:00Z"
}
```

Поля:

| Поле | Тип | Описание |
|---|---|---|
| `id` | integer | ID пользователя |
| `username` | string | Имя пользователя |
| `email` | string | Email |
| `telegram_id` | integer \| null | Telegram ID, если авторизация через Telegram |
| `vk_id` | integer \| null | VK ID, если используется |
| `auth_provider` | string | `local`, `telegram` или `vk` |
| `created_at` | string(datetime) | Дата создания |

---

### `AuthResponse`

```json
{
  "token": "jwt-token",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "auth_provider": "local",
    "created_at": "2026-06-10T12:00:00Z"
  }
}
```

| Поле | Тип | Описание |
|---|---|---|
| `token` | string | JWT токен |
| `user` | `User` | Данные пользователя |

---

### `Task`

```json
{
  "id": 10,
  "user_id": 1,
  "title": "Подготовить отчёт",
  "description": "Собрать еженедельную статистику",
  "status": "pending",
  "priority": 4,
  "duration_minutes": 120,
  "start_date": "2026-06-10T09:00:00Z",
  "end_date": "2026-06-12T18:00:00Z",
  "complexity": 3,
  "urgency": 4,
  "importance": 5,
  "estimated_minutes": 180,
  "created_at": "2026-06-10T08:00:00Z",
  "updated_at": "2026-06-10T08:00:00Z",
  "moved_deadline": false
}
```

Поля:

| Поле | Тип | Обязательное в запросе | Описание |
|---|---|---:|---|
| `id` | integer | нет | ID задачи |
| `user_id` | integer | нет | ID пользователя, подставляется сервером |
| `title` | string | да | Название, 1–255 символов |
| `description` | string | нет | Описание, до 2000 символов |
| `status` | string | да | `pending`, `in_progress`, `completed`, `cancelled` |
| `priority` | integer | да | Приоритет от 1 до 5 |
| `duration_minutes` | integer | нет | Длительность в минутах, от 0 до 480 |
| `start_date` | string(datetime) | да | Самое раннее время начала |
| `end_date` | string(datetime) | да | Дедлайн |
| `complexity` | integer | да | Сложность от 1 до 5 |
| `urgency` | integer | да | Срочность от 1 до 5 |
| `importance` | integer | да | Важность от 1 до 5 |
| `estimated_minutes` | integer | нет | Оценка времени; может вычисляться сервером |
| `created_at` | string(datetime) | нет | Время создания |
| `updated_at` | string(datetime) | нет | Время обновления |
| `moved_deadline` | boolean | нет | Был ли перенесён дедлайн |

Особенности:
- если `estimated_minutes = 0` при создании, сервер вычисляет значение по `complexity + urgency + importance`;
- в `PUT /tasks/{id}` `estimated_minutes` пересчитывается сервером заново;
- `start_date` должен быть меньше или равен `end_date`.

---

### `Schedule`

```json
{
  "id": 5,
  "task_id": 10,
  "user_id": 1,
  "title": "Подготовить отчёт",
  "status": "pending",
  "start_time": "2026-06-10T10:00:00Z",
  "end_time": "2026-06-10T12:00:00Z",
  "created_at": "2026-06-10T08:00:00Z"
}
```

| Поле | Тип | Описание |
|---|---|---|
| `id` | integer | ID записи расписания |
| `task_id` | integer | ID задачи |
| `user_id` | integer | ID пользователя |
| `title` | string | Заголовок слота |
| `status` | string | Статус связанной задачи |
| `start_time` | string(datetime) | Начало слота |
| `end_time` | string(datetime) | Конец слота |
| `created_at` | string(datetime) | Время создания |

---

### `ScheduleSlot`

```json
{
  "start_time": "2026-06-10T13:00:00Z",
  "end_time": "2026-06-10T14:30:00Z"
}
```

| Поле | Тип | Описание |
|---|---|---|
| `start_time` | string(datetime) | Начало свободного интервала |
| `end_time` | string(datetime) | Конец свободного интервала |

---

### `UserPreferences`

```json
{
  "user_id": 1,
  "work_start_hour": 9,
  "work_end_hour": 18,
  "work_days": "Mon,Tue,Wed,Thu,Fri",
  "min_slot_minutes": 30,
  "timezone": "UTC"
}
```

| Поле | Тип | Обязательное в запросе | Описание |
|---|---|---:|---|
| `user_id` | integer | нет | ID пользователя, устанавливается сервером |
| `work_start_hour` | integer | да | Час начала рабочего дня, 0–23 |
| `work_end_hour` | integer | да | Час окончания рабочего дня, 0–23 |
| `work_days` | string | да | Рабочие дни через запятую, например `Mon,Tue,Wed,Thu,Fri` |
| `min_slot_minutes` | integer | да | Минимальная длина слота в минутах |
| `timezone` | string | да | IANA timezone, например `UTC` или `Europe/Moscow` |

---

### `Statistics`

Полная статистика по задачам пользователя.

```json
{
  "total_tasks": 150,
  "completed_tasks": 85,
  "pending_tasks": 45,
  "in_progress_tasks": 20,
  "cancelled_tasks": 0,
  "tasks_by_status": {
    "pending": 45,
    "in_progress": 20,
    "completed": 85,
    "cancelled": 0
  },
  "tasks_by_priority": {
    "1": 15,
    "2": 25,
    "3": 40,
    "4": 50,
    "5": 20
  },
  "overdue_tasks": 10,
  "upcoming_deadlines": 15,
  "avg_delay_hours": 2.5,
  "on_time_completion_rate": 0.75,
  "avg_duration_hours": 4.8,
  "median_duration_hours": 3.5,
  "min_duration_hours": 0.5,
  "max_duration_hours": 24.0,
  "avg_duration_by_priority": {
    "1": 1.2,
    "2": 2.8,
    "3": 4.0,
    "4": 6.5,
    "5": 12.0
  },
  "tasks_created_this_week": 12,
  "tasks_completed_this_week": 8,
  "tasks_created_last_week": 8,
  "tasks_completed_last_week": 10,
  "completion_trend": "improving",
  "high_priority_tasks": 70,
  "low_priority_tasks": 40,
  "high_priority_completion_rate": 0.68,
  "low_priority_completion_rate": 0.85,
  "avg_tasks_per_day": 5.0,
  "peak_day": "Wednesday",
  "tasks_by_day_of_week": {
    "Monday": 20,
    "Tuesday": 25,
    "Wednesday": 30,
    "Thursday": 28,
    "Friday": 22,
    "Saturday": 10,
    "Sunday": 15
  },
  "total_users": 0,
  "active_users": 0,
  "new_users_this_month": 0,
  "productivity_score": 78.5
}
```

Поля:

| Поле | Тип |
|---|---|
| `total_tasks` | integer |
| `completed_tasks` | integer |
| `pending_tasks` | integer |
| `in_progress_tasks` | integer |
| `cancelled_tasks` | integer |
| `tasks_by_status` | object<string, integer> |
| `tasks_by_priority` | object<number, integer> |
| `overdue_tasks` | integer |
| `upcoming_deadlines` | integer |
| `avg_delay_hours` | number |
| `on_time_completion_rate` | number |
| `avg_duration_hours` | number |
| `median_duration_hours` | number |
| `min_duration_hours` | number |
| `max_duration_hours` | number |
| `avg_duration_by_priority` | object<number, number> |
| `tasks_created_this_week` | integer |
| `tasks_completed_this_week` | integer |
| `tasks_created_last_week` | integer |
| `tasks_completed_last_week` | integer |
| `completion_trend` | string |
| `high_priority_tasks` | integer |
| `low_priority_tasks` | integer |
| `high_priority_completion_rate` | number |
| `low_priority_completion_rate` | number |
| `avg_tasks_per_day` | number |
| `peak_day` | string |
| `tasks_by_day_of_week` | object<string, integer> |
| `total_users` | integer |
| `active_users` | integer |
| `new_users_this_month` | integer |
| `productivity_score` | number |

---

### `MonthlyStatistics`

```json
{
  "month": "2026-06-01T00:00:00Z",
  "total_tasks": 42,
  "completed_tasks": 35,
  "overdue_completed": 8,
  "moved_deadlines": 3,
  "heatmap": [
    {
      "date": "2026-06-01T00:00:00Z",
      "value": 0.8
    }
  ]
}
```

| Поле | Тип | Описание |
|---|---|---|
| `month` | string(datetime) | Первый день месяца; для `POST/PUT` значение из body игнорируется и заменяется `month` query-параметром |
| `total_tasks` | integer | Всего задач за месяц |
| `completed_tasks` | integer | Выполнено задач |
| `overdue_completed` | integer | Выполнено после дедлайна |
| `moved_deadlines` | integer | Количество переносов дедлайнов |
| `heatmap` | array of `HeatmapDay` | Дневная тепловая карта |

#### `HeatmapDay`

```json
{
  "date": "2026-06-01T00:00:00Z",
  "value": 0.8
}
```

| Поле | Тип | Описание |
|---|---|---|
| `date` | string(datetime) | День |
| `value` | number | Значение от `0` до `1` |

---

## Основной API (Go)

### 1. Health check

#### `GET /health`

Проверка доступности сервиса.

**Авторизация:** не требуется.

##### Response `200 OK`
```json
{
  "status": "ok"
}
```

---

## Auth

### 2. Регистрация

#### `POST /api/v1/auth/register`

Создаёт нового пользователя с локальной авторизацией.

**Авторизация:** не требуется.

##### Request body
```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "secret123"
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `username` | string | да | Имя пользователя |
| `email` | string | да | Email |
| `password` | string | да | Пароль, минимум 6 символов |

##### Response `200 OK`
```json
{
  "token": "jwt-token",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "auth_provider": "local",
    "created_at": "2026-06-10T12:00:00Z"
  }
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "недопустимый формат запроса"
}
```

или, например:

```json
{
  "error": "требуются имя пользователя, электронная почта и пароль"
}
```

```json
{
  "error": "пароль должен содержать не менее 6 символов"
}
```

```json
{
  "error": "имя пользователя или электронная почта уже существуют"
}
```

---

### 3. Вход по логину/email и паролю

#### `POST /api/v1/auth/login`

**Авторизация:** не требуется.

##### Request body
```json
{
  "username_or_email": "alice",
  "password": "secret123"
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `username_or_email` | string | да | Логин или email |
| `password` | string | да | Пароль |

##### Response `200 OK`
```json
{
  "token": "jwt-token",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "auth_provider": "local",
    "created_at": "2026-06-10T12:00:00Z"
  }
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "недопустимый формат запроса"
}
```

- `401 Unauthorized`
```json
{
  "error": "invalid credentials"
}
```

```json
{
  "error": "this account uses Telegram login, please use Telegram auth"
}
```

```json
{
  "error": "требуются имя пользователя/электронная почта и пароль"
}
```

---

### 4. Вход через Telegram

#### `POST /api/v1/auth/telegram`

Логинит пользователя через Telegram или создаёт нового, если такого ещё нет.

**Авторизация:** не требуется.

##### Request body
```json
{
  "telegram_id": 123456789,
  "username": "alice_tg",
  "first_name": "Alice",
  "last_name": "Doe",
  "auth_date": 1718000000,
  "hash": "telegram-hash"
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `telegram_id` | integer | да | Telegram user ID |
| `username` | string | да | Username из Telegram |
| `first_name` | string | нет | Имя |
| `last_name` | string | нет | Фамилия |
| `auth_date` | integer | нет | Время авторизации |
| `hash` | string | нет | Telegram hash |

> В текущей реализации хендлер принимает все эти поля, но сервис фактически проверяет только `telegram_id` и `username`.

##### Response `200 OK`
```json
{
  "token": "jwt-token",
  "user": {
    "id": 2,
    "username": "alice_tg",
    "email": "",
    "telegram_id": 123456789,
    "auth_provider": "telegram",
    "created_at": "2026-06-10T12:00:00Z"
  }
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "недопустимый формат запроса"
}
```

- `401 Unauthorized`
```json
{
  "error": "telegram_id and username are required"
}
```

---

### 5. Вход через VK Mini App

#### `POST /api/v1/auth/vk`

Логинит пользователя через VK Mini App по подписанным `launch_params`.
Новый пользователь создаётся при первом обращении (`auth_provider = "vk"`).
Никаких редиректов и обмена кодов — клиент Mini App отдаёт
`launch_params`, сервер проверяет подпись HMAC-SHA256 секретом приложения
и выдаёт JWT.

**Авторизация:** не требуется.

##### Request body
```json
{
  "launch_params": "vk_user_id=12345&vk_app_id=51671647&vk_first_name=Alice&vk_last_name=Smith&sign=abcdef0123…"
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `launch_params` | string | да | Сырая query-строка `launch_params`, которую VK Mini App получает при запуске. Содержит `vk_user_id` и подпись `sign`. |

> Сервер проверяет подпись по [схеме VK Mini App](https://dev.vk.com/mini-apps/development/launch-params): из строки убирается ключ `sign`, оставшиеся пары `k=v` сортируются по ключу и склеиваются через `\n`, после чего считается HMAC-SHA256 с `VK_ID_CLIENT_SECRET` и сравнивается с `sign`.

##### Response `200 OK`
```json
{
  "token": "jwt-token",
  "user": {
    "id": 3,
    "username": "Alice_Smith",
    "email": "",
    "vk_id": 12345,
    "auth_provider": "vk",
    "created_at": "2026-06-10T12:00:00Z"
  }
}
```

- При **первом входе** создаётся новый пользователь. Имя берётся из
  `vk_first_name` + `vk_last_name`; если оба пустые — генерируется
  `vk_<id>`. При коллизии имени добавляется суффикс `_<id>`.
- При **повторном входе** возвращается тот же `user.id` и новый JWT.

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "недопустимый формат запроса"
}
```

```json
{
  "error": "launch_params is required"
}
```

- `401 Unauthorized` — не прошла проверка подписи или не заполнен
  `vk_user_id`:
```json
{
  "error": "launch_params: invalid signature"
}
```

```json
{
  "error": "launch_params: missing 'vk_user_id'"
}
```

---

### 6. Текущий пользователь

#### `GET /api/v1/auth/me`

Возвращает данные текущего пользователя.

**Авторизация:** требуется `Bearer` token.

##### Заголовки
```http
Authorization: Bearer <token>
```

##### Response `200 OK`
```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "auth_provider": "local",
  "created_at": "2026-06-10T12:00:00Z"
}
```

##### Ошибки

- `401 Unauthorized`
```json
{
  "error": "authorization header required"
}
```

```json
{
  "error": "invalid authorization format, use 'Bearer <token>'"
}
```

```json
{
  "error": "invalid or expired token"
}
```

или:

```json
{
  "error": "необходима авторизация"
}
```

---

## Tasks

Все эндпоинты ниже требуют:

```http
Authorization: Bearer <token>
```

### 7. Создать задачу

#### `POST /api/v1/tasks`

Создаёт задачу и пытается автоматически поставить её в расписание.

##### Request body
```json
{
  "title": "Подготовить отчёт",
  "description": "Собрать еженедельную статистику",
  "status": "pending",
  "priority": 4,
  "duration_minutes": 120,
  "start_date": "2026-06-10T09:00:00Z",
  "end_date": "2026-06-12T18:00:00Z",
  "complexity": 3,
  "urgency": 4,
  "importance": 5,
  "estimated_minutes": 0,
  "moved_deadline": false
}
```

Минимально необходимое тело:

```json
{
  "title": "Подготовить отчёт",
  "status": "pending",
  "priority": 4,
  "start_date": "2026-06-10T09:00:00Z",
  "end_date": "2026-06-12T18:00:00Z",
  "complexity": 3,
  "urgency": 4,
  "importance": 5
}
```

##### Response `201 Created`
```json
{
  "task": {
    "id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт",
    "description": "Собрать еженедельную статистику",
    "status": "pending",
    "priority": 4,
    "duration_minutes": 120,
    "start_date": "2026-06-10T09:00:00Z",
    "end_date": "2026-06-12T18:00:00Z",
    "complexity": 3,
    "urgency": 4,
    "importance": 5,
    "estimated_minutes": 180,
    "created_at": "2026-06-10T08:00:00Z",
    "updated_at": "2026-06-10T08:00:00Z",
    "moved_deadline": false
  },
  "schedule": {
    "id": 5,
    "task_id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт",
    "status": "pending",
    "start_time": "2026-06-10T10:00:00Z",
    "end_time": "2026-06-10T12:00:00Z",
    "created_at": "2026-06-10T08:00:00Z"
  },
  "schedule_warning": ""
}
```

Поля ответа:

| Поле | Тип | Описание |
|---|---|---|
| `task` | `Task` | Созданная задача |
| `schedule` | `Schedule` \| null | Автоматически назначенный слот, если удалось |
| `schedule_warning` | string | Предупреждение, если автопланирование не удалось |

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "неверный формат запроса"
}
```

- `401 Unauthorized` — типовые ошибки Bearer/JWT

---

### 8. Получить список задач

#### `GET /api/v1/tasks`

Возвращает все задачи пользователя. Поддерживает фильтры.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `status` | string | нет | Фильтр по статусу: `pending`, `in_progress`, `completed`, `cancelled` |
| `priority` | integer | нет | Фильтр по приоритету `1..5` |
| `start_date_from` | string(datetime) | нет | `start_date >= value` |
| `start_date_to` | string(datetime) | нет | `start_date <= value` |
| `end_date_from` | string(datetime) | нет | `end_date >= value` |
| `end_date_to` | string(datetime) | нет | `end_date <= value` |

##### Пример
```http
GET /api/v1/tasks?status=pending&priority=5&start_date_from=2026-06-01T00:00:00Z
Authorization: Bearer <token>
```

##### Response `200 OK`
```json
[
  {
    "id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт",
    "description": "Собрать еженедельную статистику",
    "status": "pending",
    "priority": 4,
    "duration_minutes": 120,
    "start_date": "2026-06-10T09:00:00Z",
    "end_date": "2026-06-12T18:00:00Z",
    "complexity": 3,
    "urgency": 4,
    "importance": 5,
    "estimated_minutes": 180,
    "created_at": "2026-06-10T08:00:00Z",
    "updated_at": "2026-06-10T08:00:00Z",
    "moved_deadline": false
  }
]
```

Если задач нет, возвращается пустой массив:

```json
[]
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "invalid filter: parsing time \"...\" as \"2006-01-02T15:04:05.999999999Z07:00\": ..."
}
```

---

### 9. Получить задачу по ID

#### `GET /api/v1/tasks/{id}`

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Response `200 OK`
```json
{
  "id": 10,
  "user_id": 1,
  "title": "Подготовить отчёт",
  "description": "Собрать еженедельную статистику",
  "status": "pending",
  "priority": 4,
  "duration_minutes": 120,
  "start_date": "2026-06-10T09:00:00Z",
  "end_date": "2026-06-12T18:00:00Z",
  "complexity": 3,
  "urgency": 4,
  "importance": 5,
  "estimated_minutes": 180,
  "created_at": "2026-06-10T08:00:00Z",
  "updated_at": "2026-06-10T08:00:00Z",
  "moved_deadline": false
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "invalid id \"abc\": must be a positive integer"
}
```

```json
{
  "error": "invalid id 0: must be positive"
}
```

- `404 Not Found`
```json
{
  "error": "задача не найдена"
}
```

---

### 10. Обновить задачу

#### `PUT /api/v1/tasks/{id}`

Обновляет задачу. После обновления сервер пытается автоматически пересчитать расписание.

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Request body
Формат тот же, что и у `Task`.

```json
{
  "title": "Подготовить отчёт v2",
  "description": "Обновлённое описание",
  "status": "in_progress",
  "priority": 5,
  "duration_minutes": 90,
  "start_date": "2026-06-11T09:00:00Z",
  "end_date": "2026-06-13T18:00:00Z",
  "complexity": 4,
  "urgency": 5,
  "importance": 5,
  "moved_deadline": true
}
```

##### Response `200 OK`
```json
{
  "message": "задача обновлена успешно",
  "schedule": {
    "id": 5,
    "task_id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт v2",
    "status": "in_progress",
    "start_time": "2026-06-11T10:00:00Z",
    "end_time": "2026-06-11T11:30:00Z",
    "created_at": "2026-06-10T08:00:00Z"
  },
  "schedule_warning": ""
}
```

Поля ответа:

| Поле | Тип | Описание |
|---|---|---|
| `message` | string | Текст результата |
| `schedule` | `Schedule` \| null | Новый слот, если удалось пересчитать |
| `schedule_warning` | string | Предупреждение, если перепланирование не удалось |

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "неверный формат запроса"
}
```

или ошибка в `id`.

- `404 Not Found`
```json
{
  "error": "задача не найдена"
}
```

---

### 11. Удалить задачу

#### `DELETE /api/v1/tasks/{id}`

Удаляет задачу и связанное расписание.

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Response `200 OK`
```json
{
  "message": "задача удалена успешно"
}
```

##### Ошибки

- `400 Bad Request` — невалидный `id`
- `404 Not Found`
```json
{
  "error": "задача не найдена"
}
```

---

## Schedule

Все эндпоинты ниже требуют:

```http
Authorization: Bearer <token>
```

### 12. Получить расписание пользователя

#### `GET /api/v1/schedule`

Возвращает слоты расписания пользователя.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `from` | string(datetime) | нет | Показать записи со `start_time >= from` |
| `to` | string(datetime) | нет | Показать записи с `end_time <= to` |
| `status` | string | нет | Фильтр по статусу |
| `order` | string | нет | Порядок сортировки |

> `status` и `order` принимаются хендлером как есть. Допустимые конкретные значения зависят от реализации сервиса/SQL.

##### Пример
```http
GET /api/v1/schedule?from=2026-06-01T00:00:00Z&to=2026-06-30T23:59:59Z&status=pending
Authorization: Bearer <token>
```

##### Response `200 OK`
```json
[
  {
    "id": 5,
    "task_id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт",
    "status": "pending",
    "start_time": "2026-06-10T10:00:00Z",
    "end_time": "2026-06-10T12:00:00Z",
    "created_at": "2026-06-10T08:00:00Z"
  }
]
```

Если записей нет:

```json
[]
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "invalid filter: ..."
}
```

---

### 13. Получить расписание конкретной задачи

#### `GET /api/v1/schedule/task/{id}`

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Response `200 OK`
```json
{
  "id": 5,
  "task_id": 10,
  "user_id": 1,
  "title": "Подготовить отчёт",
  "status": "pending",
  "start_time": "2026-06-10T10:00:00Z",
  "end_time": "2026-06-10T12:00:00Z",
  "created_at": "2026-06-10T08:00:00Z"
}
```

##### Ошибки

- `400 Bad Request` — невалидный `id`
- `404 Not Found`
```json
{
  "error": "расписание для этой задачи не найдено"
}
```

---

### 14. Перепланировать задачу

#### `POST /api/v1/schedule/task/{id}/reschedule`

Повторно запускает автопланирование для одной задачи.

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Request body
Тело запроса **не требуется**.

##### Response `200 OK`
```json
{
  "schedule": {
    "id": 5,
    "task_id": 10,
    "user_id": 1,
    "title": "Подготовить отчёт",
    "status": "pending",
    "start_time": "2026-06-11T10:00:00Z",
    "end_time": "2026-06-11T12:00:00Z",
    "created_at": "2026-06-10T08:00:00Z"
  },
  "schedule_warning": ""
}
```

##### Response `422 Unprocessable Entity`
Если задачу не удалось поставить в расписание:

```json
{
  "schedule_warning": "не удалось автоматически запланировать задачу"
}
```

##### Ошибки

- `400 Bad Request` — невалидный `id`
- `404 Not Found`
```json
{
  "error": "задача не найдена"
}
```

---

### 15. Удалить задачу из расписания

#### `DELETE /api/v1/schedule/task/{id}`

Удаляет слот из календаря, не удаляя саму задачу.

##### Path parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `id` | integer | да | ID задачи |

##### Response `200 OK`
```json
{
  "message": "расписание удалено успешно"
}
```

##### Ошибки

- `400 Bad Request` — невалидный `id`

---

### 16. Получить свободные слоты

#### `GET /api/v1/schedule/free-slots`

Возвращает свободные интервалы рабочего времени в заданном диапазоне.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `from` | string(datetime) | да | Начало диапазона, RFC3339 |
| `to` | string(datetime) | да | Конец диапазона, RFC3339 |
| `duration` | integer | нет | Минимальная длительность в минутах, по умолчанию `60` |

##### Пример
```http
GET /api/v1/schedule/free-slots?from=2026-06-10T00:00:00Z&to=2026-06-12T00:00:00Z&duration=90
Authorization: Bearer <token>
```

##### Response `200 OK`
```json
[
  {
    "start_time": "2026-06-10T13:00:00Z",
    "end_time": "2026-06-10T14:30:00Z"
  },
  {
    "start_time": "2026-06-11T09:00:00Z",
    "end_time": "2026-06-11T11:00:00Z"
  }
]
```

Если слотов нет:

```json
[]
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "параметры 'from' и 'to' обязательны (формат RFC3339)"
}
```

```json
{
  "error": "неверная дата 'from': ..."
}
```

```json
{
  "error": "неверная дата 'to': ..."
}
```

```json
{
  "error": "'duration' должен быть положительным целым числом (минуты)"
}
```

---

## Statistics

Все эндпоинты ниже требуют:

```http
Authorization: Bearer <token>
```

### 17. Получить статистику

#### `GET /api/v1/statistics`

Если query-параметр `month` отсутствует, возвращается полная статистика `Statistics`.
Если `month` задан, возвращается месячная статистика `MonthlyStatistics`.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `month` | string | нет | Месяц в формате `YYYY.MM`, например `2026.06` |

##### Примеры
```http
GET /api/v1/statistics
Authorization: Bearer <token>
```

```http
GET /api/v1/statistics?month=2026.06
Authorization: Bearer <token>
```

##### Response `200 OK` без `month`
Тело формата `Statistics`.

##### Response `200 OK` с `month`
Тело формата `MonthlyStatistics`.

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "неверный формат месяца. Используйте ГГГГ.ММ, например, 2022.12"
}
```

```json
{
  "error": "неверный год. Год должен быть числом"
}
```

```json
{
  "error": "неверный месяц. Месяц должен быть числом от 1 до 12"
}
```

```json
{
  "error": "Месяц должен быть числом от 1 до 12"
}
```

```json
{
  "error": "нельзя получить статистику за будущие даты"
}
```

- `404 Not Found`
```json
{
  "warning": "статистика не доступна для указанного месяца"
}
```

---

### 18. Создать месячную статистику

#### `POST /api/v1/statistics?month=YYYY.MM`

Создаёт месячную статистику пользователя.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `month` | string | да | Месяц в формате `YYYY.MM` |

##### Request body
Формат `MonthlyStatistics`.

```json
{
  "month": "2026-06-01T00:00:00Z",
  "total_tasks": 42,
  "completed_tasks": 35,
  "overdue_completed": 8,
  "moved_deadlines": 3,
  "heatmap": [
    {
      "date": "2026-06-01T00:00:00Z",
      "value": 0.8
    }
  ]
}
```

> Поле `month` в body **игнорируется**. Сервер берёт месяц из query-параметра `month`.

##### Response `201 Created`
```json
{
  "message": "статистика создана успешно"
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "параметр month обязателен. Используйте формат ГГГГ.ММ, например, 2022.12"
}
```

```json
{
  "error": "неверный формат месяца. Используйте ГГГГ.ММ, например, 2022.12"
}
```

```json
{
  "error": "неверный месяц. Месяц должен быть числом от 1 до 12"
}
```

```json
{
  "error": "недопустимое тело запроса"
}
```

---

### 19. Обновить месячную статистику

#### `PUT /api/v1/statistics?month=YYYY.MM`

Обновляет месячную статистику пользователя.

##### Query parameters

| Параметр | Тип | Обязательный | Описание |
|---|---|:---:|---|
| `month` | string | да | Месяц в формате `YYYY.MM` |

##### Request body
Формат `MonthlyStatistics`.

```json
{
  "month": "2026-06-01T00:00:00Z",
  "total_tasks": 45,
  "completed_tasks": 38,
  "overdue_completed": 7,
  "moved_deadlines": 5,
  "heatmap": [
    {
      "date": "2026-06-01T00:00:00Z",
      "value": 0.9
    }
  ]
}
```

> Поле `month` в body **игнорируется**.

##### Response `200 OK`
```json
{
  "message": "статистика обновлена успешно"
}
```

##### Ошибки

- `400 Bad Request` — невалидный `month` или body

---

### 20. Удалить месячную статистику пользователя

#### `DELETE /api/v1/statistics`

Удаляет месячную статистику пользователя.

##### Request body
Не требуется.

##### Response `200 OK`
```json
{
  "message": "статистика удалена успешно"
}
```

##### Ошибки

- `401 Unauthorized`
```json
{
  "error": "необходимо авторизоваться"
}
```

---

## Preferences

Все эндпоинты ниже требуют:

```http
Authorization: Bearer <token>
```

### 21. Получить пользовательские настройки планирования

#### `GET /api/v1/preferences`

Возвращает настройки планирования пользователя. Если настройки ещё не сохранены, сервис возвращает значения по умолчанию.

##### Response `200 OK`
```json
{
  "user_id": 1,
  "work_start_hour": 9,
  "work_end_hour": 18,
  "work_days": "Mon,Tue,Wed,Thu,Fri",
  "min_slot_minutes": 30,
  "timezone": "UTC"
}
```

---

### 22. Обновить пользовательские настройки планирования

#### `PUT /api/v1/preferences`

Создаёт или полностью заменяет настройки пользователя.

##### Request body
```json
{
  "work_start_hour": 9,
  "work_end_hour": 18,
  "work_days": "Mon,Tue,Wed,Thu,Fri",
  "min_slot_minutes": 30,
  "timezone": "Europe/Moscow"
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `work_start_hour` | integer | да | Час начала рабочего дня |
| `work_end_hour` | integer | да | Час окончания рабочего дня |
| `work_days` | string | да | Рабочие дни через запятую |
| `min_slot_minutes` | integer | да | Минимальная длина слота |
| `timezone` | string | да | IANA timezone |

> `user_id` можно не передавать: сервер сам подставляет ID текущего пользователя.

##### Response `200 OK`
```json
{
  "user_id": 1,
  "work_start_hour": 9,
  "work_end_hour": 18,
  "work_days": "Mon,Tue,Wed,Thu,Fri",
  "min_slot_minutes": 30,
  "timezone": "Europe/Moscow"
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "..."
}
```

Текст ошибки зависит от валидации сервиса настроек.

---

## Gamification service (Flask)

Сервис геймификации работает отдельно от основного API.

Особенности:
- базовый URL по умолчанию: `http://<host>:8083`;
- все запросы/ответы — JSON;
- встроенной JWT-аутентификации в этом сервисе нет.

### 23. Health check геймификации

#### `GET /game/g_health`

##### Response `200 OK`
```json
{
  "status": "ok"
}
```

---

### 24. Отметить дневную активность и обновить streak

#### `POST /game/streak/check`

Фиксирует активность пользователя за текущий день.
Повторный вызов в тот же день идемпотентен.

##### Request body
```json
{
  "user_id": 42
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `user_id` | integer | да | ID пользователя |

##### Response `200 OK` — первый вызов за день
```json
{
  "message": "Streak updated",
  "previous_streak": 6,
  "current_streak": 7,
  "longest_streak": 10
}
```

##### Response `200 OK` — повторный вызов в тот же день
```json
{
  "message": "Already checked today",
  "current_streak": 7
}
```

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "user_id is required"
}
```

- `404 Not Found`
```json
{
  "error": "User not found"
}
```

- `500 Internal Server Error`
```json
{
  "error": "Database error"
}
```

---

### 25. Отметить выполнение задачи

#### `POST /game/task/complete`

Увеличивает счётчик выполненных задач, при необходимости обновляет perfect streak и начисляет ачивки.

##### Request body
```json
{
  "user_id": 42,
  "day_closed": true
}
```

| Поле | Тип | Обязательное | Описание |
|---|---|:---:|---|
| `user_id` | integer | да | ID пользователя |
| `day_closed` | boolean | нет | Закрыт ли день полностью; по умолчанию `false` |

##### Response `200 OK`
```json
{
  "message": "Task completed",
  "total_tasks": 17,
  "new_achievements": ["10 задач"]
}
```

| Поле | Тип | Описание |
|---|---|---|
| `message` | string | Всегда `Task completed` |
| `total_tasks` | integer | Обновлённый счётчик выполненных задач |
| `new_achievements` | array of string | Список новых достижений, полученных именно этим вызовом |

##### Ошибки

- `400 Bad Request`
```json
{
  "error": "user_id is required"
}
```

- `404 Not Found`
```json
{
  "error": "User not found"
}
```

- `500 Internal Server Error`
```json
{
  "error": "Database error"
}
```

---

## Сводная таблица эндпоинтов

### Основной API

| Метод | Путь | Авторизация |
|---|---|---|
| `GET` | `/health` | нет |
| `POST` | `/api/v1/auth/register` | нет |
| `POST` | `/api/v1/auth/login` | нет |
| `POST` | `/api/v1/auth/telegram` | нет |
| `POST` | `/api/v1/auth/vk` | нет |
| `GET` | `/api/v1/auth/me` | Bearer |
| `POST` | `/api/v1/tasks` | Bearer |
| `GET` | `/api/v1/tasks` | Bearer |
| `GET` | `/api/v1/tasks/{id}` | Bearer |
| `PUT` | `/api/v1/tasks/{id}` | Bearer |
| `DELETE` | `/api/v1/tasks/{id}` | Bearer |
| `GET` | `/api/v1/schedule` | Bearer |
| `GET` | `/api/v1/schedule/free-slots` | Bearer |
| `GET` | `/api/v1/schedule/task/{id}` | Bearer |
| `POST` | `/api/v1/schedule/task/{id}/reschedule` | Bearer |
| `DELETE` | `/api/v1/schedule/task/{id}` | Bearer |
| `GET` | `/api/v1/statistics` | Bearer |
| `POST` | `/api/v1/statistics` | Bearer |
| `PUT` | `/api/v1/statistics` | Bearer |
| `DELETE` | `/api/v1/statistics` | Bearer |
| `GET` | `/api/v1/preferences` | Bearer |
| `PUT` | `/api/v1/preferences` | Bearer |

### Gamification service

| Метод | Путь | Авторизация |
|---|---|---|
| `GET` | `/game/g_health` | нет |
| `POST` | `/game/streak/check` | нет |
| `POST` | `/game/task/complete` | нет |
