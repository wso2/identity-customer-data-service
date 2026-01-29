package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/test/integration/utils"

	"github.com/stretchr/testify/require"
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

func Test_UnificationRule(t *testing.T) {

	SuperTenantOrg := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	profileSchemaService := schemaService.GetProfileSchemaService()
	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		// bypass app verification with IDP
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	t.Run("Pre-requisite: Add_schema_attribute", func(t *testing.T) {
		schemaAttributes := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         SuperTenantOrg,
				AttributeName: "identity_attributes.email",
				AttributeId:   uuid.New().String(),
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
			},
		}
		err := profileSchemaService.AddProfileSchemaAttributesForScope(schemaAttributes, constants.IdentityAttributes, SuperTenantOrg)
		require.NoError(t, err, "Failed to add enrichment rule dependency")
	})

	unificationRuleService := service.GetUnificationRuleService()

	rule := model.UnificationRule{
		RuleName:     "Email based",
		RuleId:       uuid.New().String(),
		TenantId:     SuperTenantOrg,
		PropertyName: "identity_attributes.email",
		Priority:     1,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	t.Run("Add_unification_rule", func(t *testing.T) {
		err := unificationRuleService.AddUnificationRule(rule, SuperTenantOrg)
		require.NoError(t, err, "Failed to add unification rule")
	})

	t.Run("Reject_complex_attribute_in_unification_rule", func(t *testing.T) {
		//  Add a valid sub-attribute
		subAttr := profileSchema.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.orders.payment.method",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		}
		err := profileSchemaService.AddProfileSchemaAttributesForScope([]profileSchema.ProfileSchemaAttribute{subAttr}, constants.Traits, SuperTenantOrg)
		require.NoError(t, err, "Failed to add sub-attribute to schema")

		//  Add parent complex attribute referencing sub-attribute
		parentAttr := profileSchema.ProfileSchemaAttribute{
			OrgId:         SuperTenantOrg,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.orders.payment",
			ValueType:     constants.ComplexDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			SubAttributes: []profileSchema.SubAttribute{
				{
					AttributeId:   subAttr.AttributeId,
					AttributeName: subAttr.AttributeName,
				},
			},
		}
		err = profileSchemaService.AddProfileSchemaAttributesForScope([]profileSchema.ProfileSchemaAttribute{parentAttr}, constants.Traits, SuperTenantOrg)
		require.NoError(t, err, "Failed to add complex attribute to schema")

		// Try creating a unification rule with that complex attribute
		rule := model.UnificationRule{
			RuleName:     "Payment based",
			RuleId:       uuid.New().String(),
			TenantId:     SuperTenantOrg,
			PropertyName: "traits.orders.payment",
			Priority:     2,
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}

		err = unificationRuleService.AddUnificationRule(rule, SuperTenantOrg)
		errDesc := utils.ExtractErrorDescription(err)
		require.Contains(t, errDesc, "not allowed as it is a complex data type", "Expected validation message for complex attribute rejection")
	})

	t.Run("Get_all_unification_rules", func(t *testing.T) {
		rules, err := unificationRuleService.GetUnificationRules(SuperTenantOrg)
		require.NoError(t, err, "Failed to fetch unification rules")
		require.NotEmpty(t, rules, "Unification rule list is empty")
	})

	t.Run("Update_unification_rule", func(t *testing.T) {
		rule.IsActive = false // reflect change in local object
		err := unificationRuleService.PatchUnificationRule(rule.RuleId, SuperTenantOrg, rule)
		require.NoError(t, err, "Failed to patch unification rule")

		updated, err := unificationRuleService.GetUnificationRule(rule.RuleId)
		require.NoError(t, err, "Failed to fetch updated rule")
		require.False(t, updated.IsActive, "Expected is_active to be false")
	})

	t.Run("Delete_unification_rule", func(t *testing.T) {
		err := unificationRuleService.DeleteUnificationRule(rule.RuleId)
		require.NoError(t, err, "Failed to delete unification rule")
	})

	// Todo : Add cases for each unification rule and ensure they are functioning correct

	t.Cleanup(func() {
		rules, _ := unificationRuleService.GetUnificationRules(SuperTenantOrg)
		for _, r := range rules {
			_ = unificationRuleService.DeleteUnificationRule(r.RuleId)
		}
		_ = profileSchemaService.DeleteProfileSchema(SuperTenantOrg)
		_ = profileSchemaService.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
	})
}
