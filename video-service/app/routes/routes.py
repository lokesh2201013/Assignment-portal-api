import logging
from datetime import timedelta
from fastapi import APIRouter
from google.cloud import storage

from app.config import get_settings
from app.exceptions import NotFoundError, ExternalServiceError
from app.repositories.video import get_video_repository

logger = logging.getLogger(__name__)

router = APIRouter()


@router.get("/video/serve/{video_id}")
async def serve_video(video_id: int):
    logger.info(f"Serve video requested for ID: {video_id}")
    repo = get_video_repository()
    settings = get_settings()

    video = repo.get_by_id(video_id)
    if not video:
        logger.warning(f"Video {video_id} not found in database.")
        raise NotFoundError(f"Video {video_id} not found")

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
