import pymysql
import os

DB_CONFIG = {
    'host': os.getenv("MYSQL_HOST"),
    'port': os.getenv("MYSQL_PORT"),
    'user': os.getenv("MYSQL_USER"),
    'password': os.getenv("MYSQL_PASSWORD"),
    'database': os.getenv("MYSQL_NAME"),
    'charset': 'utf8mb4'
}


def get_connection():
    return pymysql.connect(**DB_CONFIG)


def compare_statistics(user_id, old_month, new_month):
    conn = None
    try:
        conn = get_connection()
        cursor = conn.cursor()

        # Получаем цели
        cursor.execute("""
            SELECT target_meeting_deadlines, target_transfer_deadlines 
            FROM user_statistics_targets WHERE user_id = %s AND month = %s
        """, (user_id, old_month))
        targets = cursor.fetchone()
        if not targets:
            return {"error": "Цели для указанного месяца не найдены"}

        target_meeting, target_transfer = targets

        # Получаем статистику
        cursor.execute("""
            SELECT current_meeting_deadlines, current_transfer_deadlines 
            FROM user_statistics_targets WHERE user_id = %s AND month = %s
        """, (user_id, new_month))
        stats = cursor.fetchone()
        if not stats:
            return {"error": "Статистика для указанного месяца не найдена"}
        
        current_meeting_deadlines, current_transfer_deadlines= stats
        
        awarded = []

        # Хранитель Дедлайнов
        if current_meeting >= target_meeting:
            cursor.execute("""
                INSERT IGNORE INTO awards (user_id, month, award_type_id)
                SELECT %s, %s, id FROM award_types WHERE award_name = 'Хранитель Дедлайнов'
            """, (user_id, old_month))
            awarded.append("Хранитель Дедлайнов")

        # Твёрдый срок
        if current_transfer <= target_transfer:
            cursor.execute("""
                INSERT IGNORE INTO awards (user_id, month, award_type_id)
                SELECT %s, %s, id FROM award_types WHERE award_name = 'Твёрдый срок'
            """, (user_id, old_month))
            awarded.append("Твёрдый срок")

        conn.commit()
        return {"status": "success", "awarded": awarded}

    except Exception as e:
        return {"error": str(e)}
    finally:
        if conn:
            conn.close()