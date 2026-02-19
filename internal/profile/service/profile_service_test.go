/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	_ = log.Init("ERROR")
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// ConvertAppData
// ---------------------------------------------------------------------------

func TestConvertAppData_EmptyInput(t *testing.T) {
	result := ConvertAppData(map[string]map[string]interface{}{})
	assert.Empty(t, result)
}

func TestConvertAppData_SingleApp(t *testing.T) {
	input := map[string]map[string]interface{}{
		"app1": {"color": "blue", "count": 3},
	}
	result := ConvertAppData(input)
	require.Len(t, result, 1)
	assert.Equal(t, "app1", result[0].AppId)
	assert.Equal(t, "blue", result[0].AppSpecificData["color"])
	assert.Equal(t, 3, result[0].AppSpecificData["count"])
}

func TestConvertAppData_MultipleApps(t *testing.T) {
	input := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
		"app2": {"k2": "v2"},
	}
	result := ConvertAppData(input)
	assert.Len(t, result, 2)
}

// ---------------------------------------------------------------------------
// validateMutability
// ---------------------------------------------------------------------------

func TestValidateMutability_ReadOnly_AlwaysError(t *testing.T) {
	err := validateMutability(constants.MutabilityReadOnly, false, nil, "val")
	assert.Error(t, err, "readOnly should always return error")

	err = validateMutability(constants.MutabilityReadOnly, true, "old", "new")
	assert.Error(t, err)
}

func TestValidateMutability_Immutable_CreateSuccess(t *testing.T) {
	err := validateMutability(constants.MutabilityImmutable, false, nil, "val")
	assert.NoError(t, err, "immutable on create should succeed")
}

func TestValidateMutability_Immutable_UpdateSameValue_Success(t *testing.T) {
	err := validateMutability(constants.MutabilityImmutable, true, "same", "same")
	assert.NoError(t, err, "immutable with same value on update should succeed")
}

func TestValidateMutability_Immutable_UpdateDifferentValue_Error(t *testing.T) {
	err := validateMutability(constants.MutabilityImmutable, true, "old", "new")
	assert.Error(t, err, "immutable field changed on update should error")
}

func TestValidateMutability_WriteOnce_CreateSuccess(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnce, false, nil, "val")
	assert.NoError(t, err)
}

func TestValidateMutability_WriteOnce_UpdateFromNil_Success(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnce, true, nil, "newVal")
	assert.NoError(t, err, "write-once: setting value when not yet set should succeed")
}

func TestValidateMutability_WriteOnce_UpdateFromEmpty_Success(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnce, true, "", "newVal")
	assert.NoError(t, err, "write-once: setting value when currently empty should succeed")
}

func TestValidateMutability_WriteOnce_UpdateAlreadySet_Error(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnce, true, "existingVal", "newVal")
	assert.Error(t, err, "write-once: changing a set value should error")
}

func TestValidateMutability_WriteOnce_UpdateSameValue_Success(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnce, true, "val", "val")
	assert.NoError(t, err, "write-once: keeping the same value should succeed")
}

func TestValidateMutability_ReadWrite_NoError(t *testing.T) {
	err := validateMutability(constants.MutabilityReadWrite, true, "old", "new")
	assert.NoError(t, err)
}

func TestValidateMutability_WriteOnly_NoError(t *testing.T) {
	err := validateMutability(constants.MutabilityWriteOnly, true, "old", "new")
	assert.NoError(t, err)
}

