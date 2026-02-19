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
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	_ = log.Init("ERROR")
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// keysOf
// ---------------------------------------------------------------------------

func TestKeysOf_EmptyMap(t *testing.T) {
	result := keysOf(map[string]bool{})
	assert.Empty(t, result)
}

func TestKeysOf_SingleKey(t *testing.T) {
	result := keysOf(map[string]bool{"alpha": true})
	assert.Equal(t, []string{"alpha"}, result)
}

func TestKeysOf_MultipleKeys(t *testing.T) {
	m := map[string]bool{"a": true, "b": false, "c": true}
	result := keysOf(m)
	assert.Len(t, result, 3)
	assert.ElementsMatch(t, []string{"a", "b", "c"}, result)
}

// ---------------------------------------------------------------------------
// parseFilter
// ---------------------------------------------------------------------------

func TestParseFilter_ValidInput(t *testing.T) {
	field, op, val, err := parseFilter("attribute_name eq identity_attributes.email")
	require.NoError(t, err)
	assert.Equal(t, "attribute_name", field)
	assert.Equal(t, "eq", op)
	assert.Equal(t, "identity_attributes.email", val)
}

func TestParseFilter_ValueWithSpaces(t *testing.T) {
	// SplitN with n=3 means the third part keeps any spaces in the value
	field, op, val, err := parseFilter("attribute_name eq some value with spaces")
	require.NoError(t, err)
	assert.Equal(t, "attribute_name", field)
	assert.Equal(t, "eq", op)
	assert.Equal(t, "some value with spaces", val)
}

func TestParseFilter_InvalidInput_TooFewParts(t *testing.T) {
	_, _, _, err := parseFilter("attribute_name eq")
	assert.Error(t, err, "filter with only two parts should error")
}

func TestParseFilter_InvalidInput_OnlyOneWord(t *testing.T) {
	_, _, _, err := parseFilter("justoneword")
	assert.Error(t, err)
}

func TestParseFilter_Empty(t *testing.T) {
	_, _, _, err := parseFilter("")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// matches
// ---------------------------------------------------------------------------

func makeAttr(name, appID string) model.ProfileSchemaAttribute {
	return model.ProfileSchemaAttribute{
		AttributeName:         name,
		ApplicationIdentifier: appID,
	}
}

func TestMatches_AttributeName_Eq_Match(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.True(t, matches(attr, "attribute_name", "eq", "identity_attributes.email"))
}

func TestMatches_AttributeName_Eq_NoMatch(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.False(t, matches(attr, "attribute_name", "eq", "identity_attributes.phone"))
}

func TestMatches_AttributeName_Contains_Match(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.True(t, matches(attr, "attribute_name", "contains", "email"))
}

func TestMatches_AttributeName_Contains_NoMatch(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.False(t, matches(attr, "attribute_name", "contains", "phone"))
}

func TestMatches_ApplicationIdentifier_Eq_Match(t *testing.T) {
	attr := makeAttr("application_data.pref", "myApp")
	assert.True(t, matches(attr, "application_identifier", "eq", "myApp"))
}

func TestMatches_ApplicationIdentifier_Eq_NoMatch(t *testing.T) {
	attr := makeAttr("application_data.pref", "myApp")
	assert.False(t, matches(attr, "application_identifier", "eq", "otherApp"))
}

func TestMatches_ApplicationIdentifier_Contains_Match(t *testing.T) {
	attr := makeAttr("application_data.pref", "myApplication")
	assert.True(t, matches(attr, "application_identifier", "contains", "App"))
}

func TestMatches_UnknownField_ReturnsFalse(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.False(t, matches(attr, "unknown_field", "eq", "anything"))
}

func TestMatches_UnknownOperator_ReturnsFalse(t *testing.T) {
	attr := makeAttr("identity_attributes.email", "")
	assert.False(t, matches(attr, "attribute_name", "unknown_op", "identity_attributes.email"))
}

// ---------------------------------------------------------------------------
// validateSchemaAttribute – pure validation (no DB calls in tested paths)
// ---------------------------------------------------------------------------

func TestValidateSchemaAttribute_MissingScope_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	restore := OverrideValidateApplicationIdentifierForTest(func(_, _ string) (error, bool) {
		return nil, true
	})
	defer restore()

	attr := model.ProfileSchemaAttribute{
		AttributeName: "noscope",
		ValueType:     "string",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_TooDeep_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "a.b.c.d.e.f", // 6 parts → depth > 5
		ValueType:     "string",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_InvalidScope_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "bad_scope.something",
		ValueType:     "string",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_ApplicationData_MissingAppIdentifier_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName:         "application_data.key",
		ValueType:             "string",
		MergeStrategy:         "combine",
		Mutability:            "readWrite",
		ApplicationIdentifier: "", // required for application_data
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_InvalidValueType_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	restore := OverrideValidateApplicationIdentifierForTest(func(_, _ string) (error, bool) {
		return nil, true
	})
	defer restore()

	attr := model.ProfileSchemaAttribute{
		AttributeName: "traits.field",
		ValueType:     "unsupported_type",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_InvalidMutability_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "traits.field",
		ValueType:     "string",
		MergeStrategy: "combine",
		Mutability:    "invalid_mutability",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_InvalidMergeStrategy_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "traits.field",
		ValueType:     "string",
		MergeStrategy: "invalid_strategy",
		Mutability:    "readWrite",
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_ComplexWithNoSubAttributes_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "traits.field",
		ValueType:     "complex",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
		SubAttributes: nil,
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}

func TestValidateSchemaAttribute_NonComplexWithSubAttributes_Error(t *testing.T) {
	svc := &ProfileSchemaService{}
	attr := model.ProfileSchemaAttribute{
		AttributeName: "traits.field",
		ValueType:     "string",
		MergeStrategy: "combine",
		Mutability:    "readWrite",
		SubAttributes: []model.SubAttribute{{AttributeId: "id1", AttributeName: "traits.field.sub"}},
	}
	err, valid := svc.validateSchemaAttribute(attr)
	assert.False(t, valid)
	assert.Error(t, err)
}
