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


def get_rewards(user_id):
    conn = None
    try:
        conn = get_connection()
        cursor = conn.cursor()

        cursor.execute("""
            SELECT a.month, t.award_name 
            FROM awards a
            JOIN award_types t ON t.id = a.award_type_id
            WHERE a.user_id = %s
            ORDER BY a.month DESC
        """, (user_id,))
        rows = cursor.fetchall()

        rewards = {}
        for month, award in rows:
            if month not in rewards:
                rewards[month] = []
            if award not in rewards[month]:
                rewards[month].append(award)

        return {"status": "success", "rewards": rewards}

    except Exception as e:
        return {"error": "Unknown error"}
    finally:
        if conn:
            conn.close()