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

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// TestSchemaEvolution_TypeChange demonstrates that profiles can adapt to schema changes
// without requiring bulk updates. When the schema type changes, the stored values are
// automatically coerced at read time.
func Test_SchemaAttribute_ValueType_Change(t *testing.T) {
	tenantId := fmt.Sprintf("test-tenant-%d", time.Now().UnixNano())
	schemaSvc := schemaService.GetProfileSchemaService()
	profileSvc := profileService.GetProfilesService()

	t.Run("String to Integer Evolution", func(t *testing.T) {
		// Step 1: Define initial schema where 'age' is a string
		ageAttrString := model.ProfileSchemaAttribute{
			OrgId:         tenantId,
			AttributeId:   uuid.New().String(),
			AttributeName: "identity_attributes.age",
			ValueType:     constants.StringDataType,
			MergeStrategy: "latest",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := schemaSvc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{ageAttrString}, constants.IdentityAttributes)
		require.NoError(t, err, "Failed to create initial schema")

		// Step 2: Create a profile with age as string
		profileReq := profileModel.ProfileRequest{
			UserId: "user-123",
			IdentityAttributes: map[string]interface{}{
				"age": "25", // Stored as string
			},
		}
		createdProfile, err := profileSvc.CreateProfile(profileReq, tenantId)
		require.NoError(t, err, "Failed to create profile")
		require.NotNil(t, createdProfile)

		// Verify age is stored as string
		age, exists := createdProfile.IdentityAttributes["age"]
		require.True(t, exists, "Age attribute should exist")
		assert.Equal(t, "25", age)

		// Step 3: Update schema - change age from string to integer
		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.age",
			"attribute_id":   ageAttrString.AttributeId,
			"value_type":     constants.IntegerDataType, // Changed to integer
			"merge_strategy": "latest",
			"mutability":     constants.MutabilityReadWrite,
		}
		err = schemaSvc.PatchProfileSchemaAttributeById(tenantId, ageAttrString.AttributeId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Step 4: Retrieve the profile - age should be automatically coerced to integer
		retrievedProfile, err := profileSvc.GetProfile(createdProfile.ProfileId)
		require.NoError(t, err, "Failed to retrieve profile")
		require.NotNil(t, retrievedProfile)

		// Verify age is now an integer (coerced from string "25" to int 25)
		ageValue, exists := retrievedProfile.IdentityAttributes["age"]
		require.True(t, exists, "Age attribute should exist after schema change")
		assert.IsType(t, 0, ageValue, "Age should be coerced to integer type")
		assert.Equal(t, 25, ageValue, "Age value should be 25")

		// Cleanup
		_ = profileSvc.DeleteProfile(createdProfile.ProfileId)
		_ = schemaSvc.DeleteProfileSchema(tenantId)
	})

	t.Run("Integer to String Evolution", func(t *testing.T) {
		// Step 1: Define initial schema where 'count' is an integer
		countAttrInt := model.ProfileSchemaAttribute{
			OrgId:         tenantId,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.count",
			ValueType:     constants.IntegerDataType,
			MergeStrategy: "latest",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := schemaSvc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{countAttrInt}, constants.Traits)
		require.NoError(t, err, "Failed to create initial schema")

		// Step 2: Create a profile with count as integer
		profileReq := profileModel.ProfileRequest{
			UserId: "user-456",
			Traits: map[string]interface{}{
				"count": 100, // Stored as integer
			},
		}
		createdProfile, err := profileSvc.CreateProfile(profileReq, tenantId)
		require.NoError(t, err, "Failed to create profile")
		require.NotNil(t, createdProfile)

		// Step 3: Update schema - change count from integer to string
		updates := map[string]interface{}{
			"attribute_name": "traits.count",
			"attribute_id":   countAttrInt.AttributeId,
			"value_type":     constants.StringDataType, // Changed to string
			"merge_strategy": "latest",
			"mutability":     constants.MutabilityReadWrite,
		}
		err = schemaSvc.PatchProfileSchemaAttributeById(tenantId, countAttrInt.AttributeId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Step 4: Retrieve the profile - count should be automatically coerced to string
		retrievedProfile, err := profileSvc.GetProfile(createdProfile.ProfileId)
		require.NoError(t, err, "Failed to retrieve profile")
		require.NotNil(t, retrievedProfile)

		// Verify count is now a string (coerced from int 100 to string "100")
		countValue, exists := retrievedProfile.Traits["count"]
		require.True(t, exists, "Count attribute should exist after schema change")
		assert.IsType(t, "", countValue, "Count should be coerced to string type")
		assert.Equal(t, "100", countValue, "Count value should be '100'")

		// Cleanup
		_ = profileSvc.DeleteProfile(createdProfile.ProfileId)
		_ = schemaSvc.DeleteProfileSchema(tenantId)
	})

	t.Run("String to Boolean Evolution", func(t *testing.T) {
		// Step 1: Define initial schema where 'active' is a string
		activeAttrString := model.ProfileSchemaAttribute{
			OrgId:         tenantId,
			AttributeId:   uuid.New().String(),
			AttributeName: "identity_attributes.active",
			ValueType:     constants.StringDataType,
			MergeStrategy: "latest",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := schemaSvc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{activeAttrString}, constants.IdentityAttributes)
		require.NoError(t, err, "Failed to create initial schema")

		// Step 2: Create a profile with active as string "true"
		profileReq := profileModel.ProfileRequest{
			UserId: "user-789",
			IdentityAttributes: map[string]interface{}{
				"active": "true", // Stored as string
			},
		}
		createdProfile, err := profileSvc.CreateProfile(profileReq, tenantId)
		require.NoError(t, err, "Failed to create profile")
		require.NotNil(t, createdProfile)

		// Step 3: Update schema - change active from string to boolean
		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.active",
			"attribute_id":   activeAttrString.AttributeId,
			"value_type":     constants.BooleanDataType, // Changed to boolean
			"merge_strategy": "latest",
			"mutability":     constants.MutabilityReadWrite,
		}
		err = schemaSvc.PatchProfileSchemaAttributeById(tenantId, activeAttrString.AttributeId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Step 4: Retrieve the profile - active should be automatically coerced to boolean
		retrievedProfile, err := profileSvc.GetProfile(createdProfile.ProfileId)
		require.NoError(t, err, "Failed to retrieve profile")
		require.NotNil(t, retrievedProfile)

		// Verify active is now a boolean (coerced from string "true" to bool true)
		activeValue, exists := retrievedProfile.IdentityAttributes["active"]
		require.True(t, exists, "Active attribute should exist after schema change")
		assert.IsType(t, true, activeValue, "Active should be coerced to boolean type")
		assert.Equal(t, true, activeValue, "Active value should be true")

		// Cleanup
		_ = profileSvc.DeleteProfile(createdProfile.ProfileId)
		_ = schemaSvc.DeleteProfileSchema(tenantId)
	})
}

// TestSchemaEvolution_AttributeRemoval demonstrates that when attributes are removed
// from the schema, existing profile data is preserved for backward compatibility.
func TestSchemaEvolution_AttributeRemoval(t *testing.T) {
	tenantId := fmt.Sprintf("test-tenant-%d", time.Now().UnixNano())
	schemaSvc := schemaService.GetProfileSchemaService()
	profileSvc := profileService.GetProfilesService()

	// Step 1: Define initial schema with two attributes
	attr1 := model.ProfileSchemaAttribute{
		OrgId:         tenantId,
		AttributeId:   uuid.New().String(),
		AttributeName: "identity_attributes.email",
		ValueType:     constants.StringDataType,
		MergeStrategy: "latest",
		Mutability:    constants.MutabilityReadWrite,
	}
	attr2 := model.ProfileSchemaAttribute{
		OrgId:         tenantId,
		AttributeId:   uuid.New().String(),
		AttributeName: "identity_attributes.phone",
		ValueType:     constants.StringDataType,
		MergeStrategy: "latest",
		Mutability:    constants.MutabilityReadWrite,
	}
	err := schemaSvc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr1, attr2}, constants.IdentityAttributes)
	require.NoError(t, err, "Failed to create initial schema")

	// Step 2: Create a profile with both attributes
	profileReq := profileModel.ProfileRequest{
		UserId: "user-removal-test",
		IdentityAttributes: map[string]interface{}{
			"email": "test@example.com",
			"phone": "+1234567890",
		},
	}
	createdProfile, err := profileSvc.CreateProfile(profileReq, tenantId)
	require.NoError(t, err, "Failed to create profile")
	require.NotNil(t, createdProfile)

	// Step 3: Remove 'phone' from the schema
	err = schemaSvc.DeleteProfileSchemaAttributeById(tenantId, attr2.AttributeId)
	require.NoError(t, err, "Failed to remove attribute from schema")

	// Step 4: Retrieve the profile - both attributes should still be present
	retrievedProfile, err := profileSvc.GetProfile(createdProfile.ProfileId)
	require.NoError(t, err, "Failed to retrieve profile")
	require.NotNil(t, retrievedProfile)

	// Verify both attributes are still present (backward compatibility)
	email, emailExists := retrievedProfile.IdentityAttributes["email"]
	phone, phoneExists := retrievedProfile.IdentityAttributes["phone"]

	assert.True(t, emailExists, "Email attribute should exist")
	assert.Equal(t, "test@example.com", email)

	assert.True(t, phoneExists, "Phone attribute should exist even after removal from schema")
	assert.Equal(t, "+1234567890", phone)

	// Cleanup
	_ = profileSvc.DeleteProfile(createdProfile.ProfileId)
	_ = schemaSvc.DeleteProfileSchema(tenantId)
}

