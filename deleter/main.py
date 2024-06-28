import time
from minio import Minio
from minio.error import S3Error
import datetime

# Configuration
MINIO_URL = 'minio:9000'
ACCESS_KEY = 'minioadmin'
SECRET_KEY = 'minioadmin'
BUCKET_NAME = 'videos'
RETENTION_PERIOD = datetime.timedelta(minutes=2)

# Initialize MinIO client
minio_client = Minio(MINIO_URL, access_key=ACCESS_KEY, secret_key=SECRET_KEY, secure=False)

def delete_all_objects():
    try:
        # List all objects in the specified bucket
        print("Starting listing files")
        objects = minio_client.list_objects(BUCKET_NAME, recursive=True)
        for obj in objects:
            object_age = datetime.datetime.utcnow() - obj.last_modified.replace(tzinfo=None)
            if object_age > RETENTION_PERIOD:
                print(f"Deleting {obj.object_name}")
                minio_client.remove_object(BUCKET_NAME, obj.object_name)
    except S3Error as e:
        print(f"Error occurred: {e}")

print("Starting deletion loop")
while True:
    print('Sleepping zzz zzz ...')
    time.sleep(20)
    delete_all_objects()
