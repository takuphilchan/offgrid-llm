package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// writeError writes an error response
func writeError(w http.ResponseWriter, message string, statusCode int) {
	writeErrorWithCode(w, message, statusCode, "")
}

func writeErrorWithCode(w http.ResponseWriter, message string, statusCode int, code string) {
	log.Printf("API Error: %s (Status: %d, Code: %s)", message, statusCode, code)
	w.WriteHeader(statusCode)
	response := api.ErrorResponse{
		Error: api.ErrorDetail{
			Message: message,
			Type:    "api_error",
			Code:    code,
		},
	}
	json.NewEncoder(w).Encode(response)
}

func handleEngineError(w http.ResponseWriter, err error) bool {
	engineErr := inference.AsEngineError(err)
	if engineErr == nil {
		return false
	}

	status := http.StatusBadRequest
	log.Printf("Engine error (%s): %s", engineErr.Code, engineErr.Details)
	writeErrorWithCode(w, engineErr.Message, status, engineErr.Code)
	return true
}
