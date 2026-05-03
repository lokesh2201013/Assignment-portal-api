import os
import sys
import asyncio
import logging
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import uvicorn

# Set up global logging configuration
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

from app.database.rabbitmq import start_consumer
from app.routes.routes import router as video_router
from app.controllers.chatcontrollers import chat_router

# Lifespan context manager for startup and shutdown events
@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Initializing Video Service...")
    # Start RabbitMQ consumer in the background
    consumer_task = asyncio.create_task(start_consumer())
    logger.info("RabbitMQ Consumer task started.")
    
    yield
    
    logger.info("Shutting down Video Service...")
    consumer_task.cancel()
    

app = FastAPI(title="Video Processing & Chat Service", lifespan=lifespan)

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
