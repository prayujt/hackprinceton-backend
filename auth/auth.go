package auth

import (
	"errors"
	"reflect"
	"time"

	"github.com/golang-jwt/jwt"
)

var SecretKey []byte

type Claims struct {
	UserId string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.StandardClaims
}

func GenerateJWT(userId string, email string, name string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(2190 * time.Hour)
	claims["userId"] = userId
	claims["email"] = email
	claims["name"] = name

	tokenString, err := token.SignedString(SecretKey)

	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseJWT(_token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(_token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(_token), nil
	})

	if (err != nil && err.Error() != "signature is invalid") || len(claims) == 0 {
		return claims, errors.New("internal error")
	}

	if claims["userId"] != nil && reflect.TypeOf(claims["userId"]).Name() != "string" {
		return claims, errors.New("invalid JWT authorization")
	}

	if claims["userId"].(string) == "" {
		return claims, errors.New("not authorized")
	}

	return claims, nil
}
