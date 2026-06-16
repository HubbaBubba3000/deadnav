# Gamification Service

Микросервис геймификации платформы **DeadNav**. Считает streak'и активности, закрытые «идеальные» дни/недели/месяцы, ведёт счётчик выполненных задач и выдаёт пользователю ачивки.

- **Фреймворк:** Flask 2.3 + Flask-SQLAlchemy 3
- **БД:** MySQL (драйвер PyMySQL)
- **Порт по умолчанию:** `8083`
- **Аутентификация:** JWT (HS256), токены выпускаются Go-сервисом `deadnav/deadnav` (`UserService.generateToken`). Защищённые эндпоинты (`/game/streak/check`, `/game/task/complete`) требуют заголовок `Authorization: Bearer <token>`. `/game/g_health` остаётся открытым для health-check’ов.

---

## Запуск

### Переменные окружения

| Переменная      | Обязательная | По умолчанию | Описание                                  |
|-----------------|:------------:|--------------|-------------------------------------------|
| `DB_USER`       | да           | —            | Пользователь MySQL                        |
| `DB_PASSWORD`   | да           | —            | Пароль MySQL                              |
| `DB_HOST`       | нет          | `mysql`      | Хост БД (имя сервиса в docker-compose)    |
| `DB_PORT`       | нет          | `3306`       | Порт MySQL                                |
| `DB_NAME`       | да           | —            | Имя базы данных                           |
| `JWT_SECRET`    | да           | —            | Секрет Flask (`SECRET_KEY`)               |
| `GAME_PORT`     | нет          | `8083`       | Порт приложения                           |
| `FLASK_DEBUG`   | нет          | `false`      | `true`/`false` — режим отладки Flask      |

### Локальный запуск

```bash
pip install -r requirements.txt
python main.py
```

При старте через `__main__` автоматически вызывается `db.create_all()` — таблицы создаются, если их нет. В продакшене (gunicorn и т. п.) рекомендуется использовать **Flask-Migrate**.

### Docker

```bash
docker build -t deadnav/gamification .
docker run --rm -p 8083:8083 \
  -e DB_USER=user -e DB_PASSWORD=pass \
  -e DB_HOST=mysql -e DB_NAME=deadnav \
  -e JWT_SECRET=change-me \
  deadnav/gamification
```

---

## Модель данных

### `users`

Существующая в БД таблица, на которую ссылается сервис.

| Поле        | Тип             | Описание                |
|-------------|-----------------|-------------------------|
| `id`        | `BIGINT` (PK)   | ID пользователя         |
| `username`  | `VARCHAR(64)`   | Логин                   |
| `email`     | `VARCHAR(120)`  | E-mail                  |

### `user_gamification` (создаётся автоматически)

| Поле                     | Тип         | По умолчанию | Описание                                      |
|--------------------------|-------------|--------------|-----------------------------------------------|
| `user_id`                | `BIGINT` PK | —            | FK → `users.id`                               |
| `current_streak`         | `INT`       | `0`          | Текущая длина серии активных дней             |
| `longest_streak`         | `INT`       | `0`          | Максимальная серия за всё время               |
| `last_activity_date`     | `DATE`      | `NULL`       | Дата последнего чека streak                   |
| `total_tasks_completed`  | `INT`       | `0`          | Сколько задач закрыто за всё время            |
| `perfect_days`           | `INT`       | `0`          | Кол-во «идеально закрытых» дней              |
| `perfect_weeks`          | `INT`       | `0`          | Счётчик каждых 7 идеальных дней подряд        |
| `perfect_months`         | `INT`       | `0`          | Счётчик каждых 30 идеальных дней подряд       |
| `current_perfect_streak` | `INT`       | `0`          | Текущая серия идеальных дней                  |
| `last_perfect_date`      | `DATE`      | `NULL`       | Дата последнего идеального дня                |
| `achievements`           | `JSON`      | `[]`         | Список ID полученных ачивок                   |

### `user_activity` (создаётся автоматически)

| Поле             | Тип        | Описание                                  |
|------------------|------------|-------------------------------------------|
| `id`             | `INT` PK   | Автоинкремент                             |
| `user_id`        | `BIGINT`   | FK → `users.id`                           |
| `activity_date`  | `DATE`     | Дата активности                            |

