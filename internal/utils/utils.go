package utils

import (
	"encoding/json"
	"errors" // Standard Go errors package
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	customerrors "github.com/wso2/identity-customer-data-service/internal/errors" // Alias for the custom errors package
)

func HandleError(c *gin.Context, err error) {
	traceID := c.GetString("traceId")
	if traceID == "" {
		traceID = uuid.NewString()
	}

	switch e := err.(type) {
	case *customerrors.ClientError:
		c.JSON(e.StatusCode, gin.H{
			"error_code":        e.Code,
			"error_message":     e.Message,
			"error_description": e.Description,
			"traceId":           traceID,
		})
	case *customerrors.ServerError:
		log.Printf("[ERROR] %s ", e.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code":        e.Code,
			"error_message":     e.Message,
			"error_description": e.Description,
			"traceId":           traceID,
		})
	default:
		log.Printf("[ERROR] %s ", e.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code":        "CDS-50000",
			"error_message":     "Internal Server Error",
			"error_description": "An unexpected error occurred.",
			"traceId":           traceID,
		})
	}
}

// HandleHTTPError sends an HTTP error response based on the provided error
func HandleHTTPError(w http.ResponseWriter, err error) {
	var clientError *customerrors.ClientError
	if ok := errors.As(err, &clientError); ok {
		w.WriteHeader(clientError.StatusCode)
		json.NewEncoder(w).Encode(clientError.Message)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "Internal server error",
	})
}

func NormalizePropertyType(propertyType string) (string, error) {
	normalized := strings.ToLower(propertyType)
	if goType, ok := constants.GoTypeMapping[normalized]; ok {
		return goType, nil
	}
	return "", fmt.Errorf("unsupported property type: %s", propertyType)
}

// MergeStringValues merges two string values based on the strategy.
func MergeStringValues(existing string, incoming string, strategy string) string {
	switch strategy {
	case "overwrite":
		return incoming
	case "ignore":
		return existing
	default: // default to "combine"
		if existing == "" {
			return incoming
		}
		if incoming == "" || existing == incoming {
			return existing
		}
		return existing + " | " + incoming
	}
}

// MergeStringSlices merges two string slices based on the strategy.
func MergeStringSlices(existing []string, incoming []string, strategy string) []string {
	switch strategy {
	case "overwrite":
		return incoming
	case "ignore":
		return existing
	default: // default to "combine"
		unique := map[string]bool{}
		for _, v := range existing {
			unique[v] = true
		}
		for _, v := range incoming {
			unique[v] = true
		}
		var merged []string
		for val := range unique {
			merged = append(merged, val)
		}
		return merged
	}
}
