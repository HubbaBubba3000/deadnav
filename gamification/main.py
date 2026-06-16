import json
import logging
import os
from datetime import date, timedelta
from functools import wraps

import jwt
from dotenv import load_dotenv
from flask import Flask, g, jsonify, request
from flask_sqlalchemy import SQLAlchemy
from sqlalchemy.exc import IntegrityError, OperationalError, ProgrammingError

load_dotenv()

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# ====================== DATABASE CONFIG ======================
db_user = os.getenv("MYSQL_USER")
db_password = os.getenv("MYSQL_PASSWORD")
db_host = os.getenv("MYSQL_HOST", "mysql")
db_port = os.getenv("MYSQL_PORT", "3306")
db_name = os.getenv("MYSQL_DATABASE")

DATABASE_URL = f"mysql+pymysql://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}"

app.config["SQLALCHEMY_DATABASE_URI"] = DATABASE_URL
app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = False
app.config["SECRET_KEY"] = os.getenv("JWT_SECRET")

db = SQLAlchemy(app)


class UserGamification(db.Model):
    __tablename__ = "user_gamification"

    # No ForeignKey("users.id") here — the `users` table belongs to the Go
    # service and is not part of this SQLAlchemy metadata. Declaring a FK would
    # cause NoReferencedTableError at flush time because SQLAlchemy tries to
    # resolve the reference against its own metadata, not the live DB schema.
    # The actual FK constraint lives in MySQL and is enforced there; we rely on
    # catching IntegrityError 1452 to handle missing-user cases.
    user_id = db.Column(db.BigInteger, primary_key=True)

    current_streak = db.Column(db.Integer, default=0)
    longest_streak = db.Column(db.Integer, default=0)
    last_activity_date = db.Column(db.Date, nullable=True)

    total_tasks_completed = db.Column(db.Integer, default=0)

    perfect_days = db.Column(db.Integer, default=0)
    perfect_weeks = db.Column(db.Integer, default=0)
    perfect_months = db.Column(db.Integer, default=0)
    current_perfect_streak = db.Column(db.Integer, default=0)
    last_perfect_date = db.Column(db.Date, nullable=True)

    achievements = db.Column(db.JSON, default=list)

    def __init__(self, user_id: int) -> None:
        super().__init__()
        self.user_id = user_id


class UserActivity(db.Model):
    __tablename__ = "user_activity"
    id = db.Column(db.Integer, primary_key=True)
    user_id = db.Column(
        db.BigInteger, nullable=False
    )  # No ForeignKey — see UserGamification note above
    activity_date = db.Column(db.Date, nullable=False)

    def __init__(self, user_id: int, activity_date):
        self.user_id = user_id
        self.activity_date = activity_date

    __table_args__ = (
        db.UniqueConstraint("user_id", "activity_date", name="unique_user_day"),
    )


# NOTE: класс `User` намеренно удалён.
# Таблица `users` принадлежит Go-сервису `app`. Из этого контейнера
# она может быть не видна (не создана или нет прав), и любой SELECT
# по ней даёт pymysql.err.OperationalError (1146 / 1142) → 500.
# Валидация существования пользователя выполняется через FK:
# при INSERT строки с несуществующим user_id MySQL бросит
# IntegrityError 1452, который мы ловим в обработчике.


# ====================== HELPER FUNCTIONS ======================
def check_and_update_streak(user: UserGamification) -> None:
    today = date.today()
    last = user.last_activity_date

    if last is None:
        user.current_streak = 1
        user.longest_streak = max(user.longest_streak, 1)
        user.last_activity_date = today
        return

    diff = (today - last).days

    if diff == 0:
        # Уже обработано выше через проверку UserActivity,
        # но оставляем явную ветку для защиты от повторного вызова
        return
    elif diff == 1:
        user.current_streak += 1
        if user.current_streak > user.longest_streak:
            user.longest_streak = user.current_streak
    else:
        # Стрик прерван — сбрасываем
        user.current_streak = 1

    user.last_activity_date = today


def check_and_update_perfect_streak(
    user: UserGamification, is_day_closed: bool
) -> None:
    if not is_day_closed:
        return

    today = date.today()
    last = user.last_perfect_date

    if last == today:
        return

    if last == (today - timedelta(days=1)):
        user.current_perfect_streak += 1
    else:
        user.current_perfect_streak = 1

    user.last_perfect_date = today
    user.perfect_days += 1

    # FIX #5: инкрементируем при достижении кратного порога стрика,
    # а не перезаписываем через max() — иначе счётчик не растёт корректно
    if user.current_perfect_streak % 7 == 0:
        user.perfect_weeks += 1

    if user.current_perfect_streak % 30 == 0:
        user.perfect_months += 1


def get_new_achievements(user: UserGamification) -> list[str]:
    # FIX #7: achievements теперь db.JSON — json.loads не нужен
    current: list = user.achievements or []
    new_ones: list[str] = []

    rewards = [
        ("task_10", "10 задач", user.total_tasks_completed >= 10),
        ("task_100", "100 задач", user.total_tasks_completed >= 100),
        ("task_500", "500 задач", user.total_tasks_completed >= 500),
        ("perfect_day", "Идеальный день", user.perfect_days >= 1),
        ("perfect_week", "Идеальная неделя", user.perfect_weeks >= 1),
        ("perfect_month", "Идеальный месяц", user.perfect_months >= 1),
    ]

    for rid, name, cond in rewards:
        if cond and rid not in current:
            current.append(rid)
            new_ones.append(name)

    user.achievements = current
    return new_ones


@app.before_request
def log_request():
    logger.info(">>> %s %s", request.method, request.path)


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


# ====================== API ENDPOINTS ======================


