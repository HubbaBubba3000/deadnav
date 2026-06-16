# Deadnav — Планировщик задач с календарём

> **Deadnav** — это REST API на Go для управления задачами с интеллектуальным автоматическим планированием. Система находит свободные временные слоты в рабочем календаре пользователя и вставляет задачи туда, учитывая рабочие часы, рабочие дни, часовой пояс и приоритет.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql)](https://www.mysql.com/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

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

```shell
git clone https://github.com/your-org/deadnav.git
cd deadnav
make dev
```

После старта API доступен на `http://localhost:80`. (через Nginx)

#### Проверка работы

```shell
curl http://localhost/health
```

Ожидаемый ответ:
```json
{
  "status": "ok"
}
```

### Локальная разработка

**Требования:**
- Go 1.25+
- MySQL 8.0+
- `git`

**1. Клонирование и установка зависимостей**

```shell
git clone https://github.com/your-org/deadnav.git
cd deadnav
go mod download
```

**2. Настройка базы данных**

Выполните SQL-схему из `scripts/init.sql`:

```shell
mysql -u root -p < scripts/init.sql
```

**3. Переменные окружения**

Создайте файл `.env` в корне проекта (или задайте переменные в shell):

```env
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

```shell
go run ./cmd/api
```

**5. Запуск тестов**

```shell
go test ./...
```

---

## Переменные окружения

| Переменная | По умолчанию | Обязательная | Описание |
|---|---|:---:|---|
| `SERVER_PORT` | `8080` | — | TCP-порт HTTP-сервера |
| `DB_HOST` | `localhost` | — | Хост базы данных |
| `DB_PORT` | `3306` | — | Порт базы данных |
| `DB_USER` | `root` | — | Пользователь БД |
| `DB_PASSWORD` | `password` | — | Пароль БД |
| `DB_NAME` | `deadnav` | — | Имя БД |
| `JWT_SECRET` | — | ✅ | Секретный ключ JWT |
| `JWT_EXPIRATION_HOURS` | `24` | — | Срок действия JWT (часы) |
| `TELEGRAM_BOT_TOKEN` | — | — | Токен Telegram бота |
| `ALLOWED_ORIGINS` | — | — | Разрешенные Origins для CORS |

---

## API Reference

### Аутентификация

#### `POST /api/v1/auth/register` — Регистрация
Зарегистрировать нового пользователя.

#### `POST /api/v1/auth/login` — Вход
Войти в систему по username/email и паролю.

#### `POST /api/v1/auth/telegram` — Вход через Telegram
Войти через Telegram Bot.

#### `GET /api/v1/auth/me` — Текущий пользователь
Получить информацию о текущем пользователе.

### Задачи

#### `POST /api/v1/tasks` — Создать задачу
Создать новую задачу. Автоматически планирует, если не указан `schedule_id`.

#### `GET /api/v1/tasks` — Список задач
Получить список всех задач пользователя.

#### `GET /api/v1/tasks/:id` — Получить задачу
Получить конкретную задачу по ID.

#### `PUT /api/v1/tasks/:id` — Обновить задачу
Обновить информацию о задаче.

#### `DELETE /api/v1/tasks/:id` — Удалить задачу
Удалить задачу.

### Расписание / Календарь

#### `GET /api/v1/schedule` — Весь календарь
Получить все запланированные задачи пользователя.

#### `GET /api/v1/schedule/free-slots` — Свободные слоты
Получить свободные временные слоты для задачи.

#### `GET /api/v1/schedule/task/:id` — Расписание задачи
Получить расписание конкретной задачи.

#### `POST /api/v1/schedule/task/:id/reschedule` — Повторное планирование
Перепланировать задачу в другой слот. Эндпоинт сначала пытается найти свободное место, а если его нет — автоматически сдвигает конфликтующие задачи в их следующие доступные слоты. При успехе возвращает объект с полем `schedule`. Если даже после каскадного перепланирования до дедлайна нет места, возвращает `422` и `schedule_warning` со списком задач, которые не удалось подвинуть.

#### `DELETE /api/v1/schedule/task/:id` — Снять задачу с расписания
Снять задачу с расписания.

### Настройки планировщика

#### `GET /api/v1/preferences` — Получить настройки
Получить текущие настройки пользователя.

#### `PUT /api/v1/preferences` — Обновить настройки
Обновить настройки планировщика.

### Статистика

#### `GET /api/v1/statistics` — Статистика пользователя
Получить статистику по задачам и продуктивности.

### Healthcheck

#### `GET /health` — Статус сервиса
Проверить доступность сервиса.

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

```env
SERVER_PORT=8080
DB_HOST=mysql
DB_PORT=3306
DB_USER=deadnav_user
DB_PASSWORD=your-db-password
DB_NAME=deadnav
JWT_SECRET=your-jwt-secret
ALLOWED_ORIGINS=https://yourdomain.com
```

**2. Запустите:**

```shell
docker-compose -f docker-compose.yml up -d
```

### Nginx reverse proxy

Для доступа извне рекомендуется использовать Nginx:

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Makefile

Команды для разработки:

```makefile
.PHONY: help dev prod build clean logs stop restart

help:
	@echo "dev     - запуск в разработке"
	@echo "prod    - запуск в продакшене"
	@echo "build   - сборка"
	@echo "clean   - очистка"
	@echo "logs    - логи"
	@echo "stop    - остановить контейнеры"
	@echo "restart - перезапустить контейнеры"

dev: 
	docker-compose up

prod:
	docker-compose -f docker-compose.yml up -d

build:
	docker build -t deadnav .

clean:
	docker-compose down -v

logs:
	docker-compose logs -f

stop:
	docker-compose stop

restart:
	docker-compose restart
```

---

## Расширение API

1. **Модель** — добавить структуру в `internal/models`
2. **Сервис** — реализовать логику в `internal/services`
3. **Обработчик** — добавить метод в `internal/handlers`
4. **Маршруты** — зарегистрировать в `cmd/api/main.go`
5. **Тесты** — добавить в соответствующие файлы

---

## Лицензия

MIT License
