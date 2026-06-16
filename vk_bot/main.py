import os
import json
from fastapi import FastAPI, Request
from fastapi.responses import Response
import vk_api

# 💡 Подтягиваем токены напрямую из Docker-окружения (из вашего docker-compose.yml)
TOKEN = os.getenv("VK_API_TOKEN")
CONFIRMATION_TOKEN = os.getenv("VK_BOT_TOKEN")

# Проверка, что переменные вообще дошли до контейнера
if not TOKEN or not CONFIRMATION_TOKEN:
    raise ValueError("Критическая ошибка: Переменные VK_API_TOKEN или VK_CONFIRMATION_TOKEN не заданы в .env/docker-compose!")

# Инициализация API ВКонтакте
vk_session = vk_api.VkApi(token=TOKEN)
vk = vk_session.get_api()

# Инициализация веб-сервера FastAPI
app = FastAPI()

keyboard = {
    "inline": True,
    "buttons": [
        [{"action": {"type": "text", "label": "Не имеет значения"}, "color": "secondary"}],
        [
            {"action": {"type": "text", "label": "16:00"}, "color": "secondary"},
            {"action": {"type": "text", "label": "17:00"}, "color": "secondary"}
        ],
        [
            {"action": {"type": "text", "label": "20:00"}, "color": "secondary"},
            {"action": {"type": "text", "label": "22:00"}, "color": "secondary"}
        ]
    ]
}

def get_answer(text):
    text = text.lower()

    if text == "привет":
        return "Привет 👋"

    elif text == "помощь":
        return "Команды:\n- Привет\n- Помощь\n- Пока"

    elif text == "пока":
        return "Пока 👋"

    return "Нажми кнопку 😊"


# 💡 Путь строго соответствует Варианту 1 (совпадает с location в Nginx)
@app.post("/api/vk/webhook")
async def vk_callback(request: Request):
    data = await request.json()
    
    # 1. Запрос на подтверждение сервера от ВК
    if data.get("type") == "confirmation":
        return Response(content=CONFIRMATION_TOKEN, media_type="text/plain")

    # 2. Обработка входящего сообщения
    if data.get("type") == "message_new":
        message_object = data["object"]["message"]
        peer_id = message_object["peer_id"]
        text = message_object["text"].strip().lower()

        try:
            if text == "67":
                vk.messages.send(
                    peer_id=peer_id,
                    random_id=0,
                    message="Наслаждайся 😊",
                    attachment="doc-239238045_702738792"
                )
            else:
                answer = get_answer(text)

                vk.messages.send(
                    peer_id=peer_id,
                    random_id=0,
                    message=answer,
                    keyboard=json.dumps(keyboard, ensure_ascii=False)
                )

        except Exception as e:
            print(f"Ошибка отправки сообщения через API ВК: {e}")

        return Response(content="ok", media_type="text/plain")