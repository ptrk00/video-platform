package auth

import (
	"time"

	"github.com/dgrijalva/jwt-go"
)

const (
	jwtSecret            = "supersecretkey"
)

type Claims struct {
	Username string `json:"username"`
	ID       int    `json:"id"`
	jwt.StandardClaims
}

func GenerateJWT(username string, id int) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		Username: username,
		ID:       id,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}