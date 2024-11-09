package routes

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func HandleUserRoutes(r *mux.Router) {
	s := r.PathPrefix("/users").Subrouter()

	s.HandleFunc("/login", loginUser).Methods("POST")
	s.HandleFunc("/register", registerUser).Methods("POST")
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	log.Println("[POST] /users/login")
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	log.Println("[POST] /users/register")
}
