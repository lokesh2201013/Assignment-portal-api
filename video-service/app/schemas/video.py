from pydantic import BaseModel, Field


class PlayUrlResponse(BaseModel):
    play_url: str


class ProcessVideoResult(BaseModel):
    message: str
    play_url: str = ""


class HealthResponse(BaseModel):
    status: str
    service: str
