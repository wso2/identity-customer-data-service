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
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"strings"
)

// Helper to marshal JSONB fields, handling nil maps
func marshalJsonb(data map[string]interface{}) (sql.NullString, error) {
	if data == nil {
		// Represent nil map as SQL NULL, or an empty JSON object if preferred:
		// return sql.NullString{String: "{}", Valid: true}, nil
		return sql.NullString{Valid: false}, nil
	}
	bytes, err := json.Marshal(data)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to marshal metadata to JSON for storing in database."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.MARSHAL_JSON.Code,
			Message:     errors2.MARSHAL_JSON.Message,
			Description: errorMsg,
		}, err)
		return sql.NullString{}, serverError
	}
	return sql.NullString{String: string(bytes), Valid: true}, nil
}

// AddEvent inserts a single event
func AddEvent(event model.Event) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for adding event with id: %s", event.EventId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT.Code,
			Message:     errors2.ADD_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	propertiesJson, err := marshalJsonb(event.Properties)
	if err != nil {
		return err
	}
	contextJson, err := marshalJsonb(event.Context)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
        INSERT INTO %s (profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constants.EventCollection)

	_, err = dbClient.ExecuteQuery(query,
		event.ProfileId, event.EventType, event.EventName, event.EventId,
		event.AppId, event.OrgId, event.EventTimestamp, propertiesJson, contextJson,
	)

	if err != nil {
		errorMsg := fmt.Sprintf("Failed in  adding event with id: %s", event.EventId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.MARSHAL_JSON.Code,
			Message:     errors2.MARSHAL_JSON.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Event with event id: %s persisted successfully", event.EventId))
	return nil
}

// AddEvents inserts multiple events in bulk using a transaction
func AddEvents(events []model.Event) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client for adding events"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT.Code,
			Message:     errors2.ADD_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			errorMsg := "Failed to rollback while adding events"
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_EVENT.Code,
				Message:     errors2.ADD_EVENT.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		errorMsg := "Failed to begin transaction to add events"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT.Code,
			Message:     errors2.ADD_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := fmt.Sprintf(`
        INSERT INTO %s (profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constants.EventCollection)

	for _, event := range events {
		propertiesJson, err := marshalJsonb(event.Properties)
		if err != nil {
			return err
		}
		contextJson, err := marshalJsonb(event.Context)
		if err != nil {
			return err
		}

		_, err = tx.Exec(query,
			event.ProfileId, event.EventType, event.EventName, event.EventId,
			event.AppId, event.OrgId, event.EventTimestamp, propertiesJson, contextJson,
		)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to insert event: %s during batch addition", event.EventId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_EVENT.Code,
				Message:     errors2.ADD_EVENT.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}

	if err := tx.Commit(); err != nil {
		errorMsg := "Failed to commit transaction"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_EVENT.Code,
			Message:     errors2.ADD_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// FindEvents fetches events based on dynamic filters and time range
func FindEvents(filters []string, timeFilter map[string]int) ([]model.Event, error) { // Assuming timeFilter values are int64 for Unix timestamps

	logger := log.GetLogger()
	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("SELECT profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context FROM %s WHERE 1=1", constants.EventCollection))

	var args []interface{}
	argCount := 1

	// Whitelist of allowed top-level field names and JSONB fields
	// For JSONB fields, we'll allow path traversal.
	allowedFields := map[string]bool{
		"profile_id":     true,
		"event_type":     true,
		"event_name":     true,
		"application_id": true,
		"org_id":         true,
		"properties":     true, // Indicates that 'properties' is a JSONB field and sub-paths are allowed
	}

	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			logger.Debug(fmt.Sprintf("Skipping malformed filter: %s", f))
			continue
		}
		field, operator, value := parts[0], strings.ToLower(parts[1]), parts[2]

		// Check if the field is a path within a JSONB column (e.g., "properties.abc")
		var sqlField string
		jsonPathParts := strings.SplitN(field, ".", 2)
		baseField := jsonPathParts[0]

		if !allowedFields[baseField] {
			logger.Debug("Invalid base field name in filter: " + baseField)
			continue
		}

		isJsonbField := (baseField == "properties" || baseField == "context") && len(jsonPathParts) > 1

		if isJsonbField {
			if len(strings.Split(jsonPathParts[1], ".")) > 1 {
				keys := strings.Split(jsonPathParts[1], ".")
				pathBuilder := strings.Builder{}
				pathBuilder.WriteString(baseField)
				for i, key := range keys {
					if i == len(keys)-1 { // Last key, get as text
						pathBuilder.WriteString(fmt.Sprintf("->>'%s'", key))
					} else { // Intermediate key, get as JSON object
						pathBuilder.WriteString(fmt.Sprintf("->'%s'", key))
					}
				}
				sqlField = pathBuilder.String()

			} else {
				// Direct key like "properties.abc" -> properties->>'abc'
				sqlField = fmt.Sprintf("%s->>'%s'", baseField, jsonPathParts[1])
			}
		} else if allowedFields[field] {
			// Standard field
			sqlField = field
		} else {
			logger.Debug(fmt.Sprintf("Invalid field name in filter: %s", field))
			continue
		}

		// Apply operators
		switch operator {
		case "eq":
			queryBuilder.WriteString(fmt.Sprintf(" AND %s = $%d", sqlField, argCount))
			args = append(args, value)
			argCount++
		case "sw": // starts with
			queryBuilder.WriteString(fmt.Sprintf(" AND %s LIKE $%d", sqlField, argCount))
			args = append(args, value+"%")
			argCount++
		case "co": // contains
			queryBuilder.WriteString(fmt.Sprintf(" AND %s LIKE $%d", sqlField, argCount))
			args = append(args, "%"+value+"%")
			argCount++
		case "neq":
			queryBuilder.WriteString(fmt.Sprintf(" AND %s <> $%d", sqlField, argCount))
			args = append(args, value)
			argCount++
		case "gt": // Greater than - ensure value is numeric or comparable
			queryBuilder.WriteString(fmt.Sprintf(" AND %s > $%d", sqlField, argCount))
			args = append(args, value) // Consider type casting in SQL if needed, e.g., (%s)::numeric
			argCount++
		case "lt": // Less than
			queryBuilder.WriteString(fmt.Sprintf(" AND %s < $%d", sqlField, argCount))
			args = append(args, value)
			argCount++
			// Add other operators as needed.
			// For JSONB existence: "properties?'key'"
			// For JSONB containment: "properties @> '{\"key\":\"value\"}'"
		}
	}

	// Add time filter if provided
	if ts, ok := timeFilter["event_timestamp_gt"]; ok {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_timestamp > $%d", argCount))
		args = append(args, ts)
		argCount++
	}
	if ts, ok := timeFilter["event_timestamp_lt"]; ok {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_timestamp < $%d", argCount))
		args = append(args, ts)
		argCount++
	}
	if ts, ok := timeFilter["event_timestamp_gte"]; ok {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_timestamp >= $%d", argCount))
		args = append(args, ts)
		argCount++
	}
	if ts, ok := timeFilter["event_timestamp_lte"]; ok {
		queryBuilder.WriteString(fmt.Sprintf(" AND event_timestamp <= $%d", argCount))
		args = append(args, ts)
	}

	queryString := queryBuilder.String()
	logger.Info(fmt.Sprintf("Executing query: %s with args: %v", queryString, args)) // Logging query

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := "Failed to get db client when filtering events"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT.Code,
			Message:     errors2.GET_EVENT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	results, err := dbClient.ExecuteQuery(queryString, args...)
	if err != nil {
		errorMsg := "Failed to filtering events"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT.Code,
			Message:     errors2.GET_EVENT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var events []model.Event
	for _, row := range results {
		var event model.Event

		event.ProfileId = row["profile_id"].(string)
		event.EventType = row["event_type"].(string)
		event.EventName = row["event_name"].(string)
		event.EventId = row["event_id"].(string)
		event.AppId = row["application_id"].(string)
		event.OrgId = row["org_id"].(string)
		event.EventTimestamp = int(row["event_timestamp"].(int64))

		// Handle properties
		if raw, ok := row["properties"].([]byte); ok && len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Properties); err != nil {
				errorMsg := "Failed in unmarshalling events from database to event object"
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UNMARSHAL_JSON.Code,
					Message:     errors2.UNMARSHAL_JSON.Message,
					Description: errorMsg,
				}, err)
				return nil, serverError
			}
		} else {
			event.Properties = make(map[string]interface{}) // Initialize empty if null or empty
		}

		// Handle context
		if raw, ok := row["context"].([]byte); ok && len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Context); err != nil {
				errorMsg := "Failed in unmarshalling event context from database to event object"
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UNMARSHAL_JSON.Code,
					Message:     errors2.UNMARSHAL_JSON.Message,
					Description: errorMsg,
				}, err)
				return nil, serverError
			}
		} else {
			event.Context = make(map[string]interface{}) // Initialize empty if null or empty
		}

		events = append(events, event)
	}

	return events, nil
}

// FindEvent fetches a single event by its ID
func FindEvent(eventId string) (*model.Event, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching event with id: %s", eventId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT.Code,
			Message:     errors2.GET_EVENT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := fmt.Sprintf(`
        SELECT profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context
        FROM %s WHERE event_id = $1`, constants.EventCollection)

	var event model.Event
	var propertiesRaw, contextRaw sql.NullString

	results, err := dbClient.ExecuteQuery(query, eventId)
	for _, row := range results {
		event.ProfileId = row["profile_id"].(string)
		event.EventType = row["event_type"].(string)
		event.EventName = row["event_name"].(string)
		event.EventId = row["event_id"].(string)
		event.AppId = row["application_id"].(string)
		event.OrgId = row["org_id"].(string)
		event.EventTimestamp = int(row["event_timestamp"].(int64))
		// Handle properties
		if raw, ok := row["properties"].([]byte); ok && len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Properties); err != nil {
				errorMsg := "Failed in unmarshalling events from database to event object"
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UNMARSHAL_JSON.Code,
					Message:     errors2.UNMARSHAL_JSON.Message,
					Description: errorMsg,
				}, err)
				return nil, serverError
			}
		} else {
			event.Properties = make(map[string]interface{}) // Initialize empty if null or empty
		}

		// Handle context
		if raw, ok := row["context"].([]byte); ok && len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Context); err != nil {
				errorMsg := "Failed in unmarshalling event context from database to event object"
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UNMARSHAL_JSON.Code,
					Message:     errors2.UNMARSHAL_JSON.Message,
					Description: errorMsg,
				}, err)
				return nil, serverError
			}
		} else {
			event.Context = make(map[string]interface{}) // Initialize empty if null or empty
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debug(fmt.Sprintf("No event found with id: %s", eventId))
			return nil, nil
		}
		return nil, err
	}

	if propertiesRaw.Valid {
		if err := json.Unmarshal([]byte(propertiesRaw.String), &event.Properties); err != nil {
			errorMsg := fmt.Sprintf("Failed when unmarshalling event with id: %s", eventId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_EVENT.Code,
				Message:     errors2.GET_EVENT.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
	} else {
		event.Properties = nil
	}
	if contextRaw.Valid {
		if err := json.Unmarshal([]byte(contextRaw.String), &event.Context); err != nil {
			errorMsg := fmt.Sprintf("Failed when unmarshalling event with id: %s", eventId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_EVENT.Code,
				Message:     errors2.GET_EVENT.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
	} else {
		event.Context = nil
	}

	return &event, nil
}

// DeleteEvent deletes a single event by its ID
func DeleteEvent(eventId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting event with id: %s", eventId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE event_id = $1", constants.EventCollection)
	result, err := dbClient.ExecuteQuery(query, eventId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting event with id: %s", eventId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	rowsAffected := len(result)
	if rowsAffected == 0 {
		logger.Debug(fmt.Sprintf("No event found with id: %s", eventId))
		return sql.ErrNoRows
	}
	return nil
}

// DeleteEventsByProfileId deletes all events for a given profile ID
func DeleteEventsByProfileId(profileId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting all events for profile with "+
			"id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE profile_id = $1", constants.EventCollection)
	_, err = dbClient.ExecuteQuery(query, profileId) // RowsAffected could be checked
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to delete events for profile with id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	return nil
}

// DeleteEventsByAppID deletes events for a specific profile ID and application ID
func DeleteEventsByAppID(profileId, appId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting all events for profile with "+
			"id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE profile_id = $1 AND application_id = $2", constants.EventCollection)
	_, err = dbClient.ExecuteQuery(query, profileId, appId) // RowsAffected could be checked
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to delete events for profile with id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_EVENT.Code,
			Message:     errors2.DELETE_EVENT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	return nil
}
