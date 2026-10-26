import logging
import hmac
from datetime import timedelta

from fastapi import APIRouter, Depends, Header, HTTPException, status
from pydantic import BaseModel, Field
from google.cloud import storage

from app.config import get_settings
from app.exceptions import NotFoundError, ExternalServiceError
from app.repositories.video import get_video_repository

logger = logging.getLogger(__name__)

router = APIRouter()

class ProgressUpdate(BaseModel):
    learner_id: str = Field(min_length=1, max_length=255)
    resume_position_seconds: float = Field(ge=0)
    completed: bool = False

class CourseCreate(BaseModel):
    title: str = Field(min_length=1, max_length=255)
    description: str = Field(default="", max_length=5000)
    status: str = Field(default="draft", pattern="^(draft|active|archived)$")

class CourseUpdate(BaseModel):
    title: str | None = Field(default=None, min_length=1, max_length=255)
    description: str | None = Field(default=None, max_length=5000)
    status: str | None = Field(default=None, pattern="^(draft|active|archived)$")

class LessonCreate(BaseModel):
    video_id: int
    title: str = Field(min_length=1, max_length=255)
    position: int = Field(default=0, ge=0)
    transcript: str = Field(default="", max_length=200000)

class LessonUpdate(BaseModel):
    title: str | None = Field(default=None, min_length=1, max_length=255)
    position: int | None = Field(default=None, ge=0)
    transcript: str | None = Field(default=None, max_length=200000)

class EnrollmentUpdate(BaseModel):
    learner_id: str = Field(min_length=1, max_length=255)

class IncidentCreate(BaseModel):
    kind: str = Field(pattern="^(ingestion|processing|delivery)$")
    detail: str = Field(default="", max_length=5000)

def require_admin(
    x_admin_api_key: str | None = Header(default=None, alias="X-Admin-API-Key"),
) -> None:
    if x_admin_api_key is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Admin API key is required")
    configured_key = get_settings().admin_api_key
    if not configured_key:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Admin API key is not configured")
    if not hmac.compare_digest(x_admin_api_key, configured_key):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid admin API key")

def require_learner(x_user_id: str | None = Header(default=None)) -> str:
    if not x_user_id:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="X-User-ID is required")
    return x_user_id

def require_course_access(course_id: int, learner_id: str) -> None:
    if not get_video_repository().is_enrolled(course_id, learner_id):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Course enrollment is required")

@router.get("/courses")
async def list_courses():
    return {"courses": get_video_repository().list_courses()}

@router.get("/courses/{course_id}/lessons")
async def list_lessons(course_id: int, x_user_id: str | None = Header(default=None)):
    require_course_access(course_id, require_learner(x_user_id))
    return {"course_id": course_id, "lessons": get_video_repository().list_lessons(course_id)}

@router.post("/admin/courses", status_code=status.HTTP_201_CREATED)
async def create_course(payload: CourseCreate, _: None = Depends(require_admin)):
    return get_video_repository().create_course(payload.title, payload.description, payload.status)

@router.patch("/admin/courses/{course_id}")
async def update_course(course_id: int, payload: CourseUpdate, _: None = Depends(require_admin)):
    return get_video_repository().update_course(course_id, payload.title, payload.description, payload.status)

