import os
import shutil
import subprocess
import tempfile
import logging
from datetime import timedelta
from google.cloud import storage

from app.config import get_settings
from app.exceptions import NotFoundError, ExternalServiceError, AppError
from app.repositories.video import get_video_repository
from app.utils.utils import download_from_gcs, upload_folder_to_gcs

logger = logging.getLogger(__name__)


def ProcessVideo(video_id: int):
    logger.info(f"Starting FFMPEG processing for video ID: {video_id}")
    repo = get_video_repository()
    settings = get_settings()

    video = repo.get_by_id(video_id)
    if not video:
        logger.error(f"Video {video_id} not found in database.")
        raise NotFoundError(f"Video {video_id} not found")

    repo.update_status(video_id, "processing")

    temp_dir = tempfile.mkdtemp()
    try:
        local_raw = os.path.join(temp_dir, "input.mp4")
        hls_output = os.path.join(temp_dir, "hls")

        os.makedirs(hls_output, exist_ok=True)
        for i in range(3):
            os.makedirs(os.path.join(hls_output, f"v{i}"), exist_ok=True)

        download_from_gcs(video.url, local_raw)

        ffmpeg_cmd = [
            "ffmpeg", "-i", local_raw,
            "-map", "0:v:0", "-map", "0:a:0",
            "-c:v", "libx264", "-c:a", "aac", "-strict", "-2",
            "-b:v:1000k", "-s", "1280x720",
            "-b:v:500k",  "-s", "854x480",
            "-b:v:300k",  "-s", "640x360",
            "-f", "hls",
            "-hls_time", "6",
            "-hls_playlist_type", "vod",
            "-hls_segment_filename", os.path.join(hls_output, "v%v/seg_%03d.ts"),
            "-master_pl_name", "master.m3u8",
            "-var_stream_map", "v:0,a:0 v:1,a:0 v:2,a:0",
            os.path.join(hls_output, "v%v/playlist.m3u8"),
        ]

        logger.info(f"Running FFmpeg: {' '.join(ffmpeg_cmd)}")
        subprocess.run(ffmpeg_cmd, check=True, capture_output=True, text=True, timeout=600)

        gcs_output_prefix = f"chunked/{video.id}"
        upload_folder_to_gcs(hls_output, gcs_output_prefix)

        repo.update_status(video_id, "completed")

        try:
            storage_client = storage.Client()
            bucket = storage_client.bucket(settings.bucket_name)
            blob = bucket.blob(f"{gcs_output_prefix}/master.m3u8")
            signed_url = blob.generate_signed_url(
                version="v4",
                expiration=timedelta(hours=settings.gcs_signed_url_hours),
                method="GET",
            )
            return {"message": "Processing complete", "play_url": signed_url}
        except Exception as e:
            logger.error(f"Failed to generate signed url: {e}")
            return {"message": "Processing complete", "play_url": ""}

    except subprocess.CalledProcessError as e:
        logger.error(f"FFmpeg failed with error: {e.stderr}")
        repo.update_status(video_id, "failed")
        raise ExternalServiceError("Video transcoding failed") from e
    except subprocess.TimeoutExpired as e:
        logger.error(f"FFmpeg timed out for video {video_id}")
        repo.update_status(video_id, "failed")
        raise ExternalServiceError("Video transcoding timed out") from e
    except (NotFoundError, ExternalServiceError):
        raise
    except Exception as e:
        logger.exception(f"Unexpected error during video processing for {video_id}: {e}")
        repo.update_status(video_id, "failed")
        raise AppError("Internal processing error") from e
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)
