from functools import lru_cache

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    db_host: str = "localhost"
    db_port: int = 5432
    db_user: str = "postgres"
    db_password: str = ""
    db_name: str = "videos"
    db_min_conn: int = 1
    db_max_conn: int = 20

    rabbitmq_url: str = "amqp://guest:guest@rabbitmq:5672/"
    rabbitmq_queue: str = "task_queue"

    bucket_name: str = "my-videos-bucket"
    gcs_signed_url_hours: int = 1

    admin_api_key: str = ""
    cors_origins: str = "*"
    max_chat_users: int = 50
    log_level: str = "INFO"
    start_worker: bool = True

    @field_validator("cors_origins", mode="before")
    @classmethod
    def _normalize_origins(cls, value: str) -> str:
        return value.strip() if isinstance(value, str) else "*"

    @property
    def cors_origin_list(self) -> list[str]:
        if self.cors_origins == "*":
            return ["*"]
        return [origin.strip() for origin in self.cors_origins.split(",") if origin.strip()]


@lru_cache
def get_settings() -> Settings:
    return Settings()
