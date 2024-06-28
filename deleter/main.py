import time
from minio import Minio
from minio.error import S3Error

# Configuration
MINIO_URL = 'minio:9000'
ACCESS_KEY = 'minioadmin'
SECRET_KEY = 'minioadmin'
BUCKET_NAME = 'videos'

# Initialize MinIO client
minio_client = Minio(MINIO_URL, access_key=ACCESS_KEY, secret_key=SECRET_KEY, secure=False)

def delete_all_objects():
    try:
        # List all objects in the specified bucket
        objects = minio_client.list_objects(BUCKET_NAME, recursive=True)
        for obj in objects:
            print(f"Deleting {obj.object_name}")
            minio_client.remove_object(BUCKET_NAME, obj.object_name)
    except S3Error as e:
        print(f"Error occurred: {e}")

print("Starting deletion loop")
while True:
    print('Sleepping zzz zzz ...')
    time.sleep(2*60)  # Wait for 3 seconds before repeating
    delete_all_objects()
