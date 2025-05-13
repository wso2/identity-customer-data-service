package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/api_keys/provider"
)

type APIKeyHandler struct{}

func NewApiKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{}
}

func extractTenantAndApp(path string) (orgID, appID string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) >= 9 && parts[1] == "t" && parts[6] == "applications" {
		return parts[2], parts[7], true
	} else if len(parts) >= 7 && parts[4] == "applications" {
		return "-1234", parts[5], true
	}
	return "", "", false
}

func extractAPIKey(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 10 && parts[8] == "api-keys" {
		return parts[9]
	} else if len(parts) >= 9 && parts[6] == "api-keys" {
		return parts[7]
	}
	return ""
}

// AddAPIKey handles adding a new API key
func (ah *APIKeyHandler) AddAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, appID, ok := extractTenantAndApp(r.URL.Path)
	if !ok {
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}
	apiKeyService := provider.NewAPIKeysProvider().GetAPIKeyService()
	apiKey, err := apiKeyService.CreateAPIKey(orgID, appID)
	if err != nil {
		http.Error(w, "failed to create api key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiKey)
}

// GetAPIKey fetches either all or one API key
func (ah *APIKeyHandler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, appID, ok := extractTenantAndApp(r.URL.Path)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}
	apiKeyID := extractAPIKey(r.URL.Path)

	apiKeyService := provider.NewAPIKeysProvider().GetAPIKeyService()
	if apiKeyID != "" {
		key, err := apiKeyService.GetAPIKey(apiKeyID)
		if err != nil || key == nil {
			http.Error(w, "api key not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(key)
		return
	}
	summary, err := apiKeyService.GetAPIKeyPerApp(orgID, appID)
	if err != nil || summary == nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "api key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// RotateAPIKey regenerates a key for a given app/key
func (ah *APIKeyHandler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, appID, ok := extractTenantAndApp(r.URL.Path)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}

	apiKeyService := provider.NewAPIKeysProvider().GetAPIKeyService()
	newKey, err := apiKeyService.RotateAPIKey(orgID, appID)
	if err != nil {
		http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newKey)
}

// RevokeAPIKey disables the API key
func (ah *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	apiKey := extractAPIKey(r.URL.Path)
	if apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "missing api key id", http.StatusBadRequest)
		return
	}
	apiKeyService := provider.NewAPIKeysProvider().GetAPIKeyService()
	if err := apiKeyService.RevokeAPIKey(apiKey); err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("api key revoked"))
}
