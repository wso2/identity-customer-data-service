package integration

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"testing"
)

func TestProfileSchemaService(t *testing.T) {
	profileSchemaService := schemaService.GetProfileSchemaService()

	t.Run("AddProfileSchemaAttributesForScope_Success", func(t *testing.T) {
		// Prepare valid attributes to add
		attrs := []model.ProfileSchemaAttribute{
			{
				AttributeName:         "identity_attributes.email",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
			},
		}

		// Call the function
		err := profileSchemaService.AddProfileSchemaAttributesForScope(attrs, constants.IdentityAttributes)
		require.NoError(t, err, "Expected no error when adding valid schema attributes")
	})

	t.Run("AddProfileSchemaAttributesForScope_InvalidScope", func(t *testing.T) {
		// Prepare invalid scope attributes
		attrs := []model.ProfileSchemaAttribute{
			{
				AttributeName:         "traits.email", // Incorrect scope
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
			},
		}

		// Call the function
		err := profileSchemaService.AddProfileSchemaAttributesForScope(attrs, constants.IdentityAttributes)
		require.Error(t, err, "Expected error due to invalid scope")
	})

	t.Run("GetProfileSchema_Success", func(t *testing.T) {
		// Assuming orgId "org-123" exists
		orgId := "org-123"

		// Mocking GetProfileSchema
		schema, err := profileSchemaService.GetProfileSchema(orgId)
		require.NoError(t, err, "Expected no error when fetching profile schema")
		require.NotNil(t, schema, "Expected profile schema to be non-nil")
	})

	t.Run("DeleteProfileSchemaAttributeById_Success", func(t *testing.T) {
		// Assuming orgId and attributeId are valid
		orgId := "org-123"
		attributeId := uuid.New().String()

		// Mocking store deletion
		err := profileSchemaService.DeleteProfileSchemaAttributeById(orgId, attributeId)
		require.NoError(t, err, "Expected no error when deleting profile schema attribute")
	})

	t.Run("PatchProfileSchemaAttributeById_Success", func(t *testing.T) {
		// Prepare patch data
		updates := map[string]interface{}{
			"value_type":     "string",
			"merge_strategy": "combine",
		}

		// Mocking patch operation
		orgId := "org-123"
		attributeId := uuid.New().String()

		err := profileSchemaService.PatchProfileSchemaAttributeById(orgId, attributeId, updates)
		require.NoError(t, err, "Expected no error when patching profile schema attribute")
	})

}
