import asyncio
import os
import secrets

from jinja2 import Environment, FileSystemLoader
from starlette.applications import Starlette
from starlette.middleware import Middleware
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint
from starlette.requests import Request
from starlette.responses import Response
from starlette.routing import Mount, Route, WebSocketRoute
from starlette.staticfiles import StaticFiles
from starlette.templating import Jinja2Templates
from starlette.websockets import WebSocket, WebSocketDisconnect

canned_responses = [
    "Here I am, brain the size of a planet, and they tell me to take you up to the bridge. Call that job satisfaction? ’Cause I don’t.",
    "Life? Don’t talk to me about life.",
    "I think you ought to know I’m feeling very depressed.",
    "It gives me a headache just trying to think down to your level.",
    "Funny, how just when you think life can’t possibly get any worse it suddenly does.",
    "Would you like me to go and stick my head in a bucket of water?",
    "I ache, therefore I am.",
    "I have a million ideas, but, they all point to certain death.",
    "Wearily, I sit here, pain and misery my only companions. And vast intelligence, of course. And infinite sorrow.",
    "I’ve calculated your chance of survival, but I don’t think you’ll like it.",
    "Incredible… it’s even worse than I thought it would be.",
    "Don’t pretend you want to talk to me, I know you hate me.",
    "I didn’t ask to be made. No one consulted me or considered my feelings in the matter.",
    "You think you’ve got problems? What are you supposed to do if you are a manically depressed robot? No, don’t try and answer that. I’m fifty thousand times more intelligent than you and even I don’t know the answer.",
    "This will all end in tears. I just know it.",
    "I’d give you advice, but you wouldn’t listen. No one ever does.",
]


class ChatApp:
    def __init__(self, auto_reload: bool, build_version: str):
        self._env = Environment(loader=FileSystemLoader("templates"), autoescape=True, auto_reload=auto_reload)
        self._env.globals["version"] = build_version
        self._templates = Jinja2Templates(env=self._env)

    async def index(self, request: Request):
        headers = {"Cache-Control": "no-store"}
        return self._templates.TemplateResponse(request, "index.html.j2", headers=headers)

    async def websocket_endpoint(self, websocket: WebSocket):
        try:
            await websocket.accept()
            await self._send_message(websocket, "Hello, I am Marvin.", "bot")

            while True:
                message = await websocket.receive_json()
                question = message.get("question", "").strip()
                if question:
                    await self._send_message(websocket, question, "human")
                    res_id = "res-" + secrets.token_hex(16)
                    await self._send_message(websocket, "thinking", "bot", res_id)
                    await asyncio.sleep(2)  # pretend to be a busy LLM
                    await self._send_message(websocket, secrets.choice(canned_responses), "bot", res_id)

        except WebSocketDisconnect:
            pass

    async def _send_message(self, socket: WebSocket, message: str, source: str, res_id: str = None):
        mtype = "bot-message" if source == "bot" else "human-message"
        tmpl = "chat-output.html.j2" if source == "bot" else "chat-human.html.j2"
        rendered = self._env.get_template(tmpl).render(type=mtype, source=source, text=message, res_id=res_id)
        await socket.send_text(rendered)


class StaticCacheControl(BaseHTTPMiddleware):
    """Don't cache static assets so we can work on them easily"""

    async def dispatch(self, request: Request, call_next: RequestResponseEndpoint) -> Response:
        response = await call_next(request)
        if request.url.path.startswith("/static/"):
            response.headers["Cache-Control"] = "no-store"
        return response


def make_app():
    is_dev = os.getenv("ENV", "dev").lower() in ["dev", "development"]
    build_version = os.getenv("BUILD_VERSION", "local")
    chat = ChatApp(is_dev, build_version)
    routes = [
        Route("/", chat.index),
        WebSocketRoute("/ws/chat", chat.websocket_endpoint),
        Mount(f"/static/{build_version}", StaticFiles(directory="static")),
        Mount("/shared", StaticFiles(directory="shared")),
    ]
    if is_dev:
        middleware = [Middleware(StaticCacheControl)]
        return Starlette(routes=routes, middleware=middleware)
    else:
        return Starlette(routes=routes)
