package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
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
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(bytes), Valid: true}, nil
}

// AddEvent inserts a single event
func AddEvent(event model.Event) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	propertiesJson, err := marshalJsonb(event.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}
	contextJson, err := marshalJsonb(event.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	query := fmt.Sprintf(`
        INSERT INTO %s (profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constants.EventCollection)

	_, err = dbClient.ExecuteQuery(query,
		event.ProfileId, event.EventType, event.EventName, event.EventId,
		event.AppId, event.OrgId, event.EventTimestamp, propertiesJson, contextJson,
	)

	if err != nil {
		//logger.LogMessage("ERROR", "Failed to insert event: "+err.Error())
		return err
	}
	// logger.LogMessage("INFO", "Event inserted successfully for user "+event.ProfileId)
	return nil
}

// AddEvents inserts multiple events in bulk using a transaction
func AddEvents(events []model.Event) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := fmt.Sprintf(`
        INSERT INTO %s (profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constants.EventCollection)

	for _, event := range events {
		propertiesJson, err := marshalJsonb(event.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal properties for event %s: %w", event.EventId, err)
		}
		contextJson, err := marshalJsonb(event.Context)
		if err != nil {
			return fmt.Errorf("failed to marshal context for event %s: %w", event.EventId, err)
		}

		_, err = tx.Exec(query,
			event.ProfileId, event.EventType, event.EventName, event.EventId,
			event.AppId, event.OrgId, event.EventTimestamp, propertiesJson, contextJson,
		)
		if err != nil {
			// logger.LogMessage("ERROR", "Failed to insert one event in batch: "+err.Error())
			return fmt.Errorf("failed to insert event %s during batch: %w", event.EventId, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// logger.LogMessage("INFO", "Batch events inserted successfully")
	return nil
}

// FindEvents fetches events based on dynamic filters and time range
func FindEvents(filters []string, timeFilter map[string]int) ([]model.Event, error) { // Assuming timeFilter values are int64 for Unix timestamps

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
			logger.Info(fmt.Sprintf("Skipping malformed filter: %s", f)) // Using log instead of logger for simplicity
			continue
		}
		field, operator, value := parts[0], strings.ToLower(parts[1]), parts[2]

		// Check if the field is a path within a JSONB column (e.g., "properties.abc")
		var sqlField string
		jsonPathParts := strings.SplitN(field, ".", 2)
		baseField := jsonPathParts[0]

		if !allowedFields[baseField] {
			logger.Info(fmt.Sprintf("Invalid base field name in filter: %s", baseField))
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
			logger.Info(fmt.Sprintf("Invalid field name in filter: %s", field))
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
		argCount++
	}

	queryString := queryBuilder.String()
	logger.Info(fmt.Sprintf("Executing query: %s with args: %v", queryString, args)) // Logging query

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	results, err := dbClient.ExecuteQuery(queryString, args...)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	var events []model.Event
	for _, row := range results {
		var event model.Event
		var propertiesRaw, contextRaw sql.NullString

		event.ProfileId = row["profile_id"].(string)
		event.EventType = row["event_type"].(string)
		event.EventName = row["event_name"].(string)
		event.EventId = row["event_id"].(string)
		event.AppId = row["application_id"].(string)
		event.OrgId = row["org_id"].(string)
		event.EventTimestamp = int(row["event_timestamp"].(int64))
		propertiesRaw = row["properties"].(sql.NullString)
		contextRaw = row["context"].(sql.NullString)

		if propertiesRaw.Valid {
			if err := json.Unmarshal([]byte(propertiesRaw.String), &event.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal properties for event %s: %w", event.EventId, err)
			}
		} else {
			event.Properties = make(map[string]interface{}) // Initialize to empty map if nil
		}

		if contextRaw.Valid {
			if err := json.Unmarshal([]byte(contextRaw.String), &event.Context); err != nil {
				return nil, fmt.Errorf("failed to unmarshal context for event %s: %w", event.EventId, err)
			}
		} else {
			event.Context = make(map[string]interface{}) // Initialize to empty map if nil
		}
		events = append(events, event)
	}

	return events, nil
}

// FindEvent fetches a single event by its ID
func FindEvent(eventId string) (*model.Event, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
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
		propertiesRaw = row["properties"].(sql.NullString)
		contextRaw = row["context"].(sql.NullString)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a specific "not found" error
		}
		return nil, err
	}

	if propertiesRaw.Valid {
		if err := json.Unmarshal([]byte(propertiesRaw.String), &event.Properties); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	} else {
		event.Properties = nil
	}
	if contextRaw.Valid {
		if err := json.Unmarshal([]byte(contextRaw.String), &event.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
	} else {
		event.Context = nil
	}

	return &event, nil
}

// DeleteEvent deletes a single event by its ID
func DeleteEvent(eventId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE event_id = $1", constants.EventCollection)
	result, err := dbClient.ExecuteQuery(query, eventId)
	if err != nil {
		return err
	}
	rowsAffected := len(result)
	if rowsAffected == 0 {
		return sql.ErrNoRows // Or a custom "not found" error
	}
	return nil
}

// DeleteEventsByProfileId deletes all events for a given profile ID
func DeleteEventsByProfileId(profileId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE profile_id = $1", constants.EventCollection)
	_, err = dbClient.ExecuteQuery(query, profileId) // RowsAffected could be checked
	return err
}

// DeleteEventsByAppID deletes events for a specific profile ID and application ID
func DeleteEventsByAppID(profileId, appId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := fmt.Sprintf("DELETE FROM %s WHERE profile_id = $1 AND application_id = $2", constants.EventCollection)
	_, err = dbClient.ExecuteQuery(query, profileId, appId) // RowsAffected could be checked
	return err
}

// FindEventsWithFilter fetches events based on a map of filters.
func FindEventsWithFilter(filter map[string]interface{}) ([]model.Event, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("SELECT profile_id, event_type, event_name, event_id, application_id, org_id, event_timestamp, properties, context FROM %s", constants.EventCollection))

	var args []interface{}
	argCount := 1
	conditions := []string{}

	for key, value := range filter {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", key, argCount))
		args = append(args, value)
		argCount++
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	results, err := dbClient.ExecuteQuery(queryBuilder.String(), args...)
	if err != nil {
		return nil, err
	}

	var events []model.Event
	for _, row := range results {
		var event model.Event
		var propertiesRaw, contextRaw sql.NullString

		event.ProfileId = row["profile_id"].(string)
		event.EventType = row["event_type"].(string)
		event.EventName = row["event_name"].(string)
		event.EventId = row["event_id"].(string)
		event.AppId = row["application_id"].(string)
		event.OrgId = row["org_id"].(string)
		event.EventTimestamp = int(row["event_timestamp"].(int64))
		propertiesRaw = row["properties"].(sql.NullString)
		contextRaw = row["context"].(sql.NullString)

		if propertiesRaw.Valid {
			if err := json.Unmarshal([]byte(propertiesRaw.String), &event.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal properties for event %s: %w", event.EventId, err)
			}
		} else {
			event.Properties = nil
		}
		if contextRaw.Valid {
			if err := json.Unmarshal([]byte(contextRaw.String), &event.Context); err != nil {
				return nil, fmt.Errorf("failed to unmarshal context for event %s: %w", event.EventId, err)
			}
		} else {
			event.Context = nil
		}
		events = append(events, event)
	}
	return events, err
}
