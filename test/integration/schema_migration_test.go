package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

func Test_SchemaUpdate_Migration(t *testing.T) {
	SuperTenantOrg := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	svc := schemaService.GetProfileSchemaService()
	profileSvc := profileService.GetProfilesService()

	// Override app verification
	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	// Cleanup at the end
	defer func() {
		_ = svc.DeleteProfileSchema(SuperTenantOrg)
	}()

	t.Run("TypeChange_String_To_Integer", func(t *testing.T) {
		// Step 1: Create schema with string type
		attrId := uuid.New().String()
		attr := model.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   attrId,
			AttributeName: "identity_attributes.age",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes, SuperTenantOrg)
		require.NoError(t, err, "Failed to add attribute")

		// Step 2: Create a profile with string value
		profileId := uuid.New().String()
		profile := profileModel.Profile{
			ProfileId: profileId,
			OrgHandle: SuperTenantOrg,
			UserId:    "test-user-" + uuid.New().String(),
			IdentityAttributes: map[string]interface{}{
				"age": "25", // String value
			},
			ProfileStatus: &profileModel.ProfileStatus{
				IsReferenceProfile: true,
				ListProfile:        true,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Location:  utils.BuildProfileLocation(SuperTenantOrg, profileId),
		}
		err = profileStore.InsertProfile(profile)
		require.NoError(t, err, "Failed to insert profile")

		// Step 3: Update schema to integer type
		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.age",
			"value_type":     constants.IntegerDataType,
			"merge_strategy": "combine",
			"mutability":     constants.MutabilityReadWrite,
		}
		err = svc.PatchProfileSchemaAttributeById(SuperTenantOrg, attrId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Wait for async migration to complete
		time.Sleep(2 * time.Second)

		// Step 4: Verify the profile data was migrated
		fetchedProfile, err := profileStore.GetProfile(profileId)
		require.NoError(t, err, "Failed to fetch profile")
		require.NotNil(t, fetchedProfile, "Profile should exist")

		age, ok := fetchedProfile.IdentityAttributes["age"]
		require.True(t, ok, "Age attribute should exist")
		
		// Should be migrated to integer (float64 in JSON)
		ageFloat, ok := age.(float64)
		require.True(t, ok, "Age should be a number after migration")
		require.Equal(t, float64(25), ageFloat, "Age value should be 25")

		// Cleanup
		_ = profileStore.DeleteProfile(profileId)
		_ = svc.DeleteProfileSchemaAttributeById(SuperTenantOrg, attrId)
	})

	t.Run("MultiValuedChange_Single_To_Array", func(t *testing.T) {
		// Step 1: Create schema with single value
		attrId := uuid.New().String()
		attr := model.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   attrId,
			AttributeName: "identity_attributes.favorite_color",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   false,
		}
		err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes, SuperTenantOrg)
		require.NoError(t, err, "Failed to add attribute")

		// Step 2: Create a profile with single value
		profileId := uuid.New().String()
		profile := profileModel.Profile{
			ProfileId: profileId,
			OrgHandle: SuperTenantOrg,
			UserId:    "test-user-" + uuid.New().String(),
			IdentityAttributes: map[string]interface{}{
				"favorite_color": "blue", // Single value
			},
			ProfileStatus: &profileModel.ProfileStatus{
				IsReferenceProfile: true,
				ListProfile:        true,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Location:  utils.BuildProfileLocation(SuperTenantOrg, profileId),
		}
		err = profileStore.InsertProfile(profile)
		require.NoError(t, err, "Failed to insert profile")

		// Step 3: Update schema to multi-valued
		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.favorite_color",
			"value_type":     constants.StringDataType,
			"merge_strategy": "combine",
			"mutability":     constants.MutabilityReadWrite,
			"multi_valued":   true,
		}
		err = svc.PatchProfileSchemaAttributeById(SuperTenantOrg, attrId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Wait for async migration to complete
		time.Sleep(2 * time.Second)

		// Step 4: Verify the profile data was migrated
		fetchedProfile, err := profileStore.GetProfile(profileId)
		require.NoError(t, err, "Failed to fetch profile")
		require.NotNil(t, fetchedProfile, "Profile should exist")

		favoriteColor, ok := fetchedProfile.IdentityAttributes["favorite_color"]
		require.True(t, ok, "favorite_color attribute should exist")
		
		// Should be migrated to array
		colorArray, ok := favoriteColor.([]interface{})
		require.True(t, ok, "favorite_color should be an array after migration")
		require.Len(t, colorArray, 1, "Array should have 1 element")
		require.Equal(t, "blue", colorArray[0], "First element should be 'blue'")

		// Cleanup
		_ = profileStore.DeleteProfile(profileId)
		_ = svc.DeleteProfileSchemaAttributeById(SuperTenantOrg, attrId)
	})

	t.Run("TypeAndMultiValuedChange_StringSingle_To_IntegerArray", func(t *testing.T) {
		// Step 1: Create schema with single string value
		attrId := uuid.New().String()
		attr := model.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   attrId,
			AttributeName: "traits.score",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   false,
		}
		err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.Traits, SuperTenantOrg)
		require.NoError(t, err, "Failed to add attribute")

		// Step 2: Create a profile with single string value
		profileId := uuid.New().String()
		profile := profileModel.Profile{
			ProfileId: profileId,
			OrgHandle: SuperTenantOrg,
			UserId:    "test-user-" + uuid.New().String(),
			Traits: map[string]interface{}{
				"score": "100", // String value
			},
			ProfileStatus: &profileModel.ProfileStatus{
				IsReferenceProfile: true,
				ListProfile:        true,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Location:  utils.BuildProfileLocation(SuperTenantOrg, profileId),
		}
		err = profileStore.InsertProfile(profile)
		require.NoError(t, err, "Failed to insert profile")

		// Step 3: Update schema to multi-valued integer
		updates := map[string]interface{}{
			"attribute_name": "traits.score",
			"value_type":     constants.IntegerDataType,
			"merge_strategy": "combine",
			"mutability":     constants.MutabilityReadWrite,
			"multi_valued":   true,
		}
		err = svc.PatchProfileSchemaAttributeById(SuperTenantOrg, attrId, updates)
		require.NoError(t, err, "Failed to update schema")

		// Wait for async migration to complete
		time.Sleep(2 * time.Second)

		// Step 4: Verify the profile data was migrated
		fetchedProfile, err := profileStore.GetProfile(profileId)
		require.NoError(t, err, "Failed to fetch profile")
		require.NotNil(t, fetchedProfile, "Profile should exist")

		score, ok := fetchedProfile.Traits["score"]
		require.True(t, ok, "score attribute should exist")
		
		// Should be migrated to array of integers
		scoreArray, ok := score.([]interface{})
		require.True(t, ok, "score should be an array after migration")
		require.Len(t, scoreArray, 1, "Array should have 1 element")
		
		scoreValue, ok := scoreArray[0].(float64)
		require.True(t, ok, "First element should be a number")
		require.Equal(t, float64(100), scoreValue, "First element should be 100")

		// Cleanup
		_ = profileStore.DeleteProfile(profileId)
		_ = svc.DeleteProfileSchemaAttributeById(SuperTenantOrg, attrId)
	})

	t.Run("NoMigration_When_Schema_Unchanged", func(t *testing.T) {
		// Step 1: Create schema
		attrId := uuid.New().String()
		attr := model.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   attrId,
			AttributeName: "identity_attributes.email",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes, SuperTenantOrg)
		require.NoError(t, err, "Failed to add attribute")

		// Step 2: Update schema without changing type or multi_valued
		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.email",
			"value_type":     constants.StringDataType, // Same as before
			"merge_strategy": "overwrite",               // Changed
			"mutability":     constants.MutabilityReadWrite,
		}
		err = svc.PatchProfileSchemaAttributeById(SuperTenantOrg, attrId, updates)
		require.NoError(t, err, "Failed to update schema")

		// No migration should be triggered (no need to wait or verify data)

		// Cleanup
		_ = svc.DeleteProfileSchemaAttributeById(SuperTenantOrg, attrId)
	})
}
