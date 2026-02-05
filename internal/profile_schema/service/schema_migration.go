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

package service

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	psstr "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// MigrateProfileData migrates profile data when schema attributes change
// This handles:
// 1. Type changes (e.g., string -> integer, integer -> string)
// 2. Multi-valued changes (single value -> array, array -> single value)
func MigrateProfileData(orgId, attributeId string, oldSchema, newSchema model.ProfileSchemaAttribute) error {
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Starting profile data migration for attribute %s in org %s", attributeId, orgId))

	// Check if migration is needed
	if oldSchema.ValueType == newSchema.ValueType && oldSchema.MultiValued == newSchema.MultiValued {
		logger.Info("No migration needed - schema attributes unchanged")
		return nil
	}

	// Get the attribute name without scope prefix
	attrKey := getAttributeKey(newSchema.AttributeName)
	if attrKey == "" {
		logger.Warn(fmt.Sprintf("Invalid attribute name format: %s", newSchema.AttributeName))
		return nil
	}

	// Determine the scope (identity_attributes, traits, or application_data)
	scope := getAttributeScope(newSchema.AttributeName)
	if scope == "" {
		logger.Warn(fmt.Sprintf("Cannot determine scope for attribute: %s", newSchema.AttributeName))
		return nil
	}

	logger.Info(fmt.Sprintf("Migrating data for %s.%s from %s (multi: %v) to %s (multi: %v)",
		scope, attrKey, oldSchema.ValueType, oldSchema.MultiValued, newSchema.ValueType, newSchema.MultiValued))

	// Perform the migration
	return migrateProfilesForAttribute(orgId, scope, attrKey, oldSchema, newSchema)
}

// getAttributeKey extracts the key from the full attribute name
// e.g., "identity_attributes.email" -> "email"
func getAttributeKey(fullName string) string {
	parts := splitAttributeName(fullName)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// getAttributeScope extracts the scope from the full attribute name
// e.g., "identity_attributes.email" -> "identity_attributes"
func getAttributeScope(fullName string) string {
	parts := splitAttributeName(fullName)
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// splitAttributeName splits an attribute name by the first dot
func splitAttributeName(name string) []string {
	for i, c := range name {
		if c == '.' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name}
}

// migrateProfilesForAttribute migrates all profiles for a specific attribute
func migrateProfilesForAttribute(orgId, scope, attrKey string, oldSchema, newSchema model.ProfileSchemaAttribute) error {
	logger := log.GetLogger()
	
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("Failed to get database client", log.Error(err))
		return err
	}
	defer dbClient.Close()

	// Get the JSONB column name based on scope
	columnName := ""
	switch scope {
	case constants.IdentityAttributes:
		columnName = "identity_attributes"
	case constants.Traits:
		columnName = "traits"
	default:
		// For application_data, handle separately
		return migrateApplicationData(orgId, attrKey, oldSchema, newSchema)
	}

	// Query to get all profiles in the org with this attribute
	query := fmt.Sprintf(`
		SELECT profile_id, %s 
		FROM profiles 
		WHERE org_handle = $1 
		AND %s ? $2
	`, columnName, columnName)

	rows, err := dbClient.ExecuteQuery(query, orgId, attrKey)
	if err != nil {
		logger.Error("Failed to query profiles for migration", log.Error(err))
		return err
	}

	profilesUpdated := 0
	for _, row := range rows {
		profileId := row["profile_id"].(string)
		jsonData := row[columnName].([]byte)

		var attrs map[string]interface{}
		if err := json.Unmarshal(jsonData, &attrs); err != nil {
			logger.Warn(fmt.Sprintf("Failed to unmarshal %s for profile %s", columnName, profileId), log.Error(err))
			continue
		}

		if value, exists := attrs[attrKey]; exists {
			// Transform the value
			newValue, err := transformValue(value, oldSchema, newSchema)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to transform value for profile %s", profileId), log.Error(err))
				continue
			}

			// Update the attribute
			attrs[attrKey] = newValue

			// Marshal back to JSON
			updatedJSON, err := json.Marshal(attrs)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to marshal updated %s for profile %s", columnName, profileId), log.Error(err))
				continue
			}

			// Update the profile
			updateQuery := fmt.Sprintf(`
				UPDATE profiles 
				SET %s = $1, updated_at = NOW() 
				WHERE profile_id = $2 AND org_handle = $3
			`, columnName)

			_, err = dbClient.ExecuteQuery(updateQuery, updatedJSON, profileId, orgId)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to update profile %s", profileId), log.Error(err))
				continue
			}

			profilesUpdated++
		}
	}

	logger.Info(fmt.Sprintf("Migration completed: updated %d profiles", profilesUpdated))
	return nil
}

