from app.models.models import Video
from app.database.database import conn
import os
import shutil
import subprocess
import tempfile
from fastapi import APIRouter, HTTPException
from google.cloud import storage
from datetime import timedelta
from app.utils.utils import download_from_gcs, upload_folder_to_gcs

storage_client = storage.Client()
BUCKET_NAME = "my-videos-bucket"


def ProcessVideo(video_id: int):
    
    curr = conn.cursor()
    curr.execute("SELECT id, url, title, description, status FROM videos WHERE id = %s", (video_id,))
    row = curr.fetchone()
    curr.close()

    if not row:
        raise HTTPException(status_code=404, detail="Video not found")

    video = Video(
        id=row[0],
        url=row[1], 
        title=row[2],
        description=row[3],
        status=row[4]
    )

    # 2. Create temp dir for processing
    temp_dir = tempfile.mkdtemp()
    local_raw = os.path.join(temp_dir, "input.mp4")
    hls_output = os.path.join(temp_dir, "hls")

    os.makedirs(hls_output, exist_ok=True)

    
    download_from_gcs(video.url, local_raw)

    master_playlist = os.path.join(hls_output, "master.m3u8")

    ffmpeg_cmd = [
        "ffmpeg", "-i", local_raw,
        "-map", "0:v:0", "-map", "0:a:0",
        "-c:v", "h264", "-c:a", "aac", "-strict", "-2",
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

    subprocess.run(ffmpeg_cmd, check=True)

    
    gcs_output_prefix = f"chunked/{video.id}"
    upload_folder_to_gcs(hls_output, gcs_output_prefix)

    # 6. Cleanup temp files
    shutil.rmtree(temp_dir)
    
    bucket = storage_client.bucket(BUCKET_NAME)
    blob = bucket.blob(f"{gcs_output_prefix}/master.m3u8")
    signed_url = blob.generate_signed_url(
        version="v4",
        expiration=timedelta(hours=1),
        method="GET"
    )

    return {"message": "Processing complete", "play_url": signed_url}
