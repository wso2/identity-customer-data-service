package utils

import (
	"encoding/json"
	"errors"                                                                             // Standard Go errors package
	customerrors "github.com/wso2/identity-customer-data-service/internal/system/errors" // Alias for the custom errors
	"net/http"
)

// HandleHTTPError sends an HTTP error response based on the provided error
func HandleHTTPError(w http.ResponseWriter, err error) {
	var clientError *customerrors.ClientError
	if ok := errors.As(err, &clientError); ok {
		w.WriteHeader(clientError.StatusCode)
		_ = json.NewEncoder(w).Encode(clientError.Message)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Internal server error",
	})
}