@app.route("/game/g_health", methods=["GET"])
def g_health():
    return jsonify({"status": "ok"}), 200


@app.route("/game/streak/check", methods=["POST"])
@jwt_required
def check_streak():
    data = request.get_json() or {}

    # Источник истины — JWT, а не тело запроса: берём user_id из токена,
    # чтобы клиент не мог действовать от имени другого пользователя.
    # Если тело всё же прислало user_id — он игнорируется (а в случае
    # несовпадения возвращаем 403).
    user_id = g.current_user_id
    body_user_id = data.get("user_id")
    if body_user_id is not None and int(body_user_id) != user_id:
        return jsonify({"error": "user_id does not match token"}), 403

    # FIX #3: валидация входных данных
    if not user_id:
        return jsonify({"error": "user_id is required"}), 400

    # FIX #2: db.session.get() вместо депрекейтед Query.get()
    user = db.session.get(UserGamification, user_id)
    if not user:
        # FIX #10: НЕ лезем в таблицу `users` — она принадлежит Go-app сервису
        # и может быть ещё не создана/недоступна из этого контейнера.
        # Если записи геймификации нет — просто создаём её. FK-валидация
        # сработает естественным образом на INSERT, и мы поймаем
        # IntegrityError ниже как «User not found» вместо 500.
        user = UserGamification(user_id=user_id)
        db.session.add(user)

    today = date.today()

    # Идемпотентная проверка — уже был чек сегодня
    try:
        already = UserActivity.query.filter_by(
            user_id=user.user_id, activity_date=today
        ).first()
    except (OperationalError, ProgrammingError) as e:
        db.session.rollback()
        logger.exception("DB unavailable in check_streak: %s", e)
        return jsonify({"error": "Database error"}), 500
    if already:
        return jsonify(
            {
                "message": "Already checked today",
                "current_streak": user.current_streak,
            }
        )

    old_streak = user.current_streak
    check_and_update_streak(user)

    # FIX #4: race condition — ловим IntegrityError на уникальный constraint
    # и на FK, если user_id не существует в users
    try:
        db.session.add(UserActivity(user_id=user.user_id, activity_date=today))
        db.session.commit()
    except IntegrityError as e:
        db.session.rollback()
        # FK violation = user_id отсутствует в таблице users
        if "foreign key" in (str(e.orig).lower() if e.orig else "") or "1452" in str(e):
            return jsonify({"error": "User not found"}), 404
        # Уникальный constraint = уже чекнули сегодня
        return jsonify(
            {
                "message": "Already checked today",
                "current_streak": old_streak,
            }
        )
    # FIX #8: общий обработчик ошибок БД
    except (OperationalError, ProgrammingError) as e:
        db.session.rollback()
        logger.exception("DB error in check_streak: %s", e)
        return jsonify({"error": "Database error"}), 500
    except Exception as e:
        db.session.rollback()
        logger.exception("Unexpected error in check_streak: %s", e)
        return jsonify({"error": "Internal error"}), 500

    return jsonify(
        {
            "message": "Streak updated",
            "previous_streak": old_streak,
            "current_streak": user.current_streak,
            "longest_streak": user.longest_streak,
        }
    )


@app.route("/game/task/complete", methods=["POST"])
@jwt_required
def complete_task():
    data = request.get_json() or {}
    user_id = g.current_user_id
    day_closed = data.get("day_closed", False)

    # user_id всегда берётся из JWT; если в теле прислали чужой — 403.
    body_user_id = data.get("user_id")
    if body_user_id is not None and int(body_user_id) != user_id:
        return jsonify({"error": "user_id does not match token"}), 403

    # FIX #3: валидация — раньше отсутствовала
    if not user_id:
        return jsonify({"error": "user_id is required"}), 400

    # FIX #2: db.session.get() вместо депрекейтед Query.get()
    user = db.session.get(UserGamification, user_id)
    if not user:
        # FIX #10: создаём запись геймификации на лету, как в check_streak
        user = UserGamification(user_id=user_id)
        db.session.add(user)
        db.session.flush()

    user.total_tasks_completed += 1
    check_and_update_perfect_streak(user, day_closed)
    new_achievements = get_new_achievements(user)

    # FIX #8: обработка ошибок БД
    try:
        db.session.commit()
    except IntegrityError as e:
        db.session.rollback()
        if "foreign key" in (str(e.orig).lower() if e.orig else "") or "1452" in str(e):
            return jsonify({"error": "User not found"}), 404
        logger.exception("Integrity error in complete_task: %s", e)
        return jsonify({"error": "Database error"}), 500
    except (OperationalError, ProgrammingError) as e:
        db.session.rollback()
        logger.exception("DB error in complete_task: %s", e)
        return jsonify({"error": "Database error"}), 500
    except Exception as e:
        db.session.rollback()
        logger.exception("Unexpected error in complete_task: %s", e)
        return jsonify({"error": "Internal error"}), 500

    return jsonify(
        {
            "message": "Task completed",
            "total_tasks": user.total_tasks_completed,
            "new_achievements": new_achievements,
        }
    )


# ====================== ENTRYPOINT ======================
if __name__ == "__main__":
    # FIX #9: db.create_all() работает при запуске через __main__,
    # но НЕ при gunicorn. Для продакшена используй Flask-Migrate:
    #   flask db init / flask db migrate / flask db upgrade
    # with app.app_context():
    #     db.create_all()

    port = int(os.getenv("GAME_PORT", 8083))

    # FIX #1: debug=True убран — теперь управляется через переменную окружения
    debug = os.getenv("FLASK_DEBUG", "false").lower() == "true"
    app.run(host="0.0.0.0", port=port, debug=debug)
