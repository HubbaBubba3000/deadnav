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


def calculate_next_month(month_str):
    year, month, day = map(int, month_str.split('-'))
    month += 1
    if month > 12:
        month = 1
        year += 1
    return f"{year}-{month:02d}-{d:02d}"


def define_target(user_id, month):
    conn = None
    try:
        conn = get_connection()
        cursor = conn.cursor()

        cursor.execute("""
            SELECT total_tasks, completed_tasks, moved_deadlines 
            FROM user_statistics WHERE user_id = %s AND month = %s
        """, (user_id, month))
        stats = cursor.fetchone()

        if not stats:
            return {"error": "Stats not found"}

        total_tasks, completed_tasks, moved_deadlines = stats
        current_meeting = int((completed_tasks / total_tasks) * 100)
        current_transfer = int((moved_deadlines / total_tasks) * 100)

        target_meeting = round(20 / (1 + 0.0036 * (current_meeting - 1) ** 1.6), 1)
        target_transfer = round(20 / (1 + 0.0036 * (101 - current_transfer) ** 1.6), 1)

        next_month = calculate_next_month(month)

        cursor.execute("""
            INSERT INTO user_statistics_targets (user_id, month, current_meeting_deadlines, current_transfer_deadlines, target_meeting_deadlines, target_transfer_deadlines)
            VALUES (%s, %s, %s, %s, %s, %s)
        """, (user_id, month, current_meeting, current_transfer, target_meeting, target_transfer))

        conn.commit()

        return {
            "status": "success",
            "target_month": next_month,
            "target_meeting_deadlines": target_meeting,
            "target_transfer_deadlines": target_transfer
        }

    except Exception as e:
        return {"error": "Unexpected error"}
    finally:
        if conn:
            conn.close()