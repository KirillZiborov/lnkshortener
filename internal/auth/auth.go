// Package auth provides authentication utilities using JSON Web Tokens (JWT).
package auth

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// Claims defines the structure of JWT claims used in the authentication process.
type Claims struct {
	jwt.RegisteredClaims        // Standart JWT fields.
	UserID               string //UserID - unique ID of the user.
}

// TokenExp specifies the duration for which a JWT token is valid.
// Tokens expire 3 hours after issuance.
const TokenExp = time.Hour * 3

// SecretKey is the secret key used to sign JWT tokens.
const SecretKey = "supersecretkey"

// GenerateToken creates a new JWT token for a given userID.
// If the provided userID is empty, it generates a new UUID for the user.
// The function returns the signed JWT token string or an error if the process fails.
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

// BuildJWTString creates a signed JWT token string for a given userID.
// It sets the token's expiration time based on the TokenExp constant.
// The function uses the HS256 signing method and returns the signed token string or an error.
func BuildJWTString(userID string) (string, error) {
	// Create a new token with given claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
		},

		UserID: userID,
	})

	// Sign token using SecretKey
	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserID extracts the UserID from a given JWT token string.
// It parses the token, validates its signature and expiration, and retrieves the UserID claim.
// If the token is invalid or expired, the function returns an empty string.
func GetUserID(tokenString string) string {
	claims := &Claims{}
	// Parse token and extract claims
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

	// fmt.Println("Token is valid")
	return claims.UserID
}

// AuthPost handles the authentication for HTTP POST requests.
// It checks for an existing authentication cookie. If absent, it generates a new JWT token,
// sets it as a cookie in the response, and retrieves the associated UserID.
// If a cookie is present, it validates the token and extracts the UserID.
// The function returns the UserID or an error if authentication fails.
func AuthPost(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie("cookie")
	var userID string

	if err != nil {
		// Generate a new token if no cookie is found
		token, err := GenerateToken("")
		if err != nil {
			http.Error(w, "Error while generating token", http.StatusInternalServerError)
			return "", err
		}

		// Set the token as an HTTP-only cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "cookie",
			Value:    token,
			Expires:  time.Now().Add(TokenExp),
			HttpOnly: true,
		})
		userID = GetUserID(token)
	} else {
		// Extract and validate the UserID from the existing cookie
		userID = GetUserID(cookie.Value)
		if userID == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return "", err
		}
	}

	return userID, nil
}

// AuthGet handles the authentication for HTTP GET requests.
// It retrieves the authentication cookie from the request, validates the JWT token,
// and extracts the associated UserID.
// The function returns the UserID or an error if authentication fails.
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
