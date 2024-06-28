package handlers

import (
	"net/http"
	"database/sql"
	"go.uber.org/zap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"github.com/minio/minio-go/v7"
	"io"
	"video-platform/uploader/pkg/queue"
	"video-platform/uploader/pkg/storage"
	"video-platform/uploader/pkg/process" 
	"video-platform/uploader/pkg/config"
	"video-platform/uploader/pkg/monitoring"
	"fmt"
)


func UploadFileHandler(config *config.ServerConfig, db *sql.DB, minioClient *minio.Client,
	l *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("uploader").Start(r.Context(), "HandleUpload")
		defer span.End()

		username := r.Context().Value("username").(string)
		userID := r.Context().Value("id").(int)

		span.SetAttributes(
			attribute.String("username", username),
			attribute.Int("user_id", userID),
		)

		l.Debugw("Handling video upload", "username", username)
		if r.Method != "POST" {
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
			return
		}

		// Parse the multipart form
		r.ParseMultipartForm(10 << 20) // Limit upload size

		file, handler, err := r.FormFile(config.VideoFormFilename)
		if err != nil {
			l.Errorw("Could not parse the multipart file",
				zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error parsing file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Get the file size and content type
		fileSize := handler.Size
		contentType := handler.Header.Get("Content-Type")

		// Create a new reader to compute the checksum and upload the file
		file.Seek(0, io.SeekStart)
		_, sha256Checksum, err := process.ComputeChecksum(ctx, file)
		if err != nil {
			l.Errorw("Could not compute checksum", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error computing checksum", http.StatusInternalServerError)
			return
		}

		// Upload the file to MinIO
		file.Seek(0, io.SeekStart)
		l.Infow("Uploading file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename))
		info, err := minioClient.PutObject(ctx, config.MinioBucket, handler.Filename, file, fileSize, minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			l.Errorw("Could not upload file", zap.String("bucketname", config.MinioBucket),
				zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}

		// Increment the Prometheus counter
		monitoring.FileUploadCount.Inc()

		// Get the file URL and ETag
		fileURL := fmt.Sprintf("http://%s/%s/%s", config.MinioHost, config.MinioBucket, handler.Filename)
		etag := info.ETag

		// Store metadata in PostgreSQL
		err = storage.StoreFileMetadata(ctx, db, handler.Filename, fileSize, contentType, etag, fileURL, sha256Checksum, userID)
		if err != nil {
			l.Errorw("Could not store file metadata", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error storing file metadata", http.StatusInternalServerError)
			return
		}

		l.Infow("Successfully uploaded file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename), zap.String("username", username))
		fmt.Fprintf(w, "Successfully uploaded %s\n", handler.Filename)
		queue.PublishMessage(ctx, config.MinioBucket, handler.Filename)
	}
}