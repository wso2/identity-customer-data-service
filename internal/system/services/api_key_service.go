package services

import (
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/api_keys/handler"
)

type APIKeyService struct {
	apiKeyHandler *handler.APIKeyHandler
}

func NewAPIKeyService(mux *http.ServeMux, apiBasePath string) *APIKeyService {
	instance := &APIKeyService{
		apiKeyHandler: handler.NewApiKeyHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)
	return instance
}

func (s *APIKeyService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		// Add or list API keys: POST or GET /applications/{app-uuid}/api-keys
		case r.Method == http.MethodPost && strings.Contains(path, "/api-keys"):
			s.apiKeyHandler.AddAPIKey(w, r)
		case r.Method == http.MethodGet && strings.Contains(path, "/api-keys"):
			s.apiKeyHandler.GetAPIKey(w, r)
		// Rotate: PUT /applications/{app-uuid}/api-keys/{key-id}/rotate
		case r.Method == http.MethodPut && strings.Contains(path, "/api-keys/") && strings.HasSuffix(path, "/rotate"):
			s.apiKeyHandler.RotateAPIKey(w, r)
		// Revoke: PUT /applications/{app-uuid}/api-keys/{key-id}/revoke
		case r.Method == http.MethodPut && strings.Contains(path, "/api-keys/") && strings.HasSuffix(path, "/revoke"):
			s.apiKeyHandler.RevokeAPIKey(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}
