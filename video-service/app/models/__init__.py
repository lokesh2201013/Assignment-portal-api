from dataclasses import dataclass


@dataclass(slots=True)
class Video:
    id: int
    url: str
    title: str = ""
    description: str | None = None
    status: str = "pending"
