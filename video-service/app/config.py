import os
from dataclasses import dataclass
from functools import lru_cache


def _get_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return int(value)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc


@dataclass(frozen=True)
class Settings:
    db_host: str
    db_port: int
    db_user: str
    db_password: str
    db_name: str
    db_min_conn: int
    db_max_conn: int
    bucket_name: str
    gcs_signed_url_hours: int
    rabbitmq_url: str
    rabbitmq_queue: str
    start_worker: bool
    admin_api_key: str


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings(
        db_host=os.getenv("DB_HOST", "localhost"),
        db_port=_get_int("DB_PORT", 5432),
        db_user=os.getenv("DB_USER", ""),
        db_password=os.getenv("DB_PASSWORD", ""),
        db_name=os.getenv("DB_NAME", ""),
        db_min_conn=_get_int("DB_MIN_CONN", 1),
        db_max_conn=_get_int("DB_MAX_CONN", 10),
        bucket_name=os.getenv("BUCKET_NAME", "my-videos-bucket"),
        gcs_signed_url_hours=_get_int("GCS_SIGNED_URL_HOURS", 1),
        rabbitmq_url=os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost/"),
        rabbitmq_queue=os.getenv("RABBITMQ_QUEUE", "task_queue"),
        start_worker=os.getenv("START_WORKER", "").strip().lower() in {"1", "true", "yes"},
        admin_api_key=os.getenv("ADMIN_API_KEY", ""),
    )
