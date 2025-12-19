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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

func TestCoerceValueToType_String(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"string to string", "hello", "hello"},
		{"int to string", 42, "42"},
		{"float to string (integer)", 42.0, "42"},
		{"float to string (decimal)", 42.5, "42.5"},
		{"bool true to string", true, "true"},
		{"bool false to string", false, "false"},
		{"nil to nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.StringDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_Integer(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"int to int", 42, 42},
		{"float (integer) to int", 42.0, 42},
		{"string int to int", "42", 42},
		{"string non-int to nil", "hello", nil},
		{"float (decimal) to nil", 42.5, nil},
		{"bool to nil", true, nil},
		{"nil to nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.IntegerDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_Decimal(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"float to float", 42.5, 42.5},
		{"int to float", 42, 42.0},
		{"string decimal to float", "42.5", 42.5},
		{"string int to float", "42", 42.0},
		{"string non-numeric to nil", "hello", nil},
		{"bool to nil", true, nil},
		{"nil to nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.DecimalDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_Boolean(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"bool true to bool", true, true},
		{"bool false to bool", false, false},
		{"string 'true' to bool", "true", true},
		{"string 'false' to bool", "false", false},
		{"string '1' to bool", "1", true},
		{"string '0' to bool", "0", false},
		{"string 'yes' to bool", "yes", true},
		{"string 'no' to bool", "no", false},
		{"int 1 to bool", 1, true},
		{"int 0 to bool", 0, false},
		{"float 1.0 to bool", 1.0, true},
		{"float 0.0 to bool", 0.0, false},
		{"string invalid to nil", "hello", nil},
		{"nil to nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.BooleanDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_DateTime(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"ISO 8601 string", "2023-01-15T10:30:00Z", "2023-01-15T10:30:00Z"},
		{"RFC3339 string", "2023-01-15T10:30:00+05:30", "2023-01-15T10:30:00+05:30"},
		{"Date only", "2023-01-15", "2023-01-15"},
		{"Invalid datetime", "not-a-date", "not-a-date"}, // Returns as-is for validation elsewhere
		{"non-string input", 12345, 12345},               // Returns as-is
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.DateTimeDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_Complex(t *testing.T) {
	complexObj := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"map to map", complexObj, complexObj},
		{"non-map to nil", "string", nil},
		{"nil to nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceValueToType(tt.input, constants.ComplexDataType, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCoerceValueToType_MultiValued(t *testing.T) {
	t.Run("array of strings", func(t *testing.T) {
		input := []interface{}{"a", "b", "c"}
		result := CoerceValueToType(input, constants.StringDataType, true)
		expected := []interface{}{"a", "b", "c"}
		assert.Equal(t, expected, result)
	})

	t.Run("array of ints converted to strings", func(t *testing.T) {
		input := []interface{}{1, 2, 3}
		result := CoerceValueToType(input, constants.StringDataType, true)
		expected := []interface{}{"1", "2", "3"}
		assert.Equal(t, expected, result)
	})

	t.Run("array of strings to ints", func(t *testing.T) {
		input := []interface{}{"1", "2", "3"}
		result := CoerceValueToType(input, constants.IntegerDataType, true)
		expected := []interface{}{1, 2, 3}
		assert.Equal(t, expected, result)
	})

	t.Run("array with invalid conversions filters out", func(t *testing.T) {
		input := []interface{}{"1", "invalid", "3"}
		result := CoerceValueToType(input, constants.IntegerDataType, true)
		expected := []interface{}{1, 3}
		assert.Equal(t, expected, result)
	})

	t.Run("single value wrapped in array", func(t *testing.T) {
		input := "single"
		result := CoerceValueToType(input, constants.StringDataType, true)
		expected := []interface{}{"single"}
		assert.Equal(t, expected, result)
	})

	t.Run("nil returns nil", func(t *testing.T) {
		result := CoerceValueToType(nil, constants.StringDataType, true)
		assert.Nil(t, result)
	})
}

func TestApplySchemaToAttributes(t *testing.T) {
	t.Run("coerce attributes based on schema", func(t *testing.T) {
		// Simulate a profile where age was stored as string but schema now expects int
		attributes := map[string]interface{}{
			"age":      "25",
			"height":   "5.9",
			"active":   "true",
			"name":     "John",
			"tags":     []interface{}{"1", "2", "3"},
			"metadata": map[string]interface{}{"key": "value"},
		}

		schema := map[string]SchemaAttribute{
			"age":      {ValueType: constants.IntegerDataType, MultiValued: false},
			"height":   {ValueType: constants.DecimalDataType, MultiValued: false},
			"active":   {ValueType: constants.BooleanDataType, MultiValued: false},
			"name":     {ValueType: constants.StringDataType, MultiValued: false},
			"tags":     {ValueType: constants.IntegerDataType, MultiValued: true},
			"metadata": {ValueType: constants.ComplexDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(attributes, schema)

		assert.Equal(t, 25, result["age"])
		assert.Equal(t, 5.9, result["height"])
		assert.Equal(t, true, result["active"])
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, []interface{}{1, 2, 3}, result["tags"])
		assert.Equal(t, map[string]interface{}{"key": "value"}, result["metadata"])
	})

	t.Run("handle attributes not in schema", func(t *testing.T) {
		attributes := map[string]interface{}{
			"name":    "John",
			"removed": "old_value",
		}

		schema := map[string]SchemaAttribute{
			"name": {ValueType: constants.StringDataType, MultiValued: false},
			// "removed" is not in schema anymore
		}

		result := ApplySchemaToAttributes(attributes, schema)

		// Attribute should be kept for backward compatibility
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, "old_value", result["removed"])
	})

	t.Run("handle incompatible conversions", func(t *testing.T) {
		attributes := map[string]interface{}{
			"age": "not_a_number",
		}

		schema := map[string]SchemaAttribute{
			"age": {ValueType: constants.IntegerDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(attributes, schema)

		// Incompatible conversion results in attribute being omitted
		_, exists := result["age"]
		assert.False(t, exists)
	})

	t.Run("handle nil and empty inputs", func(t *testing.T) {
		assert.Nil(t, ApplySchemaToAttributes(nil, nil))
		assert.Equal(t, map[string]interface{}{}, ApplySchemaToAttributes(map[string]interface{}{}, nil))
	})
}

func TestSchemaEvolution_TypeChange(t *testing.T) {
	// Simulate schema evolution: age changed from string to integer
	t.Run("age string to integer evolution", func(t *testing.T) {
		// Old profile stored age as string
		oldProfile := map[string]interface{}{
			"age": "25",
		}

		// New schema expects integer
		newSchema := map[string]SchemaAttribute{
			"age": {ValueType: constants.IntegerDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(oldProfile, newSchema)

		// Should successfully coerce
		assert.Equal(t, 25, result["age"])
	})

	t.Run("isActive string to boolean evolution", func(t *testing.T) {
		// Old profile stored boolean as string
		oldProfile := map[string]interface{}{
			"isActive": "true",
		}

		// New schema expects boolean
		newSchema := map[string]SchemaAttribute{
			"isActive": {ValueType: constants.BooleanDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(oldProfile, newSchema)

		assert.Equal(t, true, result["isActive"])
	})

	t.Run("price integer to decimal evolution", func(t *testing.T) {
		// Old profile stored price as integer
		oldProfile := map[string]interface{}{
			"price": 100,
		}

		// New schema expects decimal for precision
		newSchema := map[string]SchemaAttribute{
			"price": {ValueType: constants.DecimalDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(oldProfile, newSchema)

		assert.Equal(t, 100.0, result["price"])
	})
}

func TestSchemaEvolution_AttributeRemoval(t *testing.T) {
	t.Run("removed attribute kept for backward compatibility", func(t *testing.T) {
		// Profile has an attribute that's no longer in schema
		profile := map[string]interface{}{
			"name":       "John",
			"deprecated": "old_value",
		}

		schema := map[string]SchemaAttribute{
			"name": {ValueType: constants.StringDataType, MultiValued: false},
			// "deprecated" removed from schema
		}

		result := ApplySchemaToAttributes(profile, schema)

		// Both should be present
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, "old_value", result["deprecated"])
	})
}

func TestSchemaEvolution_NewAttribute(t *testing.T) {
	t.Run("new schema attribute not in profile", func(t *testing.T) {
		// Profile doesn't have new attribute yet
		profile := map[string]interface{}{
			"name": "John",
		}

		schema := map[string]SchemaAttribute{
			"name":     {ValueType: constants.StringDataType, MultiValued: false},
			"newField": {ValueType: constants.StringDataType, MultiValued: false},
		}

		result := ApplySchemaToAttributes(profile, schema)

		// Only existing attribute should be present
		assert.Equal(t, "John", result["name"])
		_, hasNewField := result["newField"]
		assert.False(t, hasNewField)
	})
}
