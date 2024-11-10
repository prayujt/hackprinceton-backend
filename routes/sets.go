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
	CardCount   int    `json:"cardCount" database:"card_count"`
}

type Card struct {
	CardId string `json:"id" database:"id"`
	SetId  string `json:"setId" database:"set_id"`
	Front  string `json:"front" database:"front"`
	Back   string `json:"back" database:"back"`
}

type CreateCard struct {
	Question Question `json:"question"`
	Answer   string   `json:"answer"`
}

type Question struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

type CreateSetRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Options     CreateSetOptions `json:"options"`
}

type CreateSetOptions struct {
	TrueFalseCount      int    `json:"tfCount"`
	MultipleChoiceCount int    `json:"mcCount"`
	NormalCount         int    `json:"normalCount"`
	Suggestions         string `json:"suggestions"`
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
		SELECT
				s.id AS id,
				s.name AS name,
				s.description AS description,
				s.author_id AS author_id,
				COALESCE(card_count, 0) AS card_count
		FROM
				sets s
		LEFT JOIN (
				SELECT
						set_id,
						COUNT(*) AS card_count
				FROM
						cards
				GROUP BY
						set_id
		) c ON s.id = c.set_id
		WHERE
				s.author_id = $1
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
	assistantInstructions := `
		You are given a file.
		You need to perform the action specified in the next instruction using the information from the file.
		It will be given as a PDF file.
		Your response should be in JSON format in the manner specified.
		The output should be directly parseable by a JSON decoder.
		Do not include any additional text in your response.`

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
				`
				Please generate flash cards from the file given to help study and understand it.
				Structure the response as an array of JSON objects, with a key for question and answer for each flash card.
				The question will always be a JSON object as well, with a key for type, which will be either "tf", "mc", or "normal", and a key for content, which will be the question itself formatted as specified.
				Each flash card will be one of three types: True/False, Multiple Choice, or Normal.
				You should generate %d True/False, %d Multiple Choice, and %d Normal flash cards.
				If these above counts are all -1, you can generate any number of each type, depending on how many you think are appropriate given the file content.
        If it is True/False, the answer should be either "True" or "False", and the question content should be a statement.
				If it is Multiple Choice, there should be 4 options, with the correct answer being one of them.
				The question content should be an array of the options, including the question.
				The question should always be the first item in the array, and the options should be following it, with a total of 5 items in the array.
				The answer should be the correct option, prefaced with the letter corresponding to the option (a, b, c, or d). e.g. "a) This is the correct answer".
				The letter should always be in lowercase.
				If it is Normal, the question content should be plaintext, and the answer should be the answer to that question.
				Never generate more flashcards for each category than specified.
				Please do not add any additional text to the JSON response.
				It should always only contain the JSON body, with no text before or after.
				As a suggestion, focus on the following topic: %s
				`,
				metadata.Options.TrueFalseCount,
				metadata.Options.MultipleChoiceCount,
				metadata.Options.NormalCount,
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

	prefixIndex := strings.Index(cardResponses, "```json")

	if prefixIndex != -1 {
		cardResponses = cardResponses[prefixIndex+len("```json"):]
	}
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
		log.Printf("Card: %v, %s", card.Question, card.Answer)
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
	json.NewEncoder(res).Encode(map[string]string{"setId": setId})
}

func getSet(res http.ResponseWriter, req *http.Request) {
	log.Println("GET /sets/{setId}/cards")
	params := mux.Vars(req)
	setId := params["setId"]

	type SetResponse struct {
		SetName string `json:"setName"`
		Cards   []Card `json:"cards"`
	}

	var response SetResponse

	var setName string
	QueryValue(&setName, `
		SELECT name
		FROM sets
		WHERE id = $1`,
		setId)

	cards := []Card{}
	Query(
		&cards,
		`
		SELECT id, front, back, set_id
		FROM cards
		WHERE set_id = $1;
		`,
		setId,
	)

	response.SetName = setName
	response.Cards = cards
	if len(cards) == 0 {
		response.Cards = make([]Card, 0)
	}
	json.NewEncoder(res).Encode(response)
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
