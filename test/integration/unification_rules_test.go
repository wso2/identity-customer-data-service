package integration

import (
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

func Test_UnificationRule(t *testing.T) {

	t.Run("Pre-requisite: Add_schema_attribute", func(t *testing.T) {
		schemaAttributes := []profileSchema.ProfileSchemaAttribute{
			{
				AttributeName:         "identity_attributes.email",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "",
			},
		}
		err := schemaService.GetProfileSchemaService().AddProfileSchemaAttributesForScope(schemaAttributes, constants.IdentityAttributes)
		require.NoError(t, err, "Failed to add enrichment rule dependency")
	})

	svc := service.GetUnificationRuleService()

	rule := model.UnificationRule{
		RuleName:  "Email based",
		Property:  "identity.email",
		Priority:  1,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	t.Run("Add_unification_rule", func(t *testing.T) {
		err := svc.AddUnificationRule(rule, "tenant-1")
		require.NoError(t, err, "Failed to add unification rule")
	})

	t.Run("Get_all_unification_rules", func(t *testing.T) {
		rules, err := svc.GetUnificationRules("tenant-1")
		require.NoError(t, err, "Failed to fetch unification rules")
		require.NotEmpty(t, rules, "Unification rule list is empty")
	})

	t.Run("Update_unification_rule", func(t *testing.T) {
		updates := map[string]interface{}{
			"is_active": false,
		}
		err := svc.PatchResolutionRule(rule.RuleId, SU, updates)
		require.NoError(t, err, "Failed to patch unification rule")

		updated, err := svc.GetUnificationRule(rule.RuleId)
		require.NoError(t, err, "Failed to fetch updated rule")
		require.False(t, updated.IsActive, "Expected is_active to be false")
	})

	t.Run("Delete_unification_rule", func(t *testing.T) {
		err := svc.DeleteUnificationRule(rule.RuleId)
		require.NoError(t, err, "Failed to delete unification rule")
	})
}
