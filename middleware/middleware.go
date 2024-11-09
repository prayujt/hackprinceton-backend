package middleware

import (
	"hackprinceton/auth"

	"github.com/golang-jwt/jwt"

	"context"
	"log"
	"net/http"
)

func GetClaims(request *http.Request) auth.Claims {
	ctxClaims := request.Context().Value("claims").(jwt.MapClaims)
	var claims auth.Claims

	if userId, ok := ctxClaims["userId"].(string); ok {
		claims.UserId = userId
	}

	if email, ok := ctxClaims["email"].(string); ok {
		claims.Email = email
	}

	if name, ok := ctxClaims["name"].(string); ok {
		claims.Name = name
	}

	if exp, ok := ctxClaims["exp"].(int64); ok {
		claims.StandardClaims = jwt.StandardClaims{
			ExpiresAt: exp,
		}
	}
	return claims
}

func AuthHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := request.Header.Get("Token")
		if token == "" {
			log.Println("No token is given")
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		claims, err := auth.ParseJWT(token)

		if err != nil {
			log.Printf("Error parsing JWT: %s", err)
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(request.Context(), "claims", claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func AuthHandler(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := request.Header.Get("Token")
		if token == "" {
			log.Println("No token is given")
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		claims, err := auth.ParseJWT(token)

		if err != nil {
			log.Printf("Error parsing JWT: %s", err)
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(request.Context(), "claims", claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}
