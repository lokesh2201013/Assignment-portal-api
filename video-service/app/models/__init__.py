from dataclasses import dataclass


@dataclass(slots=True)
class Video:
    id: int
    url: str
    title: str = ""
    description: str | None = None
    status: str = "pending"


@dataclass(slots=True)
class Lesson:
    id: int
    course_id: int
    video_id: int
    title: str
    position: int
    transcript: str = ""
