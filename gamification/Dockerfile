# ===================== BUILDER =====================
FROM python:3.11-alpine AS builder

WORKDIR /app

# Устанавливаем только build-зависимости — они останутся в builder-слое
RUN apk add --no-cache \
    gcc \
    musl-dev \
    mariadb-dev \
    pkgconfig

COPY requirements.txt .

# Собираем wheels для последующей установки без компилятора
RUN pip wheel --no-cache-dir --no-deps --wheel-dir /wheels -r requirements.txt


# ===================== PRODUCTION =====================
FROM python:3.11-alpine AS production

# Только runtime-библиотека — без компилятора и заголовочных файлов
RUN apk add --no-cache mariadb-connector-c

# Создаём пользователя ДО копирования файлов
RUN adduser -D -h /home/app app

WORKDIR /home/app

# Устанавливаем зависимости из wheels и сразу удаляем их
COPY --from=builder /wheels /wheels
RUN pip install --no-cache /wheels/* && rm -rf /wheels

# Копируем код с правильным владельцем сразу
COPY --chown=app:app . .

USER app

EXPOSE 8083

CMD ["python", "main.py"]
