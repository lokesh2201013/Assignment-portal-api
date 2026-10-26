import os

from google.cloud import storage

from app.config import get_settings


def _bucket():
    return storage.Client().bucket(get_settings().bucket_name)


def download_from_gcs(gcs_path: str, local_path: str) -> None:
    _bucket().blob(gcs_path).download_to_filename(local_path)


def upload_folder_to_gcs(local_folder: str, gcs_prefix: str) -> None:
    bucket = _bucket()
    for root, _, files in os.walk(local_folder):
        for filename in files:
            local_file = os.path.join(root, filename)
            relative_path = os.path.relpath(local_file, local_folder)
            blob = bucket.blob(f"{gcs_prefix}/{relative_path}")
            blob.upload_from_filename(local_file)
