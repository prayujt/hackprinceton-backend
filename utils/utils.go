package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

func DecodeBody[T any](request *http.Request) (T, error) {
	var obj T
	decoder := json.NewDecoder(request.Body)

	if err := decoder.Decode(&obj); err != nil {
		log.Printf("Error decoding request body: %s", err)
		return obj, err
	}

	return obj, nil
}
