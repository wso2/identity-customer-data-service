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

package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// CoerceValueToType attempts to convert a stored value to the target schema type.
// This enables schema evolution without requiring bulk profile updates.
//
// Design rationale:
// - Profile values are stored as generic interface{} in JSON format
// - The schema defines the authoritative type for each attribute
// - At read time, we coerce stored values to match the current schema
// - This allows type changes in the schema without rewriting all profiles
//
// Coercion rules:
// - string → integer: parse if numeric, return nil if not
// - string → decimal: parse if numeric, return nil if not
// - string → boolean: parse "true"/"false", return nil if not
// - integer → string: convert to string representation
// - decimal → string: convert to string representation
// - boolean → string: convert to "true"/"false"
// - incompatible conversions: return nil (attribute effectively absent)
//
// Parameters:
// - value: the stored value (can be any JSON-compatible type)
// - targetType: the expected type from the schema
// - multiValued: whether the attribute is an array
//
// Returns:
// - The coerced value, or nil if coercion is not possible
func CoerceValueToType(value interface{}, targetType string, multiValued bool) interface{} {
	logger := log.GetLogger()

	if value == nil {
		return nil
	}

	// Handle multi-valued (array) attributes
	if multiValued {
		return coerceArrayValue(value, targetType)
	}

	// Handle single-valued attributes
	return coerceSingleValue(value, targetType, logger)
}

// coerceArrayValue handles coercion for multi-valued attributes
func coerceArrayValue(value interface{}, targetType string) interface{} {
	logger := log.GetLogger()

	// If value is already an array, coerce each element
	if arr, ok := value.([]interface{}); ok {
		result := make([]interface{}, 0, len(arr))
		for _, item := range arr {
			coerced := coerceSingleValue(item, targetType, logger)
			if coerced != nil {
				result = append(result, coerced)
			}
		}
		// Return the array even if empty to preserve structure
		return result
	}

	// If value is not an array but should be, wrap it
	coerced := coerceSingleValue(value, targetType, logger)
	if coerced != nil {
		return []interface{}{coerced}
	}

	return []interface{}{}
}

// coerceSingleValue handles coercion for single values
func coerceSingleValue(value interface{}, targetType string, logger *log.Logger) interface{} {
	switch targetType {
	case constants.StringDataType:
		return coerceToString(value)

	case constants.IntegerDataType:
		return coerceToInteger(value, logger)

	case constants.DecimalDataType:
		return coerceToDecimal(value, logger)

	case constants.BooleanDataType:
		return coerceToBoolean(value, logger)

	case constants.DateTimeDataType:
		return coerceToDateTime(value)

	case constants.ComplexDataType:
		// Complex types should already be maps; no coercion needed
		if _, ok := value.(map[string]interface{}); ok {
			return value
		}
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce value to complex type: %v", value))
		}
		return nil

	default:
		if logger != nil {
			logger.Warn(fmt.Sprintf("Unknown target type for coercion: %s", targetType))
		}
		return value
	}
}

// coerceToString converts any value to string representation
func coerceToString(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return v
	case int, int64:
		return fmt.Sprintf("%d", v)
	case float64:
		// Check if it's an integer stored as float
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// coerceToInteger attempts to convert value to integer
func coerceToInteger(value interface{}, logger *log.Logger) interface{} {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		// JSON numbers are decoded as float64
		if v == float64(int(v)) {
			return int(v)
		}
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce decimal %v to integer without precision loss", v))
		}
		return nil
	case string:
		// Attempt to parse string as integer
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce string '%s' to integer", v))
		}
		return nil
	default:
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce type %T to integer", v))
		}
		return nil
	}
}

// coerceToDecimal attempts to convert value to decimal (float64)
func coerceToDecimal(value interface{}, logger *log.Logger) interface{} {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		// Attempt to parse string as float
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce string '%s' to decimal", v))
		}
		return nil
	default:
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce type %T to decimal", v))
		}
		return nil
	}
}

// coerceToBoolean attempts to convert value to boolean
func coerceToBoolean(value interface{}, logger *log.Logger) interface{} {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		// Parse common string representations
		lower := strings.ToLower(v)
		if lower == "true" || lower == "1" || lower == "yes" {
			return true
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false
		}
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce string '%s' to boolean", v))
		}
		return nil
	case int, int64:
		// 0 = false, non-zero = true
		return v != 0 && v != int64(0)
	case float64:
		// 0.0 = false, non-zero = true
		return v != 0.0
	default:
		if logger != nil {
			logger.Debug(fmt.Sprintf("Cannot coerce type %T to boolean", v))
		}
		return nil
	}
}

// coerceToDateTime validates and returns datetime strings
func coerceToDateTime(value interface{}) interface{} {
	// DateTime is stored as ISO 8601 string
	if str, ok := value.(string); ok {
		// Validate it's a reasonable datetime format
		// Support common formats: ISO 8601, RFC3339
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02",
		}
		for _, format := range formats {
			if _, err := time.Parse(format, str); err == nil {
				return str
			}
		}
	}
	// If not a valid datetime string, return as-is (will be validated elsewhere)
	return value
}

// ApplySchemaToAttributes applies the schema definition to a map of attribute values.
// This function coerces stored values to match their schema-defined types at read time.
//
// Design rationale:
// - Profiles store data in a schema-agnostic JSON format
// - The schema is the source of truth for data types and constraints
// - At read time, we ensure returned data matches the current schema
// - This enables schema evolution without bulk profile updates
//
// Parameters:
// - attributes: the raw attribute map from storage
// - schemaAttributes: the schema definitions for these attributes
//
// Returns:
// - A new map with coerced values matching the schema
func ApplySchemaToAttributes(attributes map[string]interface{}, schemaAttributes map[string]SchemaAttribute) map[string]interface{} {
	if len(attributes) == 0 {
		return attributes
	}

	result := make(map[string]interface{}, len(attributes))

	for key, value := range attributes {
		// Look up the schema for this attribute
		schema, found := schemaAttributes[key]
		if !found {
			// If no schema exists, attribute was removed from schema
			// We keep it in the profile for backward compatibility
			// but mark it as deprecated via logging
			logger := log.GetLogger()
			if logger != nil {
				logger.Debug(fmt.Sprintf("Attribute '%s' not in current schema; keeping raw value", key))
			}
			result[key] = value
			continue
		}

		// Apply type coercion based on schema
		coerced := CoerceValueToType(value, schema.ValueType, schema.MultiValued)
		if coerced != nil {
			result[key] = coerced
		}
		// If coercion returns nil, the value is omitted (attribute effectively absent)
	}

	return result
}

// SchemaAttribute represents the schema definition for an attribute
// Used by ApplySchemaToAttributes to coerce values at read time
type SchemaAttribute struct {
	ValueType   string
	MultiValued bool
}
