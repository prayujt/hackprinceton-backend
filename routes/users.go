package routes

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	. "hackprinceton/database"
	"hackprinceton/utils"
)

type Account struct {
	UserId   string `json:"userId" database:"id"`
	Email    string `json:"email" database:"email"`
	Name     string `json:"name" database:"name"`
	Username string `json:"username" database:"username"`
	Password string `json:"password" database:"password"`
}

func HandleUserRoutes(r *mux.Router) {
	s := r.PathPrefix("/users").Subrouter()

	s.HandleFunc("/login", loginUser).Methods("POST")
	s.HandleFunc("/register", registerUser).Methods("POST")
}

func registerUser(res http.ResponseWriter, req *http.Request) {
	log.Println("POST /users/register")
	acc, err := utils.DecodeBody[Account](req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = Execute(
		`
		INSERT INTO Accounts
		(id, email, name, username, password, created_at, updated_at)
		VALUES (
			gen_random_uuid(),
			?,
			?,
			?,
			encode(sha256(?), 'hex'),
			now(),
			now()
		)
		`,
		acc.Email, acc.Name, acc.Username, acc.Password,
	)

	if err != nil {
		log.Printf("Error inserting into db: %s", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}

func loginUser(res http.ResponseWriter, req *http.Request) {
	log.Println("POST /users/login")
}