func TestValidateMutability_Unknown_Error(t *testing.T) {
	err := validateMutability("unknownMutability", false, nil, "val")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// isValidType
// ---------------------------------------------------------------------------

func TestIsValidType_String(t *testing.T) {
	assert.True(t, isValidType("hello", constants.StringDataType, false, nil))
	assert.False(t, isValidType(123, constants.StringDataType, false, nil))
}

func TestIsValidType_String_MultiValued(t *testing.T) {
	assert.True(t, isValidType([]interface{}{"a", "b"}, constants.StringDataType, true, nil))
	assert.False(t, isValidType([]interface{}{"a", 1}, constants.StringDataType, true, nil))
	assert.False(t, isValidType("not-array", constants.StringDataType, true, nil))
}

func TestIsValidType_Integer(t *testing.T) {
	assert.True(t, isValidType(float64(42), constants.IntegerDataType, false, nil))
	assert.True(t, isValidType(42, constants.IntegerDataType, false, nil))
	assert.False(t, isValidType(42.5, constants.IntegerDataType, false, nil))
	assert.False(t, isValidType("42", constants.IntegerDataType, false, nil))
}

func TestIsValidType_Integer_MultiValued(t *testing.T) {
	assert.True(t, isValidType([]interface{}{float64(1), float64(2)}, constants.IntegerDataType, true, nil))
	assert.False(t, isValidType([]interface{}{float64(1), 2.5}, constants.IntegerDataType, true, nil))
}

func TestIsValidType_Decimal(t *testing.T) {
	assert.True(t, isValidType(float64(3.14), constants.DecimalDataType, false, nil))
	assert.False(t, isValidType("3.14", constants.DecimalDataType, false, nil))
}

func TestIsValidType_Boolean(t *testing.T) {
	assert.True(t, isValidType(true, constants.BooleanDataType, false, nil))
	assert.True(t, isValidType(false, constants.BooleanDataType, false, nil))
	assert.False(t, isValidType("true", constants.BooleanDataType, false, nil))
}

func TestIsValidType_DateTime(t *testing.T) {
	assert.True(t, isValidType("2024-01-01T00:00:00Z", constants.DateTimeDataType, false, nil))
	assert.False(t, isValidType(12345, constants.DateTimeDataType, false, nil))
}

func TestIsValidType_Epoch(t *testing.T) {
	assert.True(t, isValidType("1700000000", constants.EpochDataType, false, nil))
	assert.False(t, isValidType(1700000000, constants.EpochDataType, false, nil))
}

func TestIsValidType_Date(t *testing.T) {
	assert.True(t, isValidType("2024-01-01", constants.DateDataType, false, nil))
	assert.False(t, isValidType(20240101, constants.DateDataType, false, nil))
}

func TestIsValidType_Complex(t *testing.T) {
	assert.True(t, isValidType(map[string]interface{}{"k": "v"}, constants.ComplexDataType, false, nil))
	assert.False(t, isValidType("not-a-map", constants.ComplexDataType, false, nil))
}

func TestIsValidType_Complex_MultiValued(t *testing.T) {
	assert.True(t, isValidType([]interface{}{
		map[string]interface{}{"k": "v"},
	}, constants.ComplexDataType, true, nil))
	assert.False(t, isValidType([]interface{}{"not-a-map"}, constants.ComplexDataType, true, nil))
}

func TestIsValidType_UnknownType(t *testing.T) {
	assert.False(t, isValidType("val", "unknownType", false, nil))
}

// ---------------------------------------------------------------------------
// isValidCanonicalValue
// ---------------------------------------------------------------------------

func TestIsValidCanonicalValue_NoRestriction(t *testing.T) {
	assert.True(t, isValidCanonicalValue("any", nil))
	assert.True(t, isValidCanonicalValue("any", []schemaModel.CanonicalValue{}))
}

func TestIsValidCanonicalValue_String_Valid(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "red"}, {Value: "blue"}}
	assert.True(t, isValidCanonicalValue("red", vals))
	assert.True(t, isValidCanonicalValue("blue", vals))
}

func TestIsValidCanonicalValue_String_Invalid(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "red"}, {Value: "blue"}}
	assert.False(t, isValidCanonicalValue("green", vals))
}

func TestIsValidCanonicalValue_Array_Valid(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "a"}, {Value: "b"}}
	assert.True(t, isValidCanonicalValue([]interface{}{"a", "b"}, vals))
}

func TestIsValidCanonicalValue_Array_Invalid(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "a"}, {Value: "b"}}
	assert.False(t, isValidCanonicalValue([]interface{}{"a", "c"}, vals))
}

func TestIsValidCanonicalValue_Array_NonStringItem(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "a"}}
	assert.False(t, isValidCanonicalValue([]interface{}{123}, vals))
}

func TestIsValidCanonicalValue_NonStringValue(t *testing.T) {
	vals := []schemaModel.CanonicalValue{{Value: "a"}}
	assert.False(t, isValidCanonicalValue(42, vals))
}

