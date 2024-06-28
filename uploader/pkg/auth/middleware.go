package auth

import (
	"context"
	"github.com/dgrijalva/jwt-go"
	"go.uber.org/zap"
	"net/http"
)

func Authenticate(next http.Handler, l *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			l.Info("no auth header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenStr := authHeader[len("Bearer "):]

		// Parse and validate the token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check the token against the OPA policy
		_, err = checkOPAPolicy(tokenStr, l)
		if err != nil {
			l.Errorf("error checking opa policy: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "username", claims.Username)
		if claims.Username == "admin" {
			ctx = context.WithValue(ctx, "admin", true)
		} else {
			ctx = context.WithValue(ctx, "admin", false)
		}
		ctx = context.WithValue(ctx, "id", claims.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
