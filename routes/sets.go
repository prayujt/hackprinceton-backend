package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	// "github.com/openai/openai-go"
	openai "github.com/sashabaranov/go-openai"

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

type CreateCard struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type CreateSetRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Options     CreateSetOptions `json:"options"`
}

type CreateSetOptions struct {
	CardCount   int    `json:"cardCount"`
	Suggestions string `json:"suggestions"`
}

var openaiClient *openai.Client

func HandleSetRoutes(r *mux.Router, _openaiClient *openai.Client) {
	s := r.PathPrefix("/sets").Subrouter()

	s.Use(AuthHandle)

	s.HandleFunc("", getSets).Methods("GET")

	s.HandleFunc("", createSet).Methods("POST")
	s.HandleFunc("/{setId}", getSet).Methods("GET")
	s.HandleFunc("/{setId}", updateSet).Methods("PUT")
	s.HandleFunc("/{setId}", deleteSet).Methods("DELETE")

	openaiClient = _openaiClient
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

	claims := GetClaims(req)

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

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(res, "Failed to read file", http.StatusInternalServerError)
		log.Fatalf("Error reading file: %v", err)
		return
	}

	openaiFile, err := openaiClient.CreateFileBytes(context.Background(), openai.FileBytesRequest{
		Name:    "offer_letter.pdf",
		Bytes:   []byte(fileBytes),
		Purpose: openai.PurposeAssistants,
	})
	if err != nil {
		log.Fatalf("Error uploading file: %v", err)
		return
	}

	assistantName := "Flash Card Generator"
	assistantInstructions := "You are given a file. You need to perform the action specified in the next instruction using the information from the file. It will be given as a PDF file. Your response should be in JSON format in the manner specified. The output should be directly parseable by a JSON decoder. Do not include any additional text in your response."

	assistant, err := openaiClient.CreateAssistant(context.Background(), openai.AssistantRequest{
		Name:         &assistantName,
		Model:        openai.GPT4TurboPreview,
		Instructions: &assistantInstructions,
		Tools:        []openai.AssistantTool{{Type: openai.AssistantToolTypeRetrieval}},
		FileIDs:      []string{openaiFile.ID},
	})
	if err != nil {
		log.Fatalf("Error creating assistant: %v", err)
		return
	}

	thread, err := openaiClient.CreateThread(context.Background(), openai.ThreadRequest{})
	if err != nil {
		log.Fatalf("Error creating thread: %v", err)
		return
	}

	_, err = openaiClient.CreateMessage(context.Background(), thread.ID,
		openai.MessageRequest{
			Role: string(openai.ThreadMessageRoleUser),
			Content: fmt.Sprintf(
				"Please generate exactly %d flash cards from the file given. Structure the response in JSON format, with a key for question and answer for each flash card. Please return this as an array of these objects without any additional text. As a suggestion, focus on the following topic: %s",
				metadata.Options.CardCount,
				metadata.Options.Suggestions,
			),
		})
	if err != nil {
		log.Fatalf("Error creating message: %v", err)
		return
	}

	run, err := openaiClient.CreateRun(context.Background(), thread.ID, openai.RunRequest{
		AssistantID: assistant.ID,
	})
	if err != nil {
		log.Fatalf("Error creating run: %v", err)
		return
	}

	for run.Status != openai.RunStatusCompleted {
		time.Sleep(1 * time.Second)
		// retrieve the status of the run
		run, err = openaiClient.RetrieveRun(context.Background(), thread.ID, run.ID)
		if err != nil {
			log.Fatalf("Error retrieving run: %v", err)
		}
		log.Printf("Run Status: %s", run.Status)
	}

	msgs, err := openaiClient.ListMessage(context.Background(), thread.ID, nil, nil, nil, nil, &run.ID)
	cardResponses := msgs.Messages[0].Content[0].Text.Value

	cardResponses = strings.TrimPrefix(cardResponses, "```json")
	cardResponses = strings.TrimSuffix(cardResponses, "```")

	err = openaiClient.DeleteFile(context.Background(), openaiFile.ID)
	if err != nil {
		log.Printf("Error deleting file: %v", err)
	}

	log.Printf("Assistant Response: %s", cardResponses)

	setId := uuid.New().String()

	_, err = Execute(
		`
		INSERT INTO sets
		(id, name, description, author_id, created_at, updated_at)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			now(),
			now()
		)
		`,
		setId, metadata.Name, metadata.Description, claims.UserId,
	)
	if err != nil {
		log.Printf("Error inserting set: %v", err)
		http.Error(res, "Failed to insert set", http.StatusInternalServerError)
		return
	}

	var cards []CreateCard
	err = json.Unmarshal([]byte(cardResponses), &cards)
	if err != nil {
		log.Printf("Error decoding flashcard response: %v", err)
		http.Error(res, "Failed to decode flashcard response", http.StatusInternalServerError)
		return
	}
	for _, card := range cards {
		// log.Printf("Card: %s, %s", card.Question, card.Answer)
		_, err = Execute(
			`
			INSERT INTO cards
			(id, front, back, set_id, created_at, updated_at)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				now(),
				now()
			)
			`,
			uuid.New().String(), card.Question, card.Answer, setId,
		)
		if err != nil {
			log.Printf("Error inserting card: %v", err)
			http.Error(res, "Failed to insert card", http.StatusInternalServerError)
			return
		}
	}

	res.WriteHeader(http.StatusOK)
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
