import os
import sys
import asyncio
import logging
import json
import uuid
from contextvars import ContextVar
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
import uvicorn

# Request ID Context Var
request_id_context: ContextVar[str] = ContextVar("request_id", default="-")

class JSONFormatter(logging.Formatter):
    def format(self, record):
        log_record = {
            "time": self.formatTime(record, self.datefmt),
            "name": record.name,
            "level": record.levelname,
            "message": record.getMessage(),
            "request_id": request_id_context.get()
        }
        if record.exc_info:
            log_record["exception"] = self.formatException(record.exc_info)
        return json.dumps(log_record)

# Set up global logging configuration
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(JSONFormatter())
logging.root.handlers = [handler]
logging.root.setLevel(logging.INFO)

logger = logging.getLogger(__name__)

from app.database.rabbitmq import start_consumer
from app.database.database import init_db_pool
from app.routes.routes import router as video_router
from app.controllers.chatcontrollers import chat_router

# Lifespan context manager for startup and shutdown events
@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Initializing Video Service...")
    init_db_pool()
    # Start RabbitMQ consumer in the background
    consumer_task = asyncio.create_task(start_consumer())
    logger.info("RabbitMQ Consumer task started.")
    
    yield
    
    logger.info("Shutting down Video Service...")
    consumer_task.cancel()
    

app = FastAPI(title="Video Processing & Chat Service", lifespan=lifespan)

class RequestIDMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        req_id = request.headers.get("X-Request-ID") or str(uuid.uuid4())
        request_id_context.set(req_id)
        response = await call_next(request)
        response.headers["X-Request-ID"] = req_id
        return response

app.add_middleware(RequestIDMiddleware)

# Setup CORS, optional but standard
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(video_router)
app.include_router(chat_router)

@app.get("/health")
async def combined_health_check():
    """General health check for the entire service"""
    return {"status": "ok", "service": "video-service"}

if __name__ == '__main__':
    # When running directly use the module form for uvicorn
    uvicorn.run("app.app:app", host="0.0.0.0", port=5000, reload=True)
