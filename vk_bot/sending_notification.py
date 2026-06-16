import os
import sys
from datetime import datetime, UTC
from zoneinfo import ZoneInfo
import json

import pymysql
import vk_api

# 1. Берем настроенные переменные из Docker Compose
TOKEN = os.getenv("VK_API_TOKEN")
DB_HOST = os.getenv("MYSQL_HOST")
DB_USER = os.getenv("MYSQL_USER")
DB_PASSWORD = os.getenv("MYSQL_PASSWORD")
DB_NAME = os.getenv("MYSQL_NAME")

deadline_keyboard = {
    "inline": True,
    "buttons": [
        [
            {
                "action": {
                    "type": "open_app",
                    "app_id": 54617588,
                    "label": "Открыть Дедлайн-Навигатор"
                }
            }
        ]
    ]
}


if not TOKEN:
    print("[CRON ERROR] Переменная VK_API_TOKEN не задана!")
    sys.exit(1)

# 2. Инициализация ВК
try:
    vk_session = vk_api.VkApi(token=TOKEN)
    vk = vk_session.get_api()
except Exception as e:
    print(f"[CRON ERROR] Ошибка авторизации в ВК: {e}")
    sys.exit(1)

# 3. Подключение к MySQL
try:
    connection = pymysql.connect(
        host=DB_HOST,
        user=DB_USER,
        password=DB_PASSWORD,
        database=DB_NAME,
        cursorclass=pymysql.cursors.DictCursor
    )
except Exception as e:
    print(f"[CRON ERROR] Ошибка подключения к MySQL: {e}")
    sys.exit(1)

try:
    with connection.cursor() as cursor:

        # Диапазон текущей минуты
        now = datetime.now()
        ts_start = now.strftime("%Y-%m-%d %H:%M:00")
        ts_end = now.strftime("%Y-%m-%d %H:%M:59")

        sql_schedules = """
            SELECT *
            FROM schedules
            WHERE start_time >= %s
              AND start_time <= %s
        """

        cursor.execute(sql_schedules, (ts_start, ts_end))
        schedules = cursor.fetchall()

        for schedule in schedules:

            # Получаем пользователя
            sql_user = """
                SELECT vk_id
                FROM users
                WHERE id = %s
            """

            cursor.execute(
                sql_user,
                (schedule.get("user_id"),)
            )

            user = cursor.fetchone()

            if not user or not user.get("vk_id"):
                print(
                    f"[CRON] Пользователь для расписания "
                    f"#{schedule.get('id')} не найден или не имеет vk_id"
                )
                continue

            # Проверяем, включены ли уведомления
            sql_notifications = """
                SELECT *
                FROM users_with_notifications
                WHERE user_id = %s
                  AND notifications_enabled IS TRUE
            """

            cursor.execute(
                sql_notifications,
                (schedule.get("user_id"),)
            )

            notification_settings = cursor.fetchone()

            if not notification_settings:
                print(
                    f"[CRON] Для пользователя "
                    f"{schedule.get('user_id')} уведомления отключены"
                )
                continue

            # Получаем настройки пользователя
            sql_preferences = """
                SELECT timezone
                FROM user_preferences
                WHERE user_id = %s
            """

            cursor.execute(
                sql_preferences,
                (schedule.get("user_id"),)
            )

            preferences = cursor.fetchone()

            # Если timezone не найден — уведомление не отправляем
            if not preferences or not preferences.get("timezone"):
                print(
                    f"[CRON] Для пользователя "
                    f"{schedule.get('user_id')} не найден timezone"
                )
                continue

            timezone = preferences["timezone"]

            # Название задачи
            schedule_name = schedule.get("name", "Без названия")

            # Перевод времени из UTC в локальный часовой пояс пользователя
            if isinstance(schedule.get("start_time"), datetime):

                schedule_datetime = schedule["start_time"]

                if schedule_datetime.tzinfo is None:
                    schedule_datetime = schedule_datetime.replace(
                        tzinfo=UTC
                    )

                try:
                    local_datetime = schedule_datetime.astimezone(
                        ZoneInfo(timezone)
                    )

                    schedule_time = local_datetime.strftime("%H:%M")

                except Exception as e:
                    print(
                        f"[CRON ERROR] Некорректный timezone "
                        f"для user_id={schedule.get('user_id')}: "
                        f"{timezone}. Ошибка: {e}"
                    )
                    continue

            else:
                schedule_time = str(schedule.get("start_time"))

            message_text = (
                f"Задача «{schedule_name}» "
                f"запланирована на {schedule_time}"
            )

            try:
                vk.messages.send(
                    peer_id=user["vk_id"],
                    random_id=0,
                    message=message_text,
                    keyboard=json.dumps(
                        deadline_keyboard,
                        ensure_ascii=False
                    )
                )

                print(
                    f"[CRON SUCCESS] Отправлено напоминание "
                    f"для vk_id {user['vk_id']}"
                )

            except Exception as e:
                print(
                    f"[CRON ERROR] Не удалось отправить сообщение "
                    f"в ВК для vk_id {user['vk_id']}: {e}"
                )

finally:
    connection.close()
    