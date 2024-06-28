package queue

import (
	"context"
	"encoding/json"

	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"video-platform/handler/pkg/config"
	"video-platform/handler/pkg/process"
	"video-platform/uploader/pkg/queue"
)

func HandleMessage(msg *nats.Msg, minioClient *minio.Client, config *config.ServerConfig, l *zap.SugaredLogger) {
	var message queue.Message
	err := json.Unmarshal(msg.Data, &message)
	if err != nil {
		l.Error("Failed to unmarshal message", zap.Error(err))
		return
	}

	l.Infof("Processing file with ETag: %s from bucket: %s", message.Filename, message.Bucket)

	// Download the file
	object, err := minioClient.GetObject(context.Background(), message.Bucket, message.Filename, minio.GetObjectOptions{})
	if err != nil {
		l.Error("Failed to get object from MinIO", zap.Error(err))
		return
	}
	defer object.Close()

	// Encrypt the file
	encryptedData, err := process.EncryptData(object, config.EncryptionKey, l)
	if err != nil {
		l.Error("Failed to encrypt data", zap.Error(err))
		return
	}

	// Compress the file
	compressedData, err := process.CompressData(encryptedData)
	if err != nil {
		l.Error("Failed to compress data", zap.Error(err))
		return
	}

	// Store the file in the destination bucket
	_, err = minioClient.PutObject(context.Background(), config.MinioDestBucket, message.Filename, compressedData, -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		l.Error("Failed to put object to MinIO", zap.Error(err))
		return
	}

	l.Infof("Successfully processed and stored file with ETag: %s to bucket: %s", message.Filename, config.MinioDestBucket)
}
