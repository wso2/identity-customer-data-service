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
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

func TestSerializeValue(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		valueType   string
		multiValued bool
		expected    interface{}
	}{
		{
			name:      "string value",
			value:     "test@example.com",
			valueType: constants.StringDataType,
			expected:  "test@example.com",
		},
		{
			name:      "integer as int",
			value:     42,
			valueType: constants.IntegerDataType,
			expected:  "42",
		},
		{
			name:      "integer as float64",
			value:     float64(42),
			valueType: constants.IntegerDataType,
			expected:  "42",
		},
		{
			name:      "decimal value",
			value:     3.14159,
			valueType: constants.DecimalDataType,
			expected:  "3.14159",
		},
		{
			name:      "boolean true",
			value:     true,
			valueType: constants.BooleanDataType,
			expected:  "true",
		},
		{
			name:      "boolean false",
			value:     false,
			valueType: constants.BooleanDataType,
			expected:  "false",
		},
		{
			name:        "multi-valued strings",
			value:       []interface{}{"apple", "banana", "cherry"},
			valueType:   constants.StringDataType,
			multiValued: true,
			expected:    []string{"apple", "banana", "cherry"},
		},
		{
			name:        "multi-valued integers",
			value:       []interface{}{float64(1), float64(2), float64(3)},
			valueType:   constants.IntegerDataType,
			multiValued: true,
			expected:    []string{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := serializeValue(tt.value, tt.valueType, tt.multiValued)
			if err != nil {
				t.Errorf("serializeValue() error = %v", err)
				return
			}
			
			// For string arrays, compare properly
			if tt.multiValued {
				resultArr, ok1 := result.([]string)
				expectedArr, ok2 := tt.expected.([]string)
				if !ok1 || !ok2 {
					t.Errorf("serializeValue() = %v, want %v", result, tt.expected)
					return
				}
				if len(resultArr) != len(expectedArr) {
					t.Errorf("serializeValue() length = %d, want %d", len(resultArr), len(expectedArr))
					return
				}
				for i := range resultArr {
					if resultArr[i] != expectedArr[i] {
						t.Errorf("serializeValue()[%d] = %v, want %v", i, resultArr[i], expectedArr[i])
					}
				}
			} else if result != tt.expected {
				t.Errorf("serializeValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDeserializeValue(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		valueType   string
		multiValued bool
		expected    interface{}
	}{
		{
			name:      "string value",
			value:     "test@example.com",
			valueType: constants.StringDataType,
			expected:  "test@example.com",
		},
		{
			name:      "integer from string",
			value:     "42",
			valueType: constants.IntegerDataType,
			expected:  float64(42),
		},
		{
			name:      "decimal from string",
			value:     "3.14159",
			valueType: constants.DecimalDataType,
			expected:  3.14159,
		},
		{
			name:      "boolean true from string",
			value:     "true",
			valueType: constants.BooleanDataType,
			expected:  true,
		},
		{
			name:      "boolean false from string",
			value:     "false",
			valueType: constants.BooleanDataType,
			expected:  false,
		},
		{
			name:        "multi-valued strings",
			value:       []string{"apple", "banana", "cherry"},
			valueType:   constants.StringDataType,
			multiValued: true,
			expected:    []interface{}{"apple", "banana", "cherry"},
		},
		{
			name:        "multi-valued integers",
			value:       []string{"1", "2", "3"},
			valueType:   constants.IntegerDataType,
			multiValued: true,
			expected:    []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name:      "already typed integer",
			value:     float64(42),
			valueType: constants.IntegerDataType,
			expected:  float64(42),
		},
		{
			name:      "already typed boolean",
			value:     true,
			valueType: constants.BooleanDataType,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := deserializeValue(tt.value, tt.valueType, tt.multiValued)
			if err != nil {
				t.Errorf("deserializeValue() error = %v", err)
				return
			}
			
			// For arrays, compare properly
			if tt.multiValued {
				resultArr, ok1 := result.([]interface{})
				expectedArr, ok2 := tt.expected.([]interface{})
				if !ok1 || !ok2 {
					t.Errorf("deserializeValue() = %v (type %T), want %v (type %T)", result, result, tt.expected, tt.expected)
					return
				}
				if len(resultArr) != len(expectedArr) {
					t.Errorf("deserializeValue() length = %d, want %d", len(resultArr), len(expectedArr))
					return
				}
				for i := range resultArr {
					if resultArr[i] != expectedArr[i] {
						t.Errorf("deserializeValue()[%d] = %v, want %v", i, resultArr[i], expectedArr[i])
					}
				}
			} else if result != tt.expected {
				t.Errorf("deserializeValue() = %v (type %T), want %v (type %T)", result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestTransformProfileAttributesForStorage(t *testing.T) {
	schemaAttrs := []model.ProfileSchemaAttribute{
		{
			AttributeName: "identity_attributes.email",
			ValueType:     constants.StringDataType,
			MultiValued:   false,
		},
		{
			AttributeName: "identity_attributes.age",
			ValueType:     constants.IntegerDataType,
			MultiValued:   false,
		},
		{
			AttributeName: "identity_attributes.subscribed",
			ValueType:     constants.BooleanDataType,
			MultiValued:   false,
		},
	}

	attrs := map[string]interface{}{
		"email":      "test@example.com",
		"age":        float64(25),
		"subscribed": true,
	}

	result := transformProfileAttributesForStorage(attrs, schemaAttrs)

	// Check email (string remains string)
	if result["email"] != "test@example.com" {
		t.Errorf("email = %v, want test@example.com", result["email"])
	}

	// Check age (number becomes string)
	if result["age"] != "25" {
		t.Errorf("age = %v, want '25'", result["age"])
	}

	// Check subscribed (boolean becomes string)
	if result["subscribed"] != "true" {
		t.Errorf("subscribed = %v, want 'true'", result["subscribed"])
	}
}

func TestTransformProfileAttributesFromStorage(t *testing.T) {
	schemaAttrs := []model.ProfileSchemaAttribute{
		{
			AttributeName: "identity_attributes.email",
			ValueType:     constants.StringDataType,
			MultiValued:   false,
		},
		{
			AttributeName: "identity_attributes.age",
			ValueType:     constants.IntegerDataType,
			MultiValued:   false,
		},
		{
			AttributeName: "identity_attributes.subscribed",
			ValueType:     constants.BooleanDataType,
			MultiValued:   false,
		},
	}

	attrs := map[string]interface{}{
		"email":      "test@example.com",
		"age":        "25",
		"subscribed": "true",
	}

	result := transformProfileAttributesFromStorage(attrs, schemaAttrs)

	// Check email (string remains string)
	if result["email"] != "test@example.com" {
		t.Errorf("email = %v, want test@example.com", result["email"])
	}

	// Check age (string becomes number)
	if result["age"] != float64(25) {
		t.Errorf("age = %v (type %T), want 25.0 (float64)", result["age"], result["age"])
	}

	// Check subscribed (string becomes boolean)
	if result["subscribed"] != true {
		t.Errorf("subscribed = %v, want true", result["subscribed"])
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that old format (already typed) data is still readable
	schemaAttrs := []model.ProfileSchemaAttribute{
		{
			AttributeName: "identity_attributes.age",
			ValueType:     constants.IntegerDataType,
			MultiValued:   false,
		},
		{
			AttributeName: "identity_attributes.subscribed",
			ValueType:     constants.BooleanDataType,
			MultiValued:   false,
		},
	}

	// Old format: already typed values
	attrs := map[string]interface{}{
		"age":        float64(25),
		"subscribed": true,
	}

	result := transformProfileAttributesFromStorage(attrs, schemaAttrs)

	// Should return as-is (already typed)
	if result["age"] != float64(25) {
		t.Errorf("age = %v, want 25", result["age"])
	}

	if result["subscribed"] != true {
		t.Errorf("subscribed = %v, want true", result["subscribed"])
	}
}
