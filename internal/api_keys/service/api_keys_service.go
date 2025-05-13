package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/api_keys/model"
	"github.com/wso2/identity-customer-data-service/internal/api_keys/store"
)

type APIKeyServiceInterface interface {
	CreateAPIKey(orgID, appID string) (*model.APIKey, error)
	GetAPIKeyPerApp(orgID, appID string) (*model.APIKey, error)
	GetAPIKey(apiKey string) (*model.APIKey, error)
	RotateAPIKey(orgID, appID string) (*model.APIKey, error)
	RevokeAPIKey(apiKey string) error
}

// APIKeyService is the default implementation of APIKeyServiceInterface.
type APIKeyService struct{}

// GetAPIKeyService returns a concrete service with store injected
func GetAPIKeyService() APIKeyServiceInterface {
	return &APIKeyService{}
}

// CreateAPIKey generates and stores a new API key
func (s *APIKeyService) CreateAPIKey(orgID, appID string) (*model.APIKey, error) {
	key := generateSecureToken()
	now := time.Now().Unix()
	exp := now + (60 * 60 * 24 * 365) // 1 year

	apiKey := &model.APIKey{
		APIKey:    key,
		OrgID:     orgID,
		AppID:     appID,
		State:     "active",
		ExpiresAt: exp,
		CreatedAt: now,
	}

	apiKeyExisting, _ := store.GetAPIKeyPerApp(orgID, appID)
	if apiKeyExisting != nil {
		return nil, fmt.Errorf("API key already exists for this org/app")
	}

	if err := store.InsertAPIKey(apiKey); err != nil {
		return nil, fmt.Errorf("failed to insert API key: %w", err)
	}
	return apiKey, nil
}

// GetAPIKeyPerApp returns an API key for a specific org and app
func (s *APIKeyService) GetAPIKeyPerApp(orgID, appID string) (*model.APIKey, error) {
	apiKey, err := store.GetAPIKeyPerApp(orgID, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve API key: %w", err)
	}
	return apiKey, nil
}

// GetAPIKey retrieves an API key by its value
func (s *APIKeyService) GetAPIKey(apiKey string) (*model.APIKey, error) {
	key, err := store.GetAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve API key: %w", err)
	}
	return key, nil
}

// RotateAPIKey revokes the old API key and creates a new one for the same org/app
func (s *APIKeyService) RotateAPIKey(orgID, appID string) (*model.APIKey, error) {

	oldAPIKey, _ := store.GetAPIKeyPerApp(orgID, appID)
	if err := store.UpdateState(oldAPIKey.APIKey, "revoked"); err != nil {
		return nil, fmt.Errorf("failed to revoke old API key: %w", err)
	}

	key := generateSecureToken()
	now := time.Now().Unix()
	exp := now + (60 * 60 * 24 * 365)

	newApiKey := &model.APIKey{
		APIKey:    key,
		OrgID:     orgID,
		AppID:     appID,
		State:     "active",
		ExpiresAt: exp,
		CreatedAt: now,
	}

	if err := store.InsertAPIKey(newApiKey); err != nil {
		return nil, fmt.Errorf("failed to insert new API key: %w", err)
	}
	return newApiKey, nil
}

// RevokeAPIKey sets the state of the key to 'revoked'
func (s *APIKeyService) RevokeAPIKey(apiKey string) error {
	if err := store.UpdateState(apiKey, "revoked"); err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	return nil
}

func generateSecureToken() string {
	return uuid.New().String()
}
