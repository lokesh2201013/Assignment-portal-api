import asyncio
import logging
import os
import uuid
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
import uvicorn

from app.config import get_settings
from app.database.postgres import get_database
from app.database.rabbitmq import start_consumer
from app.exceptions import AppError
from app.logging_setup import configure_logging, request_id_context
from app.routes.routes import router as video_router
from app.controllers.chatcontrollers import chat_router

settings = get_settings()
configure_logging(settings.log_level)
logger = logging.getLogger(__name__)


def configure_tracing() -> TracerProvider:
    endpoint = os.getenv("JAEGER_ENDPOINT") or os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT") or "http://localhost:4318"
    resource = Resource.create({"service.name": "video-service"})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=f"{endpoint.rstrip('/')}/v1/traces", headers={})
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    return provider


configure_tracing()


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Initializing Video Service...")
    db = get_database()
    db.connect()

    consumer_task = None
    if settings.start_worker:
        consumer_task = asyncio.create_task(start_consumer())
        logger.info("RabbitMQ Consumer task started.")

    yield

    logger.info("Shutting down Video Service...")
    if consumer_task:
        consumer_task.cancel()
        try:
            await consumer_task
        except asyncio.CancelledError:
            pass

    db.close()
    logger.info("Video Service shutdown complete.")


app = FastAPI(
    title="Video Processing & Chat Service",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
    openapi_url="/openapi.json",
    lifespan=lifespan,
)
FastAPIInstrumentor.instrument_app(app)


class RequestIDMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        req_id = request.headers.get("X-Request-ID") or str(uuid.uuid4())
        request_id_context.set(req_id)
        response = await call_next(request)
        response.headers["X-Request-ID"] = req_id
        return response


app.add_middleware(RequestIDMiddleware)

# CORS middleware using configured origins
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origin_list,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Global Exception Handlers for AppError and generic unhandled Exception
@app.exception_handler(AppError)
async def app_error_handler(request: Request, exc: AppError):
    return JSONResponse(
        status_code=exc.status_code,
        content={"error": exc.error_type, "message": exc.message},
    )


@app.exception_handler(Exception)
async def generic_exception_handler(request: Request, exc: Exception):
    logger.exception(f"Unhandled server error on {request.method} {request.url.path}: {exc}")
    return JSONResponse(
        status_code=500,
        content={"error": "internal", "message": "An unexpected error occurred"},
    )


# Include routers
app.include_router(video_router)
app.include_router(chat_router)


@app.get("/health")
async def combined_health_check():
    """General health check for the entire service"""
    return {"status": "ok", "service": "video-service"}


if __name__ == "__main__":
    uvicorn.run("app.app:app", host="0.0.0.0", port=5000, reload=True)