// TestSchemaEvolution_MultiValuedTypeChange demonstrates that multi-valued attributes
// can also adapt to schema changes with proper type coercion.
func TestSchemaEvolution_MultiValuedTypeChange(t *testing.T) {
	tenantId := fmt.Sprintf("test-tenant-%d", time.Now().UnixNano())
	schemaSvc := schemaService.GetProfileSchemaService()
	profileSvc := profileService.GetProfilesService()

	// Step 1: Define initial schema where 'tags' is multi-valued string
	tagsAttrString := model.ProfileSchemaAttribute{
		OrgId:         tenantId,
		AttributeId:   uuid.New().String(),
		AttributeName: "traits.tags",
		ValueType:     constants.StringDataType,
		MergeStrategy: "combine",
		Mutability:    constants.MutabilityReadWrite,
		MultiValued:   true,
	}
	err := schemaSvc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{tagsAttrString}, constants.Traits)
	require.NoError(t, err, "Failed to create initial schema")

	// Step 2: Create a profile with tags as string array ["1", "2", "3"]
	profileReq := profileModel.ProfileRequest{
		UserId: "user-multivalue",
		Traits: map[string]interface{}{
			"tags": []interface{}{"1", "2", "3"}, // Stored as strings
		},
	}
	createdProfile, err := profileSvc.CreateProfile(profileReq, tenantId)
	require.NoError(t, err, "Failed to create profile")
	require.NotNil(t, createdProfile)

	// Step 3: Update schema - change tags from string to integer (still multi-valued)
	updates := map[string]interface{}{
		"attribute_name": "traits.tags",
		"attribute_id":   tagsAttrString.AttributeId,
		"value_type":     constants.IntegerDataType, // Changed to integer
		"merge_strategy": "combine",
		"mutability":     constants.MutabilityReadWrite,
		"multi_valued":   true,
	}
	err = schemaSvc.PatchProfileSchemaAttributeById(tenantId, tagsAttrString.AttributeId, updates)
	require.NoError(t, err, "Failed to update schema")

	// Step 4: Retrieve the profile - tags should be coerced to integer array
	retrievedProfile, err := profileSvc.GetProfile(createdProfile.ProfileId)
	require.NoError(t, err, "Failed to retrieve profile")
	require.NotNil(t, retrievedProfile)

	// Verify tags are now integers
	tagsValue, exists := retrievedProfile.Traits["tags"]
	require.True(t, exists, "Tags attribute should exist after schema change")

	tagsArray, ok := tagsValue.([]interface{})
	require.True(t, ok, "Tags should be an array")
	require.Len(t, tagsArray, 3, "Tags array should have 3 elements")

	// Verify each element is now an integer
	assert.Equal(t, 1, tagsArray[0], "First tag should be integer 1")
	assert.Equal(t, 2, tagsArray[1], "Second tag should be integer 2")
	assert.Equal(t, 3, tagsArray[2], "Third tag should be integer 3")

	// Cleanup
	_ = profileSvc.DeleteProfile(createdProfile.ProfileId)
	_ = schemaSvc.DeleteProfileSchema(tenantId)
}
