/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package store

import (
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
)

// InsertEventStreamId inserts a new event_stream_id  into the database using DBClientInterface
func InsertEventStreamId(eventStreamId *model.EventStreamId) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `INSERT INTO event_stream_ids (event_stream_id, org_id, app_id, state, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.Exec(query, eventStreamId.EventStreamId, eventStreamId.OrgID, eventStreamId.AppID, eventStreamId.State, eventStreamId.ExpiresAt, eventStreamId.CreatedAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert event_stream_id eventStreamId: %w", err)
	}
	return tx.Commit()
}

func GetEventStreamIdsPerApp(orgID string, appID string) ([]*model.EventStreamId, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT event_stream_id, org_id, app_id, state, expires_at, created_at 
	          FROM event_stream_ids 
	          WHERE org_id = $1 AND app_id = $2`

	rows, err := dbClient.ExecuteQuery(query, orgID, appID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var result []*model.EventStreamId
	for _, row := range rows {
		result = append(result, mapRowToEventStreamId(row))
	}
	return result, nil
}

// GetEventStreamId retrieves an API key by its key string
func GetEventStreamId(eventStreamId string) (*model.EventStreamId, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT event_stream_id, org_id, app_id, state, expires_at, created_at FROM event_stream_ids WHERE event_stream_id = $1`
	rows, err := dbClient.ExecuteQuery(query, eventStreamId)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return mapRowToEventStreamId(row), nil
}

// UpdateState updates the state of an API key
func UpdateState(eventStreamId string, state string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `UPDATE event_stream_ids SET state = $1 WHERE event_stream_id = $2`
	_, err = tx.Exec(query, state, eventStreamId)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update event_stream_id state: %w", err)
	}
	return tx.Commit()
}

// Helper function to map DB result to EventStreamId model
func mapRowToEventStreamId(row map[string]interface{}) *model.EventStreamId {
	return &model.EventStreamId{
		EventStreamId: row["event_stream_id"].(string),
		OrgID:         row["org_id"].(string),
		AppID:         row["app_id"].(string),
		State:         row["state"].(string),
		ExpiresAt:     row["expires_at"].(int64),
		CreatedAt:     row["created_at"].(int64),
	}
}