Уникальный constraint `unique_user_day` на пару `(user_id, activity_date)` — обеспечивает идемпотентность чека streak и защиту от race-condition.

---

## Эндпоинты

Все запросы и ответы — `application/json`.

### Авторизация

Все защищённые эндпоинты требуют заголовок:

```
Authorization: Bearer <jwt>
```

JWT подписывается алгоритмом `HS256` тем же `JWT_SECRET`, что и основной Go-сервис (`deadnav/deadnav`). Ожидаемые claims:

| Claim           | Тип     | Описание                                  |
|-----------------|---------|-------------------------------------------|
| `user_id`       | integer | ID пользователя; используется как ключ геймификации |
| `username`      | string  | Логин (опционально, сохраняется в контексте запроса) |
| `auth_provider` | string  | `local` / `telegram` / `vk` (не валидируется) |
| `exp`           | integer | UNIX-время истечения (обязательно)        |
| `iat`           | integer | UNIX-время выпуска (опционально)          |

`user_id` для геймификации берётся **из токена**, а не из тела запроса. Если в теле передан `user_id`, не совпадающий с токеном — возвращается `403`.

Коды ответов middleware:

| Код | Тело                                                       | Причина                                                  |
|-----|------------------------------------------------------------|----------------------------------------------------------|
| 401 | `{"error": "invalid authorization format, use 'Bearer <token>'"}` | Нет/неверный заголовок `Authorization`              |
| 401 | `{"error": "invalid or expired token"}`                    | Токен невалиден, просрочен или подписан чужим секретом   |
| 401 | `{"error": "invalid token claims"}`                        | В токене нет `user_id` или он не приводится к `int`      |
| 500 | `{"error": "server misconfiguration"}`                     | На сервере не задан `JWT_SECRET`                         |

### `POST /game/streak/check`

Фиксирует факт активности пользователя за сегодня и обновляет streak. **Идемпотентен в пределах одного дня**: повторный вызов в ту же дату вернёт текущее значение streak без изменений.

#### Тело запроса

```json
{
  "user_id": 42
}
```

| Поле      | Тип      | Обязательное | Описание                |
|-----------|----------|:------------:|-------------------------|
| `user_id` | integer  | да           | ID пользователя в БД    |

#### Логика

1. Найти запись `user_gamification` по `user_id`. Если её нет — `404`.
2. Если для пары `(user_id, today)` уже есть запись в `user_activity` — вернуть `200` с признаком «уже отмечено».
3. Иначе:
   - Если `last_activity_date` — вчера → `current_streak += 1` (с обновлением `longest_streak` при необходимости).
   - Если `last_activity_date` пусто или старше вчера → `current_streak = 1`.
   - `last_activity_date = today`.
4. Вставить строку в `user_activity`. При `IntegrityError` (гонка запросов) — откатить и вернуть «уже отмечено».

#### Ответ `200 OK`

Успешный пересчёт:

```json
{
  "message": "Streak updated",
  "previous_streak": 6,
  "current_streak": 7,
  "longest_streak": 10
}
```

Повторный вызов в тот же день:

```json
{
  "message": "Already checked today",
  "current_streak": 7
}
```

#### Ошибки

| Код | Тело                                  | Причина                              |
|-----|---------------------------------------|--------------------------------------|
| 400 | `{"error": "user_id is required"}`    | Не передан `user_id`                 |
| 404 | `{"error": "User not found"}`         | Нет `user_gamification` для юзера    |
| 500 | `{"error": "Database error"}`         | Непредвиденная ошибка БД             |

---

### `POST /game/task/complete`

Регистрирует выполнение задачи пользователем. Инкрементирует счётчик задач, обновляет серию «идеальных» дней (если день закрыт) и начисляет новые ачивки.

#### Тело запроса

```json
{
  "user_id": 42,
  "day_closed": true
}
```

| Поле         | Тип      | Обязательное | По умолчанию | Описание                                                     |
|--------------|----------|:------------:|:------------:|--------------------------------------------------------------|
| `user_id`    | integer  | да           | —            | ID пользователя в БД                                         |
| `day_closed` | boolean  | нет          | `false`      | Признак, что пользователь **закрыл день** (выполнил всё на сегодня) |

#### Логика

