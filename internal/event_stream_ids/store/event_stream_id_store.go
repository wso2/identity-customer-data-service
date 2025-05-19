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
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// InsertEventStreamId inserts a new event_stream_id  into the database using DBClientInterface
func InsertEventStreamId(eventStreamId *model.EventStreamId) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for event stream id with id: %s",
			eventStreamId.EventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT_STREAM_ID.Code,
			Message:     errors2.ADD_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction to add event stream id with id: %s",
			eventStreamId.EventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT_STREAM_ID.Code,
			Message:     errors2.ADD_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := `INSERT INTO event_stream_ids (event_stream_id, org_id, app_id, state, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.Exec(query, eventStreamId.EventStreamId, eventStreamId.OrgID, eventStreamId.AppID, eventStreamId.State, eventStreamId.ExpiresAt, eventStreamId.CreatedAt)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to rollback adding event stream id: %s",
				eventStreamId.EventStreamId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_EVENT_STREAM_ID.Code,
				Message:     errors2.ADD_EVENT_STREAM_ID.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		errorMsg := fmt.Sprintf("Failed to add event stream id: %s",
			eventStreamId.EventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT_STREAM_ID.Code,
			Message:     errors2.ADD_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	return tx.Commit()
}

func GetEventStreamIdsPerApp(orgID string, appID string) ([]*model.EventStreamId, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for event stream id for app: %s", appID)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := `SELECT event_stream_id, org_id, app_id, state, expires_at, created_at 
	          FROM event_stream_ids 
	          WHERE org_id = $1 AND app_id = $2`

	rows, err := dbClient.ExecuteQuery(query, orgID, appID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed in fetching event stream ids for app: %s", appID)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
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
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for  fetching meta data for event stream id: %s", eventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := `SELECT event_stream_id, org_id, app_id, state, expires_at, created_at FROM event_stream_ids WHERE event_stream_id = $1`
	rows, err := dbClient.ExecuteQuery(query, eventStreamId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed in fetching meta data for event stream id: %s", eventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return mapRowToEventStreamId(row), nil
}

// UpdateState updates the state of an API key
func UpdateState(eventStreamId string, state string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for updating meta data for event stream id: %s", eventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_EVENT_STREAM_ID.Code,
			Message:     errors2.UPDATE_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for updating meta data for event stream id: %s", eventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_EVENT_STREAM_ID.Code,
			Message:     errors2.UPDATE_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := `UPDATE event_stream_ids SET state = $1 WHERE event_stream_id = $2`
	_, err = tx.Exec(query, state, eventStreamId)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to rollback updating meta data for event stream id: %s", eventStreamId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_EVENT_STREAM_ID.Code,
				Message:     errors2.UPDATE_EVENT_STREAM_ID.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		errorMsg := fmt.Sprintf("Failed to updating meta data for event stream id: %s", eventStreamId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_EVENT_STREAM_ID.Code,
			Message:     errors2.UPDATE_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, err)
		return serverError
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
