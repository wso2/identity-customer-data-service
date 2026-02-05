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
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// serializeValue converts a typed value to its string representation for storage
func serializeValue(value interface{}, valueType string, multiValued bool) (interface{}, error) {
	logger := log.GetLogger()
	
	// If already stored as typed value, keep backward compatibility
	// This allows gradual migration of existing data
	if multiValued {
		arr, ok := value.([]interface{})
		if !ok {
			return value, nil // Keep as-is if not an array
		}
		
		serializedArr := make([]string, len(arr))
		for i, v := range arr {
			serialized, err := serializeValueSingle(v, valueType)
			if err != nil {
				logger.Debug("Failed to serialize array element", log.Error(err))
				return value, nil // Keep original on error
			}
			serializedArr[i] = serialized
		}
		return serializedArr, nil
	}
	
	// Single value
	serialized, err := serializeValueSingle(value, valueType)
	if err != nil {
		return value, nil // Keep original on error for backward compatibility
	}
	return serialized, nil
}

// serializeValueSingle converts a single typed value to string
func serializeValueSingle(value interface{}, valueType string) (string, error) {
	switch valueType {
	case constants.StringDataType:
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
		
	case constants.IntegerDataType:
		switch v := value.(type) {
		case int:
			return strconv.Itoa(v), nil
		case float64:
			return strconv.FormatInt(int64(v), 10), nil
		case string:
			return v, nil // Already serialized
		default:
			return fmt.Sprintf("%v", value), nil
		}
		
	case constants.DecimalDataType:
		switch v := value.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		case int:
			return strconv.FormatFloat(float64(v), 'f', -1, 64), nil
		case string:
			return v, nil // Already serialized
		default:
			return fmt.Sprintf("%v", value), nil
		}
		
	case constants.BooleanDataType:
		switch v := value.(type) {
		case bool:
			return strconv.FormatBool(v), nil
		case string:
			return v, nil // Already serialized
		default:
			return fmt.Sprintf("%v", value), nil
		}
		
	case constants.DateTimeDataType, constants.DateDataType, constants.EpochDataType:
		// Keep dates/times as strings
		return fmt.Sprintf("%v", value), nil
		
	case constants.ComplexDataType:
		// For complex types, serialize as JSON string
		if str, ok := value.(string); ok {
			return str, nil // Already serialized
		}
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
		
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

// deserializeValue converts a stored string value back to its typed representation
func deserializeValue(value interface{}, valueType string, multiValued bool) (interface{}, error) {
	logger := log.GetLogger()
	
	// Handle nil values
	if value == nil {
		return nil, nil
	}
	
	// If it's already typed (backward compatibility with existing data), return as-is
	if !multiValued {
		// Check if value is already in the expected type
		if isAlreadyTyped(value, valueType) {
			return value, nil
		}
	}
	
	if multiValued {
		// Handle multi-valued attributes
		switch arr := value.(type) {
		case []interface{}:
			// Could be array of strings (new format) or array of typed values (old format)
			if len(arr) == 0 {
				return arr, nil
			}
			
			// Check if first element is string (new format)
			if _, ok := arr[0].(string); ok {
				// Deserialize each string element
				deserializedArr := make([]interface{}, len(arr))
				for i, v := range arr {
					strVal, ok := v.(string)
					if !ok {
						// Mixed format or old format, return as-is
						return value, nil
					}
					deserialized, err := deserializeValueSingle(strVal, valueType)
					if err != nil {
						logger.Debug("Failed to deserialize array element", log.Error(err))
						return value, nil // Keep original on error
					}
					deserializedArr[i] = deserialized
				}
				return deserializedArr, nil
			}
			// Old format (already typed), return as-is
			return value, nil
			
		case []string:
			// New format: array of strings
			deserializedArr := make([]interface{}, len(arr))
			for i, strVal := range arr {
				deserialized, err := deserializeValueSingle(strVal, valueType)
				if err != nil {
					logger.Debug("Failed to deserialize array element", log.Error(err))
					return value, nil
				}
				deserializedArr[i] = deserialized
			}
			return deserializedArr, nil
			
		default:
			return value, nil // Keep as-is if not an array
		}
	}
	
	// Single value
	strVal, ok := value.(string)
	if !ok {
		// Not a string, might be old format (already typed)
		return value, nil
	}
	
	return deserializeValueSingle(strVal, valueType)
}

// deserializeValueSingle converts a single string value to its typed representation
func deserializeValueSingle(strVal string, valueType string) (interface{}, error) {
	switch valueType {
	case constants.StringDataType:
		return strVal, nil
		
	case constants.IntegerDataType:
		intVal, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return nil, err
		}
		return float64(intVal), nil // Return as float64 for JSON compatibility
		
	case constants.DecimalDataType:
		floatVal, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			return nil, err
		}
		return floatVal, nil
		
	case constants.BooleanDataType:
		boolVal, err := strconv.ParseBool(strVal)
		if err != nil {
			return nil, err
		}
		return boolVal, nil
		
	case constants.DateTimeDataType, constants.DateDataType, constants.EpochDataType:
		// Keep dates/times as strings
		return strVal, nil
		
	case constants.ComplexDataType:
		// For complex types, deserialize from JSON string
		var complexVal interface{}
		err := json.Unmarshal([]byte(strVal), &complexVal)
		if err != nil {
			return strVal, nil // Return as string if not valid JSON
		}
		return complexVal, nil
		
	default:
		return strVal, nil
	}
}