// migrateApplicationData migrates application data attributes
func migrateApplicationData(orgId, attrKey string, oldSchema, newSchema model.ProfileSchemaAttribute) error {
	logger := log.GetLogger()
	
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("Failed to get database client", log.Error(err))
		return err
	}
	defer dbClient.Close()

	// Query to get all application data in the org with this attribute
	query := `
		SELECT ad.app_data_id, ad.profile_id, ad.app_id, ad.application_data 
		FROM application_data ad
		INNER JOIN profiles p ON ad.profile_id = p.profile_id
		WHERE p.org_handle = $1
		AND ad.application_data ? $2
	`

	// If application identifier is specified, filter by it
	if newSchema.ApplicationIdentifier != "" {
		query += ` AND ad.app_id = $3`
	}

	var rows []map[string]interface{}
	if newSchema.ApplicationIdentifier != "" {
		rows, err = dbClient.ExecuteQuery(query, orgId, attrKey, newSchema.ApplicationIdentifier)
	} else {
		rows, err = dbClient.ExecuteQuery(query, orgId, attrKey)
	}

	if err != nil {
		logger.Error("Failed to query application data for migration", log.Error(err))
		return err
	}

	profilesUpdated := 0
	for _, row := range rows {
		appDataId := row["app_data_id"]
		jsonData := row["application_data"].([]byte)

		var attrs map[string]interface{}
		if err := json.Unmarshal(jsonData, &attrs); err != nil {
			logger.Warn(fmt.Sprintf("Failed to unmarshal application_data for app_data_id %v", appDataId), log.Error(err))
			continue
		}

		if value, exists := attrs[attrKey]; exists {
			// Transform the value
			newValue, err := transformValue(value, oldSchema, newSchema)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to transform value for app_data_id %v", appDataId), log.Error(err))
				continue
			}

			// Update the attribute
			attrs[attrKey] = newValue

			// Marshal back to JSON
			updatedJSON, err := json.Marshal(attrs)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to marshal updated application_data for app_data_id %v", appDataId), log.Error(err))
				continue
			}

			// Update the application data
			updateQuery := `
				UPDATE application_data 
				SET application_data = $1 
				WHERE app_data_id = $2
			`

			_, err = dbClient.ExecuteQuery(updateQuery, updatedJSON, appDataId)
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to update app_data_id %v", appDataId), log.Error(err))
				continue
			}

			profilesUpdated++
		}
	}

	logger.Info(fmt.Sprintf("Application data migration completed: updated %d records", profilesUpdated))
	return nil
}

// transformValue transforms a value from old schema format to new schema format
func transformValue(value interface{}, oldSchema, newSchema model.ProfileSchemaAttribute) (interface{}, error) {
	// Handle multi-valued changes first
	if !oldSchema.MultiValued && newSchema.MultiValued {
		// Single value -> Array
		return []interface{}{convertType(value, oldSchema.ValueType, newSchema.ValueType)}, nil
	} else if oldSchema.MultiValued && !newSchema.MultiValued {
		// Array -> Single value (take first element)
		if arr, ok := value.([]interface{}); ok && len(arr) > 0 {
			return convertType(arr[0], oldSchema.ValueType, newSchema.ValueType), nil
		}
		// Empty array or invalid, return default value based on new type
		return getDefaultValue(newSchema.ValueType), nil
	}

	// Handle type changes
	if oldSchema.MultiValued && newSchema.MultiValued {
		// Array of one type -> Array of another type
		if arr, ok := value.([]interface{}); ok {
			result := make([]interface{}, len(arr))
			for i, v := range arr {
				result[i] = convertType(v, oldSchema.ValueType, newSchema.ValueType)
			}
			return result, nil
		}
		return value, nil
	}

	// Single value type change
	return convertType(value, oldSchema.ValueType, newSchema.ValueType), nil
}

// convertType converts a value from one type to another
func convertType(value interface{}, fromType, toType string) interface{} {
	// If types are the same, no conversion needed
	if fromType == toType {
		return value
	}

	// Convert to string first (common intermediate format)
	strValue := fmt.Sprintf("%v", value)

	// Convert from string to target type
	switch toType {
	case constants.StringDataType:
		return strValue

	case constants.IntegerDataType:
		if i, err := strconv.ParseInt(strValue, 10, 64); err == nil {
			return float64(i) // JSON uses float64 for numbers
		}
		// Try parsing as float and converting to int
		if f, err := strconv.ParseFloat(strValue, 64); err == nil {
			return float64(int64(f))
		}
		return float64(0)

	case constants.DecimalDataType:
		if f, err := strconv.ParseFloat(strValue, 64); err == nil {
			return f
		}
		return 0.0

	case constants.BooleanDataType:
		if b, err := strconv.ParseBool(strValue); err == nil {
			return b
		}
		// Try some common boolean string representations
		switch strValue {
		case "1", "yes", "Yes", "YES", "y", "Y":
			return true
		case "0", "no", "No", "NO", "n", "N":
			return false
		}
		return false

	default:
		// For complex types or unknown types, return as string
		return strValue
	}
}

// getDefaultValue returns a default value for a given type
func getDefaultValue(valueType string) interface{} {
	switch valueType {
	case constants.StringDataType:
		return ""
	case constants.IntegerDataType:
		return float64(0)
	case constants.DecimalDataType:
		return 0.0
	case constants.BooleanDataType:
		return false
	default:
		return nil
	}
}

// ValidateSchemaUpdate validates if a schema update is allowed
// Returns whether migration is needed and any validation errors
func ValidateSchemaUpdate(orgId, attributeId string, updates map[string]interface{}) (bool, error) {
	// Get the current schema
	currentSchema, err := psstr.GetProfileSchemaAttributeById(orgId, attributeId)
	if err != nil {
		return false, err
	}

	// Check if value_type or multi_valued is being changed
	needsMigration := false

	if valueType, ok := updates["value_type"]; ok {
		if valueType != currentSchema.ValueType {
			needsMigration = true
		}
	}

	if multiValued, ok := updates["multi_valued"]; ok {
		if multiValued != currentSchema.MultiValued {
			needsMigration = true
		}
	}

	return needsMigration, nil
}
