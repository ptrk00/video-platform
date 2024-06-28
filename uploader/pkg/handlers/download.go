package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"io"
	"net/http"
)

func DownloadFile(db *sql.DB, minioClient *minio.Client, bucketName string, l *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		etag := r.URL.Query().Get("etag")
		if etag == "" {
			http.Error(w, "Missing etag", http.StatusBadRequest)
			return
		}

		// Verify that the file belongs to the user
		var filename, contentType string
		var err error
		isAdmin, _ := r.Context().Value("admin").(bool)

		if isAdmin {
			err = db.QueryRow("SELECT filename, content_type FROM files WHERE etag=$1", etag).Scan(&filename, &contentType)
		} else {
			err = db.QueryRow("SELECT filename, content_type FROM files WHERE etag=$1 AND user_id=$2", etag, userID).Scan(&filename, &contentType)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "File not found", http.StatusNotFound)
			} else {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
			return
		}

		// Get the file from MinIO
		archived := r.URL.Query().Get("archived")
		if archived == "true" {
			l.Info("Changing bucket name to backup")
			bucketName = "backup"
		} else {
			bucketName = "videos"
		}
		object, err := minioClient.GetObject(context.Background(), bucketName, filename, minio.GetObjectOptions{})
		if err != nil {
			l.Error(err)
			http.Error(w, "Error retrieving file", http.StatusInternalServerError)
			return
		}
		defer object.Close()

		// Set the content type and other headers, then write the file to the response
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		if _, err := io.Copy(w, object); err != nil {
			l.Error(err)
			http.Error(w, "Error writing file to response", http.StatusInternalServerError)
			return
		}
	}
}
