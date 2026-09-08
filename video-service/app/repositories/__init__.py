from typing import Protocol

from app.models import Video


class VideoRepository(Protocol):
    def get_by_id(self, video_id: int) -> Video | None: ...
    def update_status(self, video_id: int, status: str) -> None: ...
