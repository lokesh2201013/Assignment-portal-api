from fastapi import APIRouter, Req , HTTPException
from app.models.models import Video
from app.database.database import conn
from google.cloud import storage
from datetime import timedelta


router = APIRouter()


@router.get("/video/serve/{video_id}")
async def serve_video(video_id: int):
    client = storage.Client()
    bucket = client.bucket("my-videos")
    blob = bucket.blob(f"chunked/{video_id}/master.m3u8")

    signed_url = blob.generate_signed_url(
        version="v4",
        expiration=timedelta(hours=1),
        method="GET",
    )

    return {"play_url": signed_url}
