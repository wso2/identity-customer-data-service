package store

import (
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/api_keys/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
)

// InsertAPIKey inserts a new API key into the database using DBClientInterface
func InsertAPIKey(key *model.APIKey) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `INSERT INTO api_keys (api_key, org_id, app_id, state, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.Exec(query, key.APIKey, key.OrgID, key.AppID, key.State, key.ExpiresAt, key.CreatedAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert API key: %w", err)
	}
	return tx.Commit()
}

// GetAPIKeyPerApp retrieves an API key by org and app ID
func GetAPIKeyPerApp(orgID string, appID string) (*model.APIKey, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT api_key, org_id, app_id, state, expires_at, created_at FROM api_keys WHERE org_id = $1 AND app_id = $2 AND state = 'active'`
	rows, err := dbClient.ExecuteQuery(query, orgID, appID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return mapRowToAPIKey(row), nil
}

// GetAPIKey retrieves an API key by its key string
func GetAPIKey(apiKey string) (*model.APIKey, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT api_key, org_id, app_id, state, expires_at, created_at FROM api_keys WHERE api_key = $1`
	rows, err := dbClient.ExecuteQuery(query, apiKey)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return mapRowToAPIKey(row), nil
}

// UpdateState updates the state of an API key
func UpdateState(apiKey string, state string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `UPDATE api_keys SET state = $1 WHERE api_key = $2`
	_, err = tx.Exec(query, state, apiKey)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update API key state: %w", err)
	}
	return tx.Commit()
}

// Helper function to map DB result to APIKey model
func mapRowToAPIKey(row map[string]interface{}) *model.APIKey {
	return &model.APIKey{
		APIKey:    row["api_key"].(string),
		OrgID:     row["org_id"].(string),
		AppID:     row["app_id"].(string),
		State:     row["state"].(string),
		ExpiresAt: row["expires_at"].(int64),
		CreatedAt: row["created_at"].(int64),
	}
}
