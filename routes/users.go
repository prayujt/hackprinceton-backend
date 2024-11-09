package routes

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"hackprinceton/auth"
	. "hackprinceton/database"
	"hackprinceton/utils"
)

type User struct {
	UserId   string `json:"userId" database:"id"`
	Email    string `json:"email" database:"email"`
	Name     string `json:"name" database:"name"`
	Username string `json:"username" database:"username"`
	Password string `json:"password" database:"password"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func HandleUserRoutes(r *mux.Router) {
	s := r.PathPrefix("/users").Subrouter()

	s.HandleFunc("/login", loginUser).Methods("POST")
	s.HandleFunc("/register", registerUser).Methods("POST")
}

func registerUser(res http.ResponseWriter, req *http.Request) {
	log.Println("POST /users/register")
	acc, err := utils.DecodeBody[User](req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = Execute(
		`
		INSERT INTO users
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
	loginReq, err := utils.DecodeBody[LoginRequest](req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	var user []User

	Query(
		&user,
		`
		SELECT id, email, name, username
		FROM users
		WHERE
		(username = ? OR email = ?)
		AND password = encode(sha256(?), 'hex')
		`,
		loginReq.Identifier, loginReq.Identifier, loginReq.Password,
	)

	if len(user) == 0 {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user[0].UserId, user[0].Email, user[0].Name)
	if err != nil {
		log.Printf("Error generating JWT: %s", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(res).Encode(map[string]string{"token": token})
}
