# Use an official Python runtime as a parent image
FROM python:3.9-slim

# Set the working directory in the container
WORKDIR /app

# Copy the current directory contents into the container at /usr/src/app
COPY . .

# Install the MinIO Python SDK
RUN pip install --no-cache-dir minio

# Run the script
CMD ["python", "-u", "/app/main.py"]
