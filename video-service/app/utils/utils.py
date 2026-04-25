
BUCKET_NAME = "my-videos-bucket"
storage_client = storage.Client()

def download_from_gcs(gcs_path: str, local_path: str):
    bucket = storage_client.bucket(BUCKET_NAME)
    blob = bucket.blob(gcs_path)
    blob.download_to_filename(local_path)
    print(f"Downloaded {gcs_path} → {local_path}")


def upload_folder_to_gcs(local_folder: str, gcs_prefix: str):
    bucket = storage_client.bucket(BUCKET_NAME)
    for root, _, files in os.walk(local_folder):
        for file in files:
            local_file = os.path.join(root, file)
            rel_path = os.path.relpath(local_file, local_folder)
            blob = bucket.blob(f"{gcs_prefix}/{rel_path}")
            blob.upload_from_filename(local_file)
            print(f"Uploaded {local_file} → gs://{BUCKET_NAME}/{gcs_prefix}/{rel_path}")
