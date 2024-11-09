package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"

	"hackprinceton/database"
	"hackprinceton/routes"
)

func main() {
	databaseUrl := os.Getenv("DATABASE_URL")
	if databaseUrl == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	database.InitDatabase(databaseUrl)
	log.Println("Connected to database")

	r := mux.NewRouter()
	routes.HandleUserRoutes(r)

	fmt.Println("Server running on 0.0.0.0:8080")

	if environment == "development" {
		corsMiddleware := handlers.CORS(
			handlers.AllowedOrigins([]string{"http://localhost:5173", "http://localhost:4173"}),
			handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
			handlers.AllowCredentials(),
		)
		log.Fatal(http.ListenAndServe(":8080", corsMiddleware(r)))
	} else {
		log.Fatal(http.ListenAndServe(":8080", r))
	}
}
