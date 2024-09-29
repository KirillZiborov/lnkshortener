package auth

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string
}

const TokenExp = time.Hour * 3
const SecretKey = "supersecretkey"

func GenerateToken(userID string) (string, error) {
	if userID == "" {
		userID = uuid.New().String()
	}

	tokenString, err := BuildJWTString(userID)
	if err != nil {
		log.Fatal(err)
	}

	return tokenString, nil
}

func BuildJWTString(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
		},

		UserID: userID,
	})

	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GetUserID(tokenString string) string {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(SecretKey), nil
	})
	if err != nil {
		return ""
	}

	if !token.Valid {
		fmt.Println("Token is not valid")
		return ""
	}

	fmt.Println("Token is valid")
	return claims.UserID
}

func AuthPost(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie("cookie")
	var userID string

	if err != nil {
		token, err := GenerateToken("")
		if err != nil {
			http.Error(w, "Error while generating token", http.StatusInternalServerError)
			return "", err
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "cookie",
			Value:    token,
			Expires:  time.Now().Add(TokenExp),
			HttpOnly: true,
		})
		userID = GetUserID(token)
	} else {
		userID = GetUserID(cookie.Value)
		if userID == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return "", err
		}
	}

	return userID, nil
}

func AuthGet(r *http.Request) (string, error) {
	cookie, err := r.Cookie("cookie")
	if err != nil {
		return "", err
	}

	userID := GetUserID(cookie.Value)
	if userID == "" {
		return "", err
	}
	return userID, err
}