@router.delete("/admin/courses/{course_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_course(course_id: int, _: None = Depends(require_admin)):
    get_video_repository().delete_course(course_id)

@router.post("/admin/courses/{course_id}/lessons", status_code=status.HTTP_201_CREATED)
async def create_lesson(course_id: int, payload: LessonCreate, _: None = Depends(require_admin)):
    if not get_video_repository().get_by_id(payload.video_id): raise NotFoundError(f"Video {payload.video_id} not found")
    return get_video_repository().create_lesson(course_id, payload.video_id, payload.title, payload.position, payload.transcript)

@router.patch("/admin/lessons/{lesson_id}")
async def update_lesson(lesson_id: int, payload: LessonUpdate, _: None = Depends(require_admin)):
    return get_video_repository().update_lesson(lesson_id, payload.title, payload.position, payload.transcript)

@router.delete("/admin/lessons/{lesson_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_lesson(lesson_id: int, _: None = Depends(require_admin)):
    get_video_repository().delete_lesson(lesson_id)

@router.put("/admin/courses/{course_id}/enrollments", status_code=status.HTTP_204_NO_CONTENT)
async def enroll(course_id: int, payload: EnrollmentUpdate, _: None = Depends(require_admin)):
    get_video_repository().enroll(course_id, payload.learner_id)

@router.delete("/admin/courses/{course_id}/enrollments/{learner_id}", status_code=status.HTTP_204_NO_CONTENT)
async def unenroll(course_id: int, learner_id: str, _: None = Depends(require_admin)):
    get_video_repository().unenroll(course_id, learner_id)

@router.get("/admin/courses/{course_id}/enrollments")
async def list_enrollments(course_id: int, _: None = Depends(require_admin)):
    return {"course_id": course_id, "enrollments": get_video_repository().list_enrollments(course_id)}

@router.get("/video/{video_id}/transcript")
async def get_transcript(video_id: int, x_user_id: str | None = Header(default=None)):
    lesson = get_video_repository().get_lesson(video_id)
    if not lesson: raise NotFoundError(f"Lesson for video {video_id} not found")
    require_course_access(lesson.course_id, require_learner(x_user_id))
    return {"video_id": video_id, "transcript": lesson.transcript}

@router.get("/video/{video_id}/progress/{learner_id}")
async def get_progress(video_id: int, learner_id: str, x_user_id: str | None = Header(default=None)):
    if learner_id != require_learner(x_user_id): raise HTTPException(status_code=403, detail="Cannot access another learner's progress")
    progress = get_video_repository().get_progress(video_id, learner_id)
    return progress or {"video_id": video_id, "learner_id": learner_id, "resume_position_seconds": 0, "completed_at": None}

@router.put("/video/{video_id}/progress")
async def save_progress(video_id: int, update: ProgressUpdate, x_user_id: str | None = Header(default=None)):
    if not get_video_repository().get_by_id(video_id): raise NotFoundError(f"Video {video_id} not found")
    lesson = get_video_repository().get_lesson(video_id)
    if not lesson: raise NotFoundError(f"Lesson for video {video_id} not found")
    if update.learner_id != require_learner(x_user_id): raise HTTPException(status_code=403, detail="Cannot update another learner's progress")
    require_course_access(lesson.course_id, update.learner_id)
    return get_video_repository().save_progress(video_id, update.learner_id, update.resume_position_seconds, update.completed)


@router.get("/video/serve/{video_id}")
async def serve_video(video_id: int, x_user_id: str | None = Header(default=None)):
    logger.info(f"Serve video requested for ID: {video_id}")
    repo = get_video_repository()
    settings = get_settings()

    video = repo.get_by_id(video_id)
    if not video:
        logger.warning(f"Video {video_id} not found in database.")
        raise NotFoundError(f"Video {video_id} not found")
    lesson = repo.get_lesson(video_id)
    if not lesson:
        raise NotFoundError(f"Lesson for video {video_id} not found")
    require_course_access(lesson.course_id, require_learner(x_user_id))

    try:
        client = storage.Client()
        bucket = client.bucket(settings.bucket_name)
        blob = bucket.blob(f"chunked/{video_id}/master.m3u8")

        signed_url = blob.generate_signed_url(
            version="v4",
            expiration=timedelta(hours=settings.gcs_signed_url_hours),
            method="GET",
        )

        return {"play_url": signed_url}
    except Exception as e:
        logger.exception(f"Error serving video URL for id {video_id}: {e}")
        raise ExternalServiceError("Failed to generate playback URL") from e

@router.post("/admin/videos/{video_id}/incidents", status_code=status.HTTP_201_CREATED)
async def create_incident(video_id: int, payload: IncidentCreate, _: None = Depends(require_admin)):
    if not get_video_repository().get_by_id(video_id): raise NotFoundError(f"Video {video_id} not found")
    return get_video_repository().create_incident(video_id, payload.kind, payload.detail)

@router.get("/admin/video-incidents")
async def list_incidents(status_filter: str | None = None, _: None = Depends(require_admin)):
    if status_filter not in (None, "open", "resolved"): raise HTTPException(status_code=400, detail="status_filter must be open or resolved")
    return {"incidents": get_video_repository().list_incidents(status_filter)}

@router.post("/admin/video-incidents/{incident_id}/resolve", status_code=status.HTTP_204_NO_CONTENT)
async def resolve_incident(incident_id: int, _: None = Depends(require_admin)):
    get_video_repository().resolve_incident(incident_id)
