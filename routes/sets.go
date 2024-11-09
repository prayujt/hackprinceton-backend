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

type Card struct {
	CardId string `json:"cardId" database:"id"`
	SetId  string `json:"setId" database:"set_id"`
	Front  string `json:"front" database:"front"`
	Back   string `json:"back" database:"back"`
}

type CreateSetRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Options     CreateSetOptions `json:"options"`
}

type CreateSetOptions struct {
	CardCount    int `json:"cardCount"`
	Suggesstions int `json:"suggestions"`
}

var openaiKey string

func HandleSetRoutes(r *mux.Router, _openaiKey string) {
	s := r.PathPrefix("/sets").Subrouter()

	s.Use(AuthHandle)

	s.HandleFunc("", getSets).Methods("GET")

	s.HandleFunc("", createSet).Methods("POST")
	s.HandleFunc("/{setId}", getSet).Methods("GET")
	s.HandleFunc("/{setId}", updateSet).Methods("PUT")
	s.HandleFunc("/{setId}", deleteSet).Methods("DELETE")

	openaiKey = _openaiKey
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

	// 512 MB limit on file size
	err := req.ParseMultipartForm(512 << 20)
	if err != nil {
		http.Error(res, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, handler, err := req.FormFile("file")
	if err != nil {
		http.Error(res, "Error retrieving the file", http.StatusBadRequest)
		return
	}

	defer file.Close()

	log.Printf("Uploaded File: %+v\n", handler.Filename)
	log.Printf("File Size: %+v\n", handler.Size)
	log.Printf("MIME Header: %+v\n", handler.Header)

	var metadata CreateSetRequest
	metadataField := req.FormValue("metadata")

	if metadataField != "" {
		if err := json.Unmarshal([]byte(metadataField), &metadata); err != nil {
			http.Error(res, "Failed to decode JSON metadata", http.StatusBadRequest)
			return
		}
	}

	claims := GetClaims(req)

	_, err = Execute(
		`
		INSERT INTO sets
		(id, name, description, author_id, created_at, updated_at)
		VALUES (
			gen_random_uuid(),
			$1,
			$2,
			$3,
			now(),
			now()
		)
		`,
		metadata.Name, metadata.Description, claims.UserId,
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
		WHERE id = $1
		`,
		setId,
	)

	if len(set) == 0 {
		res.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(res).Encode(set[0])
	}
}

func getSetCards(res http.ResponseWriter, req *http.Request) {
	log.Println("GET /sets/{setId}/cards")
	params := mux.Vars(req)
	setId := params["setId"]

	cards := []Card{}
	Query(
		&cards,
		`
		SELECT id, front, back, set_id
		FROM cards
		WHERE set_id = $1
		`,
		setId,
	)

	if len(cards) == 0 {
		cards = make([]Card, 0)
	}
	json.NewEncoder(res).Encode(cards)
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
		SET name = $1, description = $2
		WHERE id = $3
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
		WHERE id = $1
		`,
		setId,
	)

	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}
