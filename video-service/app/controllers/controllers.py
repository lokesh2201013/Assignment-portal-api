import os
import shutil
import subprocess
import tempfile
import logging
from fastapi import HTTPException
from google.cloud import storage
from datetime import timedelta

from app.models.models import Video
from app.database.database import get_db_connection
from app.utils.utils import download_from_gcs, upload_folder_to_gcs

logger = logging.getLogger(__name__)

BUCKET_NAME = os.getenv("BUCKET_NAME", "my-videos-bucket")

def ProcessVideo(video_id: int):
    logger.info(f"Starting FFMPEG processing for video ID: {video_id}")
    
    video = None
    with get_db_connection() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT id, url, title, description, status FROM videos WHERE id = %s", (video_id,))
            row = cur.fetchone()
            
            if not row:
                logger.error(f"Video {video_id} not found in database.")
                raise HTTPException(status_code=404, detail="Video not found")
                
            video = Video(
                id=row[0],
                url=row[1], 
                title=row[2],
                description=row[3],
                status=row[4]
            )
            
            # Update status to processing
            cur.execute("UPDATE videos SET status = %s WHERE id = %s", ("processing", video_id))
            conn.commit()

    # Create temp dir for processing
    temp_dir = tempfile.mkdtemp()
    try:
        local_raw = os.path.join(temp_dir, "input.mp4")
        hls_output = os.path.join(temp_dir, "hls")
        
        # FFmpeg will need the targeted base directory and stream directories
        os.makedirs(hls_output, exist_ok=True)
        for i in range(3):
            os.makedirs(os.path.join(hls_output, f"v{i}"), exist_ok=True)
            
        download_from_gcs(video.url, local_raw)

        master_playlist = os.path.join(hls_output, "master.m3u8")

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
            os.path.join(hls_output, "v%v/playlist.m3u8")
        ]

        logger.info(f"Running FFmpeg: {' '.join(ffmpeg_cmd)}")
        subprocess.run(ffmpeg_cmd, check=True, capture_output=True, text=True)

        gcs_output_prefix = f"chunked/{video.id}"
        upload_folder_to_gcs(hls_output, gcs_output_prefix)

        # Update status to completed
        with get_db_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("UPDATE videos SET status = %s WHERE id = %s", ("completed", video_id))
                conn.commit()
                
        # Generate URL
        try:
            storage_client = storage.Client()
            bucket = storage_client.bucket(BUCKET_NAME)
            blob = bucket.blob(f"{gcs_output_prefix}/master.m3u8")
            signed_url = blob.generate_signed_url(
                version="v4",
                expiration=timedelta(hours=1),
                method="GET"
            )
            return {"message": "Processing complete", "play_url": signed_url}
        except Exception as e:
            logger.error(f"Failed to generate signed url: {e}")
            return {"message": "Processing complete", "play_url": ""}

    except subprocess.CalledProcessError as e:
        logger.error(f"FFmpeg failed with error: {e.stderr}")
        with get_db_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("UPDATE videos SET status = %s WHERE id = %s", ("failed", video_id))
                conn.commit()
        raise HTTPException(status_code=500, detail="Video processing failed")
    except Exception as e:
        logger.exception(f"Unexpected error during video processing for {video_id}: {e}")
        with get_db_connection() as conn:
            with conn.cursor() as cur:
                cur.execute("UPDATE videos SET status = %s WHERE id = %s", ("failed", video_id))
                conn.commit()
        raise HTTPException(status_code=500, detail="Internal processing error")
    finally:
        # Cleanup temp files
        shutil.rmtree(temp_dir, ignore_errors=True)
