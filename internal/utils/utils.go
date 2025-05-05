package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	"github.com/wso2/identity-customer-data-service/internal/errors"
	"log"
	"net/http"
	"strings"
)

func HandleError(c *gin.Context, err error) {
	traceID := c.GetString("traceId")
	if traceID == "" {
		traceID = uuid.NewString()
	}

	switch e := err.(type) {
	case *errors.ClientError:
		c.JSON(e.StatusCode, gin.H{
			"error_code":        e.Code,
			"error_message":     e.Message,
			"error_description": e.Description,
			"traceId":           traceID,
		})
	case *errors.ServerError:
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

// ApplyMasking applies the given masking strategy to a string value.
func ApplyMasking(value string, strategy string) string {
	switch strings.ToLower(strategy) {
	case "partial":
		return maskPartial(value)
	case "hash":
		return hashValue(value)
	case "redact":
		return "REDACTED"
	default:
		return value // no masking
	}
}

// maskPartial masks the middle part of a string (e.g., email)
func maskPartial(value string) string {
	// todo: see if this has to be applied for profiles
	if len(value) <= 4 {
		return "***"
	}
	visible := 2
	masked := strings.Repeat("*", len(value)-2*visible)
	return value[:visible] + masked + value[len(value)-visible:]
}

// hashValue returns a SHA256 hash of the value
func hashValue(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

// ReverseMasking returns the visible portions of a partially masked string.
func ReverseMasking(maskedValue, strategy string) string {
	switch strings.ToLower(strategy) {
	case "partial":
		return getVisibleFromPartial(maskedValue)
	default:
		return "" // not reversible
	}
}

// getVisibleFromPartial extracts the first and last 2 characters
func getVisibleFromPartial(value string) string {
	if len(value) <= 4 {
		return ""
	}
	return value[:2] + "..." + value[len(value)-2:]
}
