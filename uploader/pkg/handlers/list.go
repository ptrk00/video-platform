package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)


func GetUserFiles(db *sql.DB, l *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin, _ := r.Context().Value("admin").(bool)
		var rows *sql.Rows
		var err error
		if isAdmin {
			rows, err = db.Query("SELECT filename, filesize, content_type, etag, file_url, checksum, upload_timestamp FROM files")
			if err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
		} else {
			rows, err = db.Query("SELECT filename, filesize, content_type, etag, file_url, checksum, upload_timestamp FROM files WHERE user_id=$1", userID)
			if err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
		}
		defer rows.Close()

		var files []map[string]interface{}
		for rows.Next() {
			var file map[string]interface{}
			var filename, contentType, etag, fileURL, checksum string
			var uploaded_timestamp time.Time

			var filesize int64
			if err := rows.Scan(&filename, &filesize, &contentType, &etag, &fileURL, &checksum, &uploaded_timestamp); err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}

			deleted := time.Now().After(uploaded_timestamp.Add(2*time.Minute))
			l.Infof("Time now is %s", time.Now())
			l.Infof("File %s is marked as %t due to %s uploaded_tiemstamp", filename, deleted, uploaded_timestamp.String())
			file = map[string]interface{}{
				"filename":     filename,
				"filesize":     filesize,
				"content_type": contentType,
				"etag":         etag,
				"file_url":     fileURL,
				"checksum":     checksum,
				"deleted": 		deleted,
			}
			files = append(files, file)
		}
		if err := rows.Err(); err != nil {
			l.Error(err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(files)
	}
}