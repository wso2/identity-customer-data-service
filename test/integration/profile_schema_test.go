package integration

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"testing"
	"time"
)

func Test_ProfileSchemaService(t *testing.T) {

	SUPER_TENANT_ORG := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	profileSchemaService := schemaService.GetProfileSchemaService()

	// Scenario 1: Add schema attributes for all scopes
	t.Run("Add_ProfileSchemaAttributes_Success", func(t *testing.T) {
		attrs := []model.ProfileSchemaAttribute{
			{
				OrgId:                 SUPER_TENANT_ORG,
				AttributeId:           uuid.New().String(),
				AttributeName:         "identity_attributes.email",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
				MultiValued:           true,
			},
			{
				OrgId:                 SUPER_TENANT_ORG,
				AttributeId:           uuid.New().String(),
				AttributeName:         "identity_attributes.phone",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
				MultiValued:           true,
			},
			{
				OrgId:                 SUPER_TENANT_ORG,
				AttributeId:           uuid.New().String(),
				AttributeName:         "identity_attributes.userId",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "app_1",
			},
		}

		err := profileSchemaService.AddProfileSchemaAttributesForScope(attrs, constants.IdentityAttributes)
		require.NoError(t, err, "Failed to add schema attributes for valid scope")
	})

	// Scenario 2: Invalid attribute name scope
	t.Run("Add_ProfileSchemaAttributes_InvalidScope", func(t *testing.T) {
		attrs := []model.ProfileSchemaAttribute{
			{
				OrgId:                 SUPER_TENANT_ORG,
				AttributeId:           uuid.New().String(),
				AttributeName:         "invalidscope.email",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
			},
		}

		err := profileSchemaService.AddProfileSchemaAttributesForScope(attrs, "invalidscope")
		require.Error(t, err, "Expected validation failure for invalid scope")
	})

	// Scenario 3: Invalid when app identifier is not present
	t.Run("Add_ProfileSchemaAttributes_InvalidScope", func(t *testing.T) {
		attrs := []model.ProfileSchemaAttribute{
			{
				OrgId:         SUPER_TENANT_ORG,
				AttributeId:   uuid.New().String(),
				AttributeName: "application_data.email",
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
			},
		}

		err := profileSchemaService.AddProfileSchemaAttributesForScope(attrs, constants.ApplicationData)
		require.Error(t, err, "Expected validation failure for invalid scope")
	})

	// -------------------------------------------------------
	t.Run("Get_ProfileSchema_Success", func(t *testing.T) {
		schema, err := profileSchemaService.GetProfileSchema(SUPER_TENANT_ORG)
		require.NoError(t, err)
		require.NotNil(t, schema)
		require.Contains(t, schema, constants.IdentityAttributes)
		require.Contains(t, schema, constants.ApplicationData)
		require.Contains(t, schema, constants.Traits)
	})

	// Scenario 5: Get attribute by ID
	t.Run("Get_ProfileSchemaAttribute_ById", func(t *testing.T) {
		// Create a temp attribute first
		attr := model.ProfileSchemaAttribute{
			OrgId:                 SUPER_TENANT_ORG,
			AttributeId:           uuid.New().String(),
			AttributeName:         "identity_attributes.phone_number",
			ValueType:             constants.StringDataType,
			MergeStrategy:         "combine",
			Mutability:            constants.MutabilityReadWrite,
			ApplicationIdentifier: "",
		}
		err := profileSchemaService.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes)
		require.NoError(t, err)

		fetched, err := profileSchemaService.GetProfileSchemaAttributeById(SUPER_TENANT_ORG, attr.AttributeId)
		require.NoError(t, err)
		require.Equal(t, attr.AttributeName, fetched.AttributeName)
	})

	// Scenario 6: Get attributes by scope & filter
	t.Run("Get_ProfileSchemaAttributes_ByScopeAndFilter", func(t *testing.T) {
		filters := []string{"attribute_name eq identity_attributes.email"}
		filtered, err := profileSchemaService.GetProfileSchemaAttributesByScopeAndFilter(SUPER_TENANT_ORG, constants.IdentityAttributes, filters)
		require.NoError(t, err)
		require.NotNil(t, filtered)
	})

	// Scenario 7: Patch an existing attribute
	t.Run("Patch_ProfileSchemaAttribute_ById", func(t *testing.T) {
		attrId := uuid.New().String()
		attr := model.ProfileSchemaAttribute{
			OrgId:         SUPER_TENANT_ORG,
			AttributeId:   attrId,
			AttributeName: "identity_attributes.temp_field",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		}

		updates := map[string]interface{}{
			"attribute_name": "identity_attributes.temp_field",
			"attribute_id":   attrId,
			"value_type":     "integer",
			"merge_strategy": "latest",
			"mutability":     constants.MutabilityReadWrite,
		}
		_ = profileSchemaService.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes)

		err := profileSchemaService.PatchProfileSchemaAttributeById(SUPER_TENANT_ORG, attrId, updates)
		require.NoError(t, err, "Failed to patch profile schema attribute")
	})

	// Scenario 8: Delete schema attribute by ID
	t.Run("Delete_ProfileSchemaAttribute_ById", func(t *testing.T) {
		attr := model.ProfileSchemaAttribute{
			OrgId:         SUPER_TENANT_ORG,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.to_delete",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := profileSchemaService.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.Traits)
		require.NoError(t, err)

		err = profileSchemaService.DeleteProfileSchemaAttributeById(SUPER_TENANT_ORG, attr.AttributeId)
		require.NoError(t, err, "Expected no error when deleting schema attribute")
	})

	// Scenario 9: Delete all schema attributes under scope
	t.Run("Delete_ProfileSchemaAttributes_ByScope", func(t *testing.T) {
		err := profileSchemaService.DeleteProfileSchemaAttributesByScope(SUPER_TENANT_ORG, constants.IdentityAttributes)
		require.NoError(t, err, "Expected no error when deleting all schema attributes by scope")
	})

	// Cleanup
	t.Cleanup(func() {
		_ = profileSchemaService.DeleteProfileSchema(SUPER_TENANT_ORG)
		_ = profileSchemaService.DeleteProfileSchemaAttributesByScope(SUPER_TENANT_ORG, constants.IdentityAttributes)
	})

}
