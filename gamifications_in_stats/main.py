import pymysql

import os
from functools import wraps
from flask import Flask, request, jsonify

from compare_statistics import compare_statistics
from define_target import define_target
from get_rewards import get_rewards

app = Flask(__name__)

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


def get_user_by_token(token):
    conn = None
    try:
        conn = get_connection()
        cursor = conn.cursor()
        cursor.execute("""
            SELECT id, username FROM users WHERE token = %s
        """, (token,))
        return cursor.fetchone()
    finally:
        if conn:
            conn.close()


def require_token(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        token = request.args.get('token')

        if not token:
            return jsonify({"error": "Не указан параметр token"}), 401

        user = get_user_by_token(token)

        if not user:
            return jsonify({"error": "Неверный token"}), 401

        request.user_id, request.username = user

        return f(*args, **kwargs)
    return decorated


@app.route('/gamifications_in_stats/compare_statistics', methods=['GET'])
@require_token
def compare_statistics_route():
    old_month = request.args.get('old_month')
    new_month = request.args.get('new_month')

    if not old_month or not new_month:
        return jsonify({"error": "Не указаны параметры old_month и/или new_month"}), 400

    result = compare_statistics(request.user_id, old_month, new_month)
    return jsonify(result), 200


@app.route('/gamifications_in_stats/define_target', methods=['GET'])
@require_token
def define_target_route():
    month = request.args.get('month')

    if not month:
        return jsonify({"error": "Не указан параметр month"}), 400

    result = define_target(request.user_id, month)
    return jsonify(result), 200


@app.route('/gamifications_in_stats/get_rewards', methods=['GET'])
@require_token
def get_rewards_route():
    result = get_rewards(request.user_id)
    return jsonify(result), 200


if __name__ == '__main__':
    app.run(port=5000)