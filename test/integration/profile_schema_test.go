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

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/test/integration/utils"
)

// createAttr is a helper to quickly generate a schema attribute.
func createAttr(org, name, vType, merge, mut string) model.ProfileSchemaAttribute {
	return model.ProfileSchemaAttribute{
		OrgId:                 org,
		AttributeId:           uuid.New().String(),
		AttributeName:         name,
		ValueType:             vType,
		MergeStrategy:         merge,
		Mutability:            mut,
		ApplicationIdentifier: "",
	}
}

func Test_ProfileSchemaService(t *testing.T) {

	SuperTenantOrg := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	svc := schemaService.GetProfileSchemaService()

	t.Run("Add Operations", func(t *testing.T) {

		t.Run("Add_ValidAttributes_ShouldSucceed", func(t *testing.T) {
			// 1. Identity attributes
			identityAttrs := []model.ProfileSchemaAttribute{
				createAttr(SuperTenantOrg, "identity_attributes.email", constants.StringDataType, "combine", constants.MutabilityReadWrite),
				createAttr(SuperTenantOrg, "identity_attributes.phone", constants.StringDataType, "combine", constants.MutabilityReadWrite),
			}
			err := svc.AddProfileSchemaAttributesForScope(identityAttrs, constants.IdentityAttributes)
			require.NoError(t, err, "Failed to add identity attributes")

			// 2. Traits
			traits := []model.ProfileSchemaAttribute{
				{
					OrgId:         SuperTenantOrg,
					AttributeId:   uuid.New().String(),
					AttributeName: "traits.interests",
					ValueType:     constants.StringDataType,
					MergeStrategy: "combine",
					Mutability:    constants.MutabilityReadWrite,
					MultiValued:   true,
				},
			}
			err = svc.AddProfileSchemaAttributesForScope(traits, constants.Traits)
			require.NoError(t, err, "Failed to add traits attributes")

			// 3. Application Data
			appData := []model.ProfileSchemaAttribute{
				{
					OrgId:                 SuperTenantOrg,
					AttributeId:           uuid.New().String(),
					AttributeName:         "application_data.device_id",
					ValueType:             constants.StringDataType,
					MergeStrategy:         "combine",
					Mutability:            constants.MutabilityReadWrite,
					MultiValued:           true,
					ApplicationIdentifier: "app_1",
				},
			}
			err = svc.AddProfileSchemaAttributesForScope(appData, constants.ApplicationData)
			require.NoError(t, err, "Failed to add application_data attributes")

			_ = svc.DeleteProfileSchema(SuperTenantOrg)
			_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
		})

		t.Run("Add_InvalidScope_ShouldFail", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "invalidscope.email", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, "invalidscope")
			errDesc := utils.ExtractErrorDescription(err)
			require.Contains(t, errDesc, "Invalid scope", "Expected validation failure for invalid scope")
		})

		t.Run("Add_ConflictingScope_ShouldFail", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "identity_attributes.email", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, "invalidscope")
			errDesc := utils.ExtractErrorDescription(err)
			require.Contains(t, errDesc, "does not match the scope", "Expected validation failure for invalid scope")
		})

		t.Run("Add_MissingAppIdentifier_ShouldFail", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "application_data.email", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.ApplicationData)
			errDesc := utils.ExtractErrorDescription(err)
			require.Contains(t, errDesc, "Application identifier is required", "Expected validation failure for missing application identifier")
		})

		t.Run("Add_TooDeepAttribute_ShouldFail", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "traits.orders.payment.card.type.extra", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.Traits)
			errDesc := utils.ExtractErrorDescription(err)
			require.Contains(t, errDesc, "Attribute exceeds the maximum depth of 4", "Expected validation failure for attribute depth > 4")
		})

		t.Run("Add_MaxDepthAttribute_ShouldSucceed", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "traits.orders.payment.card.type", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.Traits)
			require.NoError(t, err, "Expected success for depth 4 attribute")
			_ = svc.DeleteProfileSchema(SuperTenantOrg)
			_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
		})

		t.Run("Add_ValidSubAttribute_ShouldSucceed", func(t *testing.T) {
			// Create sub-attribute first
			subAttr := createAttr(SuperTenantOrg, "traits.orders.payment", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{subAttr}, constants.Traits)
			require.NoError(t, err)

			// Create parent with sub-attribute (valid: one level deeper)
			parent := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders",
				ValueType:     constants.ComplexDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				SubAttributes: []model.SubAttribute{
					{
						AttributeId:   subAttr.AttributeId,
						AttributeName: subAttr.AttributeName,
					},
				},
			}

			err = svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{parent}, constants.Traits)
			require.NoError(t, err, "Expected success for valid sub-attribute relationship")
			_ = svc.DeleteProfileSchema(SuperTenantOrg)
			_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
		})

		t.Run("Add_InvalidSubAttribute_ShouldFail", func(t *testing.T) {
			// Step 1: Create sub-attribute first
			subAttr := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders.payment.card.type", // depth 4
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
			}
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{subAttr}, constants.Traits)
			require.NoError(t, err, "Sub-attribute creation failed unexpectedly")

			// Step 2: Parent referencing invalid deeper sub-attribute
			parent := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders",
				ValueType:     constants.ComplexDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				SubAttributes: []model.SubAttribute{
					{
						AttributeId:   subAttr.AttributeId,
						AttributeName: subAttr.AttributeName,
					},
				},
			}

			err = svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{parent}, constants.Traits)
			errDesc := utils.ExtractErrorDescription(err)
			require.Contains(t, errDesc, "one level deeper", "Expected failure due to invalid sub-attribute depth")
			_ = svc.DeleteProfileSchema(SuperTenantOrg)
			_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
		})

		t.Run("Add_MaxDepthHierarchy_ShouldSucceed", func(t *testing.T) {
			// 1️ Level 4 leaf
			l4 := createAttr(SuperTenantOrg, "traits.orders.payment.card.type",
				constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{l4}, constants.Traits)
			require.NoError(t, err, "Failed to add level 4 attribute")

			// 2️Level 3 parent (complex) → references level 4
			l3 := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders.payment.card",
				ValueType:     constants.ComplexDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				SubAttributes: []model.SubAttribute{
					{AttributeId: l4.AttributeId, AttributeName: l4.AttributeName},
				},
			}
			err = svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{l3}, constants.Traits)
			require.NoError(t, err, "Failed to add level 3 attribute")

			// 3️ Level 2 parent → references level 3
			l2 := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders.payment",
				ValueType:     constants.ComplexDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				SubAttributes: []model.SubAttribute{
					{AttributeId: l3.AttributeId, AttributeName: l3.AttributeName},
				},
			}
			err = svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{l2}, constants.Traits)
			require.NoError(t, err, "Failed to add level 2 attribute")

			// 4️Level 1 parent → references level 2
			l1 := model.ProfileSchemaAttribute{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.orders",
				ValueType:     constants.ComplexDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				SubAttributes: []model.SubAttribute{
					{AttributeId: l2.AttributeId, AttributeName: l2.AttributeName},
				},
			}
			err = svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{l1}, constants.Traits)
			require.NoError(t, err, "Failed to add top-level parent attribute")

			//  Everything should pass, proving depth=4 hierarchy works correctly
			_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.Traits)
		})

	})

	t.Run("Get Operations", func(t *testing.T) {

		identityAttrs := []model.ProfileSchemaAttribute{
			createAttr(SuperTenantOrg, "identity_attributes.email", constants.StringDataType, "combine", constants.MutabilityReadWrite),
			createAttr(SuperTenantOrg, "identity_attributes.phone", constants.StringDataType, "combine", constants.MutabilityReadWrite),
		}
		_ = svc.AddProfileSchemaAttributesForScope(identityAttrs, constants.IdentityAttributes)

		// 2. Traits
		traits := []model.ProfileSchemaAttribute{
			{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.interests",
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				MultiValued:   true,
			},
		}
		_ = svc.AddProfileSchemaAttributesForScope(traits, constants.Traits)

		// 3. Application Data
		appData := []model.ProfileSchemaAttribute{
			{
				OrgId:                 SuperTenantOrg,
				AttributeId:           uuid.New().String(),
				AttributeName:         "application_data.device_id",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				MultiValued:           true,
				ApplicationIdentifier: "app_1",
			},
		}
		_ = svc.AddProfileSchemaAttributesForScope(appData, constants.ApplicationData)

		t.Run("Get_ProfileSchema_Success", func(t *testing.T) {
			schema, err := svc.GetProfileSchema(SuperTenantOrg)
			require.NoError(t, err)
			require.NotNil(t, schema)
			require.Contains(t, schema, constants.IdentityAttributes)
			require.Contains(t, schema, constants.ApplicationData)
			require.Contains(t, schema, constants.Traits)
		})

		t.Run("Get_ById_ShouldReturnMatchingAttribute", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "identity_attributes.phone_number", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes)
			require.NoError(t, err)

			fetched, err := svc.GetProfileSchemaAttributeById(SuperTenantOrg, attr.AttributeId)
			require.NoError(t, err)
			require.Equal(t, attr.AttributeName, fetched.AttributeName)
		})

		t.Run("Get_ByScopeAndFilter_ShouldReturnFilteredResults", func(t *testing.T) {
			filters := []string{"attribute_name eq identity_attributes.email"}
			filtered, err := svc.GetProfileSchemaAttributesByScopeAndFilter(SuperTenantOrg, constants.IdentityAttributes, filters)
			require.NoError(t, err)
			require.NotNil(t, filtered)
			require.NotEmpty(t, filtered)
		})
	})

	t.Run("Update Operations", func(t *testing.T) {
		t.Run("Patch_ProfileSchemaAttribute_ById", func(t *testing.T) {
			attrId := uuid.New().String()
			attr := createAttr(SuperTenantOrg, "identity_attributes.temp_field", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			attr.AttributeId = attrId

			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.IdentityAttributes)
			require.NoError(t, err)

			updates := map[string]interface{}{
				"attribute_name": "identity_attributes.temp_field",
				"attribute_id":   attrId,
				"value_type":     "integer", // todo: ensure if we are allowing such changes
				"merge_strategy": "latest",
				"mutability":     constants.MutabilityReadWrite,
			}

			err = svc.PatchProfileSchemaAttributeById(SuperTenantOrg, attrId, updates)
			require.NoError(t, err, "Failed to patch profile schema attribute")

			// Verify patch persisted
			patched, err := svc.GetProfileSchemaAttributeById(SuperTenantOrg, attrId)
			require.NoError(t, err)
			require.Equal(t, "integer", patched.ValueType)
			require.Equal(t, "latest", patched.MergeStrategy)
		})
	})

	t.Run("Delete Operations", func(t *testing.T) {

		t.Run("Delete_ProfileSchemaAttribute_ById", func(t *testing.T) {
			attr := createAttr(SuperTenantOrg, "traits.to_delete", constants.StringDataType, "combine", constants.MutabilityReadWrite)
			err := svc.AddProfileSchemaAttributesForScope([]model.ProfileSchemaAttribute{attr}, constants.Traits)
			require.NoError(t, err)

			err = svc.DeleteProfileSchemaAttributeById(SuperTenantOrg, attr.AttributeId)
			require.NoError(t, err, "Expected no error when deleting schema attribute by ID")
		})

		t.Run("Delete_ProfileSchemaAttributes_ByScope", func(t *testing.T) {
			err := svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
			require.NoError(t, err, "Expected no error when deleting all schema attributes by scope")
		})
	})

	t.Cleanup(func() {
		_ = svc.DeleteProfileSchema(SuperTenantOrg)
		_ = svc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
	})
}
