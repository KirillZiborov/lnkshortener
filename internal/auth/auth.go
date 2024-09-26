package auth

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string
}

var UserUUID string

const TOKEN_EXP = time.Hour * 3
const SECRET_KEY = "supersecretkey"

func GenerateToken() (string, error) {
	tokenString, err := BuildJWTString()
	if err != nil {
		log.Fatal(err)
	}

	return tokenString, nil
}

func BuildJWTString() (string, error) {

	UserUUID = uuid.New().String()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TOKEN_EXP)),
		},

		UserID: UserUUID,
	})

	tokenString, err := token.SignedString([]byte(SECRET_KEY))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GetUserID(tokenString string) string {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(SECRET_KEY), nil
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