// ---------------------------------------------------------------------------
// ValidateProfileAgainstSchema
// ---------------------------------------------------------------------------

func makeSchemaAttr(name, vType, mut string) schemaModel.ProfileSchemaAttribute {
	return schemaModel.ProfileSchemaAttribute{
		AttributeName: name,
		ValueType:     vType,
		Mutability:    mut,
	}
}

func TestValidateProfileAgainstSchema_EmptyRequest_Success(t *testing.T) {
	schema := schemaModel.ProfileSchema{}
	req := profileModel.ProfileRequest{}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.NoError(t, err)
}

func TestValidateProfileAgainstSchema_ValidIdentityAttribute(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		IdentityAttributes: []schemaModel.ProfileSchemaAttribute{
			makeSchemaAttr("identity_attributes.email", constants.StringDataType, constants.MutabilityReadWrite),
		},
	}
	req := profileModel.ProfileRequest{
		IdentityAttributes: map[string]interface{}{"email": "test@example.com"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.NoError(t, err)
}

func TestValidateProfileAgainstSchema_UnknownIdentityAttribute_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{}
	req := profileModel.ProfileRequest{
		IdentityAttributes: map[string]interface{}{"unknown": "val"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_IdentityAttributeTypeMismatch_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		IdentityAttributes: []schemaModel.ProfileSchemaAttribute{
			makeSchemaAttr("identity_attributes.age", constants.IntegerDataType, constants.MutabilityReadWrite),
		},
	}
	req := profileModel.ProfileRequest{
		IdentityAttributes: map[string]interface{}{"age": "not-an-int"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_ValidTrait(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		Traits: []schemaModel.ProfileSchemaAttribute{
			makeSchemaAttr("traits.color", constants.StringDataType, constants.MutabilityReadWrite),
		},
	}
	req := profileModel.ProfileRequest{
		Traits: map[string]interface{}{"color": "blue"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.NoError(t, err)
}

func TestValidateProfileAgainstSchema_UnknownTrait_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{}
	req := profileModel.ProfileRequest{
		Traits: map[string]interface{}{"unknown": "val"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_TraitTypeMismatch_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		Traits: []schemaModel.ProfileSchemaAttribute{
			makeSchemaAttr("traits.score", constants.DecimalDataType, constants.MutabilityReadWrite),
		},
	}
	req := profileModel.ProfileRequest{
		Traits: map[string]interface{}{"score": "not-a-decimal"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_ValidAppData(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		ApplicationData: map[string][]schemaModel.ProfileSchemaAttribute{
			"myapp": {
				makeSchemaAttr("application_data.setting", constants.StringDataType, constants.MutabilityReadWrite),
			},
		},
	}
	req := profileModel.ProfileRequest{
		ApplicationData: map[string]map[string]interface{}{
			"myapp": {"setting": "value"},
		},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.NoError(t, err)
}

func TestValidateProfileAgainstSchema_UnknownAppData_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		ApplicationData: map[string][]schemaModel.ProfileSchemaAttribute{},
	}
	req := profileModel.ProfileRequest{
		ApplicationData: map[string]map[string]interface{}{
			"unknownApp": {"key": "val"},
		},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_ReadOnlyField_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		IdentityAttributes: []schemaModel.ProfileSchemaAttribute{
			makeSchemaAttr("identity_attributes.status", constants.StringDataType, constants.MutabilityReadOnly),
		},
	}
	req := profileModel.ProfileRequest{
		IdentityAttributes: map[string]interface{}{"status": "active"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}

func TestValidateProfileAgainstSchema_CanonicalValueViolation_Error(t *testing.T) {
	schema := schemaModel.ProfileSchema{
		Traits: []schemaModel.ProfileSchemaAttribute{
			{
				AttributeName:   "traits.status",
				ValueType:       constants.StringDataType,
				Mutability:      constants.MutabilityReadWrite,
				CanonicalValues: []schemaModel.CanonicalValue{{Value: "active"}, {Value: "inactive"}},
			},
		},
	}
	req := profileModel.ProfileRequest{
		Traits: map[string]interface{}{"status": "unknown-value"},
	}
	err := ValidateProfileAgainstSchema(req, profileModel.Profile{}, schema, false)
	assert.Error(t, err)
}
