import os
import logging
from google.cloud import storage

# Configure basic logging for the module
logger = logging.getLogger(__name__)

BUCKET_NAME = "my-videos-bucket"
# Delay client initialization or initialized globally
try:
    storage_client = storage.Client()
except Exception as e:
    logger.warning(f"Could not initialize GCS client globally: {e}")
    storage_client = None

def download_from_gcs(gcs_path: str, local_path: str):
    """Download a file from Google Cloud Storage."""
    if not storage_client:
        logger.error("Storage client not initialized. Cannot download.")
        return
    
    bucket = storage_client.bucket(BUCKET_NAME)
    blob = bucket.blob(gcs_path)
    blob.download_to_filename(local_path)
    logger.info(f"Downloaded {gcs_path} → {local_path}")


def upload_folder_to_gcs(local_folder: str, gcs_prefix: str):
    """Upload an entire folder and its subdirectories to Google Cloud Storage."""
    if not storage_client:
        logger.error("Storage client not initialized. Cannot upload.")
        return
        
    bucket = storage_client.bucket(BUCKET_NAME)
    for root, _, files in os.walk(local_folder):
        for file in files:
            local_file = os.path.join(root, file)
            rel_path = os.path.relpath(local_file, local_folder)
            
            # GCS prefix formatting
            blob_path = f"{gcs_prefix}/{rel_path}".replace(os.sep, "/")
            blob = bucket.blob(blob_path)
            
            blob.upload_from_filename(local_file)
            logger.info(f"Uploaded {local_file} → gs://{BUCKET_NAME}/{blob_path}")
