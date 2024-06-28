package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"video-platform/uploader/pkg/auth"
)

func Login(db *sql.DB, l *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds auth.Credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Query the user from the database
		var storedPassword string
		var userID int
		err = db.QueryRow("SELECT id, password FROM app_users WHERE username=$1", creds.Username).Scan(&userID, &storedPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				l.Error(err)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			} else {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
			return
		}

		// Compare the stored hashed password with the provided password
		err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(creds.Password))
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := auth.GenerateJWT(creds.Username, userID)
		if err != nil {
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}
