from fastapi import APIRouter, Request, HTTPException
from app.models.models import Video
from app.database.database import get_db_connection
from google.cloud import storage
import logging
from datetime import timedelta
import os

logger = logging.getLogger(__name__)

router = APIRouter()

BUCKET_NAME = os.getenv("BUCKET_NAME", "my-videos-bucket")

@router.get("/video/serve/{video_id}")
async def serve_video(video_id: int):
    logger.info(f"Serve video requested for ID: {video_id}")
    try:
        client = storage.Client()
        bucket = client.bucket(BUCKET_NAME)
        blob = bucket.blob(f"chunked/{video_id}/master.m3u8")

        signed_url = blob.generate_signed_url(
            version="v4",
            expiration=timedelta(hours=1),
            method="GET",
        )

        return {"play_url": signed_url}
    except Exception as e:
        logger.exception(f"Error serving video URL for id {video_id}: {e}")
        raise HTTPException(status_code=500, detail="Failed to generate playback URL")
