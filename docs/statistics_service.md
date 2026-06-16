# Документация по сервису статистики

## Обзор

Сервис статистики предоставляет API для получения аналитических данных о задачах пользователя. Сервис позволяет получать как общую статистику по всем задачам пользователя, так и детализированную месячную статистику с визуализацией в виде тепловой карты.

## Доступные эндпоинты

### GET /api/v1/statistics - Получение статистики

Получает статистическую информацию о задачах пользователя. Без параметров возвращает полную статистику, с параметром month возвращает месячную статистику.

#### Параметры запроса

| Параметр | Тип | Обязательный | Описание |
|---------|-----|-------------|----------|
| month | string | Нет | Месяц и год в формате YYYY.MM (например, 2023.12). Если не указан, возвращается полная статистика. |

#### Заголовки

| Заголовок | Значение | Обязательный | Описание |
|---------|--------|-------------|----------|
| Authorization | Bearer <token> | Да | JWT токен для аутентификации пользователя. |

#### Примеры запросов

**Запрос полной статистики:**
```
GET /api/v1/statistics
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Запрос месячной статистики:**
```
GET /api/v1/statistics?month=2023.12
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

#### Ответы

**200 OK - Успешный ответ**

При отсутствии параметра month возвращается полная статистика:

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
  "productivity_score": 78.5
}
```

При наличии параметра month возвращается месячная статистика:

```json
{
  "total_tasks": 42,
  "completed_tasks": 35,
  "overdue_completed": 8,
  "moved_deadlines": 3,
  "heatmap": [
    {
      "date": "2023-12-01T00:00:00Z",
      "value": 0.8
    },
    {
      "date": "2023-12-02T00:00:00Z",
      "value": 0.6
    },
    {
      "date": "2023-12-03T00:00:00Z",
      "value": 1.0
    }
    // ... остальные дни месяца
  ]
}
```


**400 Bad Request - Неверный формат параметра**

```json
{
  "error": "Invalid month format. Use YYYY.MM, e.g., 2022.12"
}
```
```json
{
  "error": "Invalid month. Month must be a number between 1 and 12"
}
```
```json
{
  "error": "Invalid year. Year must be a valid number"
}
```
```json
{
  "error": "Cannot get statistics for future dates"
}
```

**401 Unauthorized - Неавторизованный доступ**
```json
{
  "error": "unauthorized"
}
```

**404 Not Found - Отсутствуют данные за указанный период**
```json
{
  "warning": "No statistics data available for the specified month"
}
```

**500 Internal Server Error - Внутренняя ошибка сервера**
```json
{
  "error": "GetStatistics: fetch"
}
```

### POST /api/v1/statistics - Создание месячной статистики

Создает новую запись месячной статистики для пользователя.

#### Заголовки

| Заголовок | Значение | Обязательный | Описание |
|---------|--------|-------------|----------|
| Authorization | Bearer <token> | Да | JWT токен для аутентификации пользователя. |
| Content-Type | application/json | Да | Тип содержимого запроса. |

#### Тело запроса

```json
{
  "total_tasks": 42,
  "completed_tasks": 35,
  "overdue_completed": 8,
  "moved_deadlines": 3,
  "heatmap": [
    {
      "date": "2023-12-01T00:00:00Z",
      "value": 0.8
    },
    {
      "date": "2023-12-02T00:00:00Z",
      "value": 0.6
    }
  ]
}
```

#### Ответы

**201 Created - Статистика успешно создана**
```json
{
  "message": "Statistics created successfully"
}
```

**400 Bad Request - Неверное тело запроса**
```json
{
  "error": "Invalid request body"
}
```

**401 Unauthorized - Неавторизованный доступ**
```json
{
  "error": "unauthorized"
}
```

**500 Internal Server Error - Внутренняя ошибка сервера**
```json
{
  "error": "CreateMonthlyStatistics"
}
```

### PUT /api/v1/statistics - Обновление месячной статистики

Обновляет существующую запись месячной статистики для пользователя. Если запись не существует, она будет создана.

#### Заголовки

| Заголовок | Значение | Обязательный | Описание |
|---------|--------|-------------|----------|
| Authorization | Bearer <token> | Да | JWT токен для аутентификации пользователя. |
| Content-Type | application/json | Да | Тип содержимого запроса. |

#### Тело запроса

```json
{
  "total_tasks": 45,
  "completed_tasks": 38,
  "overdue_completed": 7,
  "moved_deadlines": 5,
  "heatmap": [
    {
      "date": "2023-12-01T00:00:00Z",
      "value": 0.9
    },
    {
      "date": "2023-12-02T00:00:00Z",
      "value": 0.7
    }
  ]
}
```

#### Ответы

**200 OK - Статистика успешно обновлена**
```json
{
  "message": "Statistics updated successfully"
}
```

**400 Bad Request - Неверное тело запроса**
```json
{
  "error": "Invalid request body"
}
```

**401 Unauthorized - Неавторизованный доступ**
```json
{
  "error": "unauthorized"
}
```

**500 Internal Server Error - Внутренняя ошибка сервера**
```json
{
  "error": "UpdateMonthlyStatistics"
}
```

### DELETE /api/v1/statistics - Удаление статистики

Удаляет все записи статистики для пользователя.

#### Заголовки

| Заголовок | Значение | Обязательный | Описание |
|---------|--------|-------------|----------|
| Authorization | Bearer <token> | Да | JWT токен для аутентификации пользователя. |

#### Ответы

**200 OK - Статистика успешно удалена**
```json
{
  "message": "Statistics deleted successfully"
}
```

**401 Unauthorized - Неавторизованный доступ**
```json
{
  "error": "unauthorized"
}
```

**500 Internal Server Error - Внутренняя ошибка сервера**
```json
{
  "error": "DeleteMonthlyStatistics"
}
```

## Описание моделей данных

### Полная статистика (models.Statistics)

| Поле | Тип | Описание |
|------|-----|----------|
| total_tasks | int64 | Общее количество задач |
| completed_tasks | int64 | Количество завершенных задач |
| pending_tasks | int64 | Количество задач в ожидании |
| in_progress_tasks | int64 | Количество задач в процессе выполнения |
| cancelled_tasks | int64 | Количество отмененных задач |
| tasks_by_status | map[string]int64 | Количество задач по статусам |
| tasks_by_priority | map[int]int64 | Количество задач по приоритетам |
| overdue_tasks | int64 | Количество просроченных задач |
| upcoming_deadlines | int64 | Количество задач с ближайшими дедлайнами (в течение 7 дней) |
| avg_delay_hours | float64 | Среднее опоздание при завершении задач (в часах) |
| on_time_completion_rate | float64 | Процент своевременного завершения задач |
| avg_duration_hours | float64 | Средняя продолжительность выполнения задач (в часах) |
| median_duration_hours | float64 | Медианная продолжительность выполнения задач (в часах) |
| min_duration_hours | float64 | Минимальная продолжительность выполнения задач (в часах) |
| max_duration_hours | float64 | Максимальная продолжительность выполнения задач (в часах) |
| avg_duration_by_priority | map[int]float64 | Средняя продолжительность выполнения задач по приоритетам |
| tasks_created_this_week | int64 | Количество созданных задач на этой неделе |
| tasks_completed_this_week | int64 | Количество завершенных задач на этой неделе |
| tasks_created_last_week | int64 | Количество созданных задач на прошлой неделе |
| tasks_completed_last_week | int64 | Количество завершенных задач на прошлой неделе |
| completion_trend | string | Тренд завершения задач ("improving" - улучшается, "declining" - ухудшается, "stable" - стабильно) |
| high_priority_tasks | int64 | Количество задач высокого приоритета (4-5) |
| low_priority_tasks | int64 | Количество задач низкого приоритета (1-2) |
| high_priority_completion_rate | float64 | Процент завершения задач высокого приоритета |
| low_priority_completion_rate | float64 | Процент завершения задач низкого приоритета |
| avg_tasks_per_day | float64 | Среднее количество задач в день (последние 30 дней) |
| peak_day | string | День недели с наибольшим количеством созданных задач |
| tasks_by_day_of_week | map[string]int64 | Количество задач по дням недели |
| productivity_score | float64 | Оценка продуктивности (0-100) |

### Месячная статистика (dto.MonthlyStatistics)

| Поле | Тип | Описание |
|------|-----|----------|
| total_tasks | int64 | Общее количество задач на месяц |
| completed_tasks | int64 | Количество завершенных задач за месяц |
| overdue_completed | int64 | Количество завершенных просроченных задач |
| moved_deadlines | int64 | Количество задач с перенесенными дедлайнами |
| heatmap | []HeatmapDay | Данные для тепловой карты по дням месяца |

### Данные для тепловой карты (dto.HeatmapDay)

| Поле | Тип | Описание |
|------|-----|----------|
| date | time.Time (string) | Дата в формате ISO 8601 |
| value | float64 | Значение от 0 до 1, где 0 - красный (ничего не выполнено), 1 - зеленый (все выполнено). Промежуточные значения представляют градиентные цвета. |