1. Найти `user_gamification`. Если её нет — `404`.
2. `total_tasks_completed += 1`.
3. Если `day_closed == true`:
   - если `last_perfect_date == today` — ничего не делать (уже учтено);
   - иначе продлить или начать серию `current_perfect_streak`, поставить `last_perfect_date = today`, инкрементнуть `perfect_days`;
   - если `current_perfect_streak % 7 == 0` → `perfect_weeks += 1`;
   - если `current_perfect_streak % 30 == 0` → `perfect_months += 1`.
4. Пересчитать ачивки через `get_new_achievements` — добавить в `user.achievements` все впервые выполненные условия и вернуть их список в ответе.
5. Сохранить изменения.

#### Ответ `200 OK`

```json
{
  "message": "Task completed",
  "total_tasks": 17,
  "new_achievements": ["10 задач"]
}
```

| Поле                | Тип     | Описание                                                |
|---------------------|---------|---------------------------------------------------------|
| `message`           | string  | Фиксированная строка `"Task completed"`                 |
| `total_tasks`       | integer | Новое значение `total_tasks_completed`                  |
| `new_achievements`  | array   | Список **человекочитаемых названий** вновь полученных ачивок (за этот вызов). Пустой массив, если новых нет |

> ⚠️ Поле `achievements` в БД хранит **ID** ачивок (`task_10`, `perfect_day`, …), а в ответ попадают **названия** на русском. Это поведение текущей версии.

#### Текущий каталог ачивок

| ID              | Название              | Условие                                    |
|-----------------|-----------------------|--------------------------------------------|
| `task_10`       | `10 задач`            | `total_tasks_completed >= 10`              |
| `task_100`      | `100 задач`           | `total_tasks_completed >= 100`             |
| `task_500`      | `500 задач`           | `total_tasks_completed >= 500`             |
| `perfect_day`   | `Идеальный день`      | `perfect_days >= 1`                        |
| `perfect_week`  | `Идеальная неделя`    | `perfect_weeks >= 1`                       |
| `perfect_month` | `Идеальный месяц`     | `perfect_months >= 1`                      |

#### Ошибки

| Код | Тело                                  | Причина                              |
|-----|---------------------------------------|--------------------------------------|
| 400 | `{"error": "user_id is required"}`    | Не передан `user_id`                 |
| 404 | `{"error": "User not found"}`         | Нет `user_gamification` для юзера    |
| 500 | `{"error": "Database error"}`         | Непредвиденная ошибка БД             |

---

## Примеры

### `curl`

```bash
# Чек streak
curl -X POST http://localhost:8083/game/streak/check \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{}'

# Закрытие задачи
curl -X POST http://localhost:8083/game/task/complete \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"day_closed": true}'
```

> `user_id` в теле можно не передавать — он берётся из токена. Если передан и не совпадает с токеном — сервис вернёт `403`.

### Типичный дневной сценарий

```text
[утро]  POST /game/streak/check   { "user_id": 42 }           → streak 1 → 2
[день]  POST /game/task/complete  { "user_id": 42 }           → total 1
[день]  POST /game/task/complete  { "user_id": 42 }           → total 2
…
[вечер] POST /game/task/complete  { "user_id": 42,
                                    "day_closed": true }       → perfect_day
```

---

## Известные особенности

- **`achievements` как JSON** хранится нативно в MySQL 8 (`JSON`). На более старых версиях MySQL колонка может не создаться — требуется `LONGTEXT` с ручной миграцией.
- **`/game/task/complete` не идемпотентен** — каждый вызов инкрементит `total_tasks_completed`. Дедупликация задач — на стороне вызывающего сервиса.
- **`day_closed=true` без смены даты** — если сегодня уже был закрыт день, повторный вызов с `day_closed=true` не инкрементит `perfect_days` (защита `last_perfect_date == today`).
- **Сервис не разворачивает миграции в проде** — для деплоя используйте `flask db upgrade` (Flask-Migrate) или `alembic` до запуска приложения.
- **`debug=True` выключен по умолчанию** — включается только переменной `FLASK_DEBUG=true`.
- **Аутентификация** — реализована на уровне самого приложения через JWT (HS256) с тем же `JWT_SECRET`, что и Go-сервис. Эндпоинт `/game/g_health` намеренно остаётся открытым.
