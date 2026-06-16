import logging
import os
from functools import wraps

import jwt
import pymysql
from compare_statistics import compare_statistics
from define_target import define_target
from flask import Flask, g, jsonify, request
from get_rewards import get_rewards

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

DB_CONFIG = {
    "host": os.getenv("MYSQL_HOST"),
    "port": os.getenv("MYSQL_PORT"),
    "user": os.getenv("MYSQL_USER"),
    "password": os.getenv("MYSQL_PASSWORD"),
    "database": os.getenv("MYSQL_NAME"),
    "charset": "utf8mb4",
}


def get_connection():
    return pymysql.connect(**DB_CONFIG)


def get_user_by_token(token):
    conn = None
    try:
        conn = get_connection()
        cursor = conn.cursor()
        cursor.execute(
            """
            SELECT id, username FROM users WHERE token = %s
        """,
            (token,),
        )
        return cursor.fetchone()
    finally:
        if conn:
            conn.close()


# ====================== JWT MIDDLEWARE ======================
# Зеркалит pkg/middleware/auth.go из Go-сервиса deadnav/deadnav.
# Токен подписывается HS256 с тем же JWT_SECRET, claims содержат
# user_id / username / auth_provider / exp / iat (см. UserService.generateToken).

JWT_ALGORITHM = "HS256"
JWT_SECRET = os.getenv("JWT_SECRET", "")


def _extract_bearer_token(auth_header: str) -> str | None:
    """Возвращает токен из заголовка `Authorization: Bearer <token>` или None."""
    if not auth_header:
        return None
    parts = auth_header.split(None, 1)
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1].strip():
        return None
    return parts[1].strip()


def jwt_required(view):
    """Декоратор: требует валидный JWT в `Authorization: Bearer <token>`.

    При успешной валидации кладёт `user_id` (int) в `flask.g.current_user_id`
    и `username` в `flask.g.current_username`, чтобы handler'ы могли
    доверять идентификации пользователя. Соответствует поведению
    middleware.JWTAuth в Go-сервисе.
    """

    @wraps(view)
    def wrapper(*args, **kwargs):
        if not JWT_SECRET:
            logger.error("JWT_SECRET is not configured")
            return jsonify({"error": "server misconfiguration"}), 500

        token = _extract_bearer_token(request.headers.get("Authorization", ""))
        if token is None:
            return jsonify(
                {"error": "invalid authorization format, use 'Bearer <token>'"}
            ), 401

        try:
            claims = jwt.decode(
                token,
                JWT_SECRET,
                algorithms=[JWT_ALGORITHM],
            )
        except jwt.ExpiredSignatureError:
            return jsonify({"error": "invalid or expired token"}), 401
        except jwt.InvalidTokenError as e:
            logger.warning("JWT validation failed: %s", e)
            return jsonify({"error": "invalid or expired token"}), 401

        user_id = claims.get("user_id")
        if user_id is None:
            return jsonify({"error": "invalid token claims"}), 401

        try:
            g.current_user_id = int(user_id)
        except (TypeError, ValueError):
            return jsonify({"error": "invalid token claims"}), 401
        g.current_username = claims.get("username")
        return view(*args, **kwargs)

    return wrapper


@app.route("/gamifications_in_stats/compare_statistics", methods=["GET"])
@jwt_required
def compare_statistics_route():
    old_month = request.args.get("old_month")
    new_month = request.args.get("new_month")

    if not old_month or not new_month:
        return jsonify({"error": "Не указаны параметры old_month и/или new_month"}), 400

    result = compare_statistics(g.current_user_id, old_month, new_month)
    return jsonify(result), 200


@app.route("/gamifications_in_stats/define_target", methods=["GET"])
@jwt_required
def define_target_route():
    month = request.args.get("month")

    if not month:
        return jsonify({"error": "Не указан параметр month"}), 400

    result = define_target(g.current_user_id, month)
    return jsonify(result), 200


@app.route("/gamifications_in_stats/get_rewards", methods=["GET"])
@jwt_required
def get_rewards_route():
    result = get_rewards(g.current_user_id)
    return jsonify(result), 200


if __name__ == "__main__":
    app.run(port=5000)