// isAlreadyTyped checks if a value is already in the expected type (backward compatibility)
func isAlreadyTyped(value interface{}, valueType string) bool {
	switch valueType {
	case constants.StringDataType:
		_, ok := value.(string)
		return ok
		
	case constants.IntegerDataType:
		switch v := value.(type) {
		case int, int64:
			return true
		case float64:
			return v == float64(int(v))
		default:
			return false
		}
		
	case constants.DecimalDataType:
		_, ok := value.(float64)
		return ok
		
	case constants.BooleanDataType:
		_, ok := value.(bool)
		return ok
		
	default:
		return false
	}
}

// transformProfileAttributes transforms profile attributes for storage (serialize to strings)
func transformProfileAttributesForStorage(attrs map[string]interface{}, schemaAttrs []model.ProfileSchemaAttribute) map[string]interface{} {
	if attrs == nil {
		return attrs
	}
	
	logger := log.GetLogger()
	transformed := make(map[string]interface{})
	
	// Create a map for quick lookup of schema attributes
	schemaMap := make(map[string]model.ProfileSchemaAttribute)
	for _, attr := range schemaAttrs {
		// Extract the attribute key from the full name (e.g., "identity_attributes.email" -> "email")
		parts := strings.SplitN(attr.AttributeName, ".", 2)
		if len(parts) == 2 {
			schemaMap[parts[1]] = attr
		}
	}
	
	for key, value := range attrs {
		if schema, found := schemaMap[key]; found {
			serialized, err := serializeValue(value, schema.ValueType, schema.MultiValued)
			if err != nil {
				logger.Debug(fmt.Sprintf("Failed to serialize attribute %s, keeping original", key), log.Error(err))
				transformed[key] = value
			} else {
				transformed[key] = serialized
			}
		} else {
			// No schema found, keep as-is
			transformed[key] = value
		}
	}
	
	return transformed
}

// transformProfileAttributesFromStorage transforms profile attributes after retrieval (deserialize from strings)
func transformProfileAttributesFromStorage(attrs map[string]interface{}, schemaAttrs []model.ProfileSchemaAttribute) map[string]interface{} {
	if attrs == nil {
		return attrs
	}
	
	logger := log.GetLogger()
	transformed := make(map[string]interface{})
	
	// Create a map for quick lookup of schema attributes
	schemaMap := make(map[string]model.ProfileSchemaAttribute)
	for _, attr := range schemaAttrs {
		// Extract the attribute key from the full name (e.g., "identity_attributes.email" -> "email")
		parts := strings.SplitN(attr.AttributeName, ".", 2)
		if len(parts) == 2 {
			schemaMap[parts[1]] = attr
		}
	}
	
	for key, value := range attrs {
		if schema, found := schemaMap[key]; found {
			deserialized, err := deserializeValue(value, schema.ValueType, schema.MultiValued)
			if err != nil {
				logger.Debug(fmt.Sprintf("Failed to deserialize attribute %s, keeping original", key), log.Error(err))
				transformed[key] = value
			} else {
				transformed[key] = deserialized
			}
		} else {
			// No schema found, keep as-is
			transformed[key] = value
		}
	}
	
	return transformed
}
