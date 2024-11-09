package routes

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	. "hackprinceton/database"
	. "hackprinceton/middleware"
	"hackprinceton/utils"
)

type Set struct {
	SetId       string `json:"setId" database:"id"`
	Name        string `json:"name" database:"name"`
	Description string `json:"description" database:"description"`
	AuthorId    string `json:"authorId" database:"author_id"`
}

func HandleSetRoutes(r *mux.Router) {
	s := r.PathPrefix("/sets").Subrouter()

	s.Use(AuthHandle)

	s.HandleFunc("", getSets).Methods("GET")

	s.HandleFunc("", createSet).Methods("POST")
	s.HandleFunc("/{setId}", getSet).Methods("GET")
	s.HandleFunc("/{setId}", updateSet).Methods("PUT")
	s.HandleFunc("/{setId}", deleteSet).Methods("DELETE")
}

func getSets(res http.ResponseWriter, req *http.Request) {
	log.Println("GET /sets")

	claims := GetClaims(req)

	sets := []Set{}
	Query(
		&sets,
		`
		SELECT id, name, description, author_id
		FROM sets
		WHERE author_id = ?
		`,
		claims.UserId,
	)

	if len(sets) == 0 {
		sets = make([]Set, 0)
	}
	json.NewEncoder(res).Encode(sets)
}

func createSet(res http.ResponseWriter, req *http.Request) {
	log.Println("POST /sets")
	acc, err := utils.DecodeBody[Set](req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	claims := GetClaims(req)
	if err != nil {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	_, err = Execute(
		`
		INSERT INTO sets
		(id, name, description, author_id, created_at, updated_at)
		VALUES (
			gen_random_uuid(),
			?,
			?,
			?,
			now(),
			now()
		)
		`,
		acc.Name, acc.Description, claims.UserId,
	)

	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusCreated)
}

func getSet(res http.ResponseWriter, req *http.Request) {
	log.Println("GET /sets/{setId}")
	params := mux.Vars(req)
	setId := params["setId"]

	set := []Set{}
	Query(
		&set,
		`
		SELECT id, name, description, author_id
		FROM sets
		WHERE id = ?
		`,
		setId,
	)

	if len(set) == 0 {
		res.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(res).Encode(set[0])
	}
}

func updateSet(res http.ResponseWriter, req *http.Request) {
	log.Println("PUT /sets/{setId}")
	params := mux.Vars(req)
	setId := params["setId"]

	acc, err := utils.DecodeBody[Set](req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = Execute(
		`
		UPDATE sets
		SET name = ?, description = ?
		WHERE id = ?
		`,
		acc.Name, acc.Description, setId,
	)

	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}

func deleteSet(res http.ResponseWriter, req *http.Request) {
	log.Println("DELETE /sets/{setId}")
	params := mux.Vars(req)
	setId := params["setId"]

	_, err := Execute(
		`
		DELETE FROM sets
		WHERE id = ?
		`,
		setId,
	)

	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}
