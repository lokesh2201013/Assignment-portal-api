import os
import shutil
import subprocess
import tempfile
from datetime import timedelta

from google.cloud import storage
from fastapi import HTTPException

from app.config import get_settings
from app.exceptions import NotFoundError
from app.repositories.video import get_video_repository
from app.utils.utils import download_from_gcs, upload_folder_to_gcs


def ProcessVideo(video_id: int):
    video = get_video_repository().get_by_id(video_id)
    if not video:
        raise NotFoundError(f"Video {video_id} not found")

    settings = get_settings()
    with tempfile.TemporaryDirectory() as temp_dir:
        local_raw = os.path.join(temp_dir, "input.mp4")
        hls_output = os.path.join(temp_dir, "hls")
        os.makedirs(hls_output, exist_ok=True)
        download_from_gcs(video.url, local_raw)

        ffmpeg_cmd = [
            "ffmpeg", "-i", local_raw,
            "-map", "0:v:0", "-map", "0:a:0",
            "-c:v", "h264", "-c:a", "aac", "-strict", "-2",
            "-b:v:1000k", "-s", "1280x720",
            "-b:v:500k", "-s", "854x480",
            "-b:v:300k", "-s", "640x360",
            "-f", "hls",
            "-hls_time", "6",
            "-hls_playlist_type", "vod",
            "-hls_segment_filename", os.path.join(hls_output, "v%v/seg_%03d.ts"),
            "-master_pl_name", "master.m3u8",
            "-var_stream_map", "v:0,a:0 v:1,a:0 v:2,a:0",
            os.path.join(hls_output, "v%v/playlist.m3u8"),
        ]
        subprocess.run(ffmpeg_cmd, check=True)

        gcs_output_prefix = f"chunked/{video.id}"
        upload_folder_to_gcs(hls_output, gcs_output_prefix)

    try:
        client = storage.Client()
        blob = client.bucket(settings.bucket_name).blob(
            f"{gcs_output_prefix}/master.m3u8"
        )
        signed_url = blob.generate_signed_url(
            version="v4",
            expiration=timedelta(hours=settings.gcs_signed_url_hours),
            method="GET",
        )
    except Exception as exc:
        raise HTTPException(status_code=502, detail="Failed to generate playback URL") from exc

    return {"message": "Processing complete", "play_url": signed_url}
