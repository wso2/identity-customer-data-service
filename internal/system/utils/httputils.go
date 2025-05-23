package utils

import (
	"encoding/json"
	"errors"                                                                             // Standard Go errors package
	customerrors "github.com/wso2/identity-customer-data-service/internal/system/errors" // Alias for the custom errors
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
)

// HandleError sends an HTTP error response based on the provided error
func HandleError(w http.ResponseWriter, err error) {
	var clientError *customerrors.ClientError
	w.Header().Set("Content-Type", "application/json")
	if ok := errors.As(err, &clientError); ok {
		w.WriteHeader(clientError.StatusCode)
		_ = json.NewEncoder(w).Encode(struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			Description string `json:"description"`
		}{
			Code:        clientError.ErrorMessage.Code,
			Message:     clientError.ErrorMessage.Message,
			Description: clientError.ErrorMessage.Description,
		})
		return
	}

	var serverError *customerrors.ServerError
	if ok := errors.As(err, &serverError); ok {
		logger := log.GetLogger()
		logger.Error(err.Error(), log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal server error",
		})
		return
	}
}

func ExtractOrgID(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 2 && parts[1] == "t" {
		return parts[2] // e.g., "carbon.super"
	}
	return "carbon.super"
}
