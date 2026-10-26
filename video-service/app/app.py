import asyncio
import logging
from uuid import uuid4

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from app.config import get_settings
from app.controllers.chatcontrollers import chat_router
from app.exceptions import (
    ConflictError,
    DatabaseError,
    ExternalServiceError,
    NotFoundError,
    ValidationError,
)
from app.routes.routes import router as video_router

logger = logging.getLogger(__name__)


def create_app() -> FastAPI:
    settings = get_settings()
    application = FastAPI(title="Video Processing & Chat Service")
    worker_task: asyncio.Task | None = None
    application.include_router(video_router)
    application.include_router(chat_router)

    @application.on_event("startup")
    async def start_background_worker():
        nonlocal worker_task
        if settings.start_worker:
            from app.database.rabbitmq import start_consumer

            worker_task = asyncio.create_task(start_consumer())

    @application.on_event("shutdown")
    async def stop_background_worker():
        if worker_task is not None:
            worker_task.cancel()
            await asyncio.gather(worker_task, return_exceptions=True)

    @application.middleware("http")
    async def request_id_header(request: Request, call_next):
        request_id = request.headers.get("X-Request-ID") or str(uuid4())
        response = await call_next(request)
        response.headers["X-Request-ID"] = request_id
        return response

    @application.exception_handler(ValidationError)
    async def validation_error_handler(request: Request, exc: ValidationError):
        return JSONResponse(
            status_code=400,
            content={"error": "validation", "message": str(exc)},
        )

    @application.exception_handler(NotFoundError)
    async def not_found_error_handler(request: Request, exc: NotFoundError):
        return JSONResponse(
            status_code=404,
            content={"error": "not_found", "message": str(exc)},
        )

    @application.exception_handler(ConflictError)
    async def conflict_error_handler(request: Request, exc: ConflictError):
        return JSONResponse(
            status_code=409,
            content={"error": "conflict", "message": str(exc)},
        )

    @application.exception_handler(DatabaseError)
    async def database_error_handler(request: Request, exc: DatabaseError):
        logger.exception("Database request failed")
        return JSONResponse(
            status_code=500,
            content={"error": "database", "message": str(exc)},
        )

    @application.exception_handler(ExternalServiceError)
    async def external_service_error_handler(
        request: Request, exc: ExternalServiceError
    ):
        return JSONResponse(
            status_code=502,
            content={"error": "external_service", "message": str(exc)},
        )

    @application.exception_handler(Exception)
    async def unhandled_error_handler(request: Request, exc: Exception):
        logger.exception("Unhandled request error")
        return JSONResponse(
            status_code=500,
            content={"error": "internal", "message": "An unexpected error occurred"},
        )

    @application.get("/health")
    async def health_check():
        return {"status": "ok", "service": "video-service"}

    return application


app = create_app()


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app.app:app", host="0.0.0.0", port=5000)
