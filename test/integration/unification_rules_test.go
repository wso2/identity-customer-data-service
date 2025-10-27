package integration

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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

	SUPER_TENANT_ORG := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	profileSchemaService := schemaService.GetProfileSchemaService()

	t.Run("Pre-requisite: Add_schema_attribute", func(t *testing.T) {
		schemaAttributes := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         SUPER_TENANT_ORG,
				AttributeName: "identity_attributes.email",
				AttributeId:   uuid.New().String(),
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
			},
		}
		err := profileSchemaService.AddProfileSchemaAttributesForScope(schemaAttributes, constants.IdentityAttributes)
		require.NoError(t, err, "Failed to add enrichment rule dependency")
	})

	unificationRuleService := service.GetUnificationRuleService()

	jsonData := []byte(`{
        "rule_name": "Email based",
		"rule_id": "` + uuid.New().String() + `",
		"tenant_id": "` + SUPER_TENANT_ORG + `",
        "property_name": "identity_attributes.email",
        "priority": 1,
        "is_active": true
    }`)

	var rule model.UnificationRule
	err := json.Unmarshal(jsonData, &rule)
	require.NoError(t, err, "Failed to unmarshal rule JSON")

	// Add timestamps programmatically
	rule.CreatedAt = time.Now().Unix()
	rule.UpdatedAt = time.Now().Unix()

	t.Run("Add_unification_rule", func(t *testing.T) {
		err := unificationRuleService.AddUnificationRule(rule, SUPER_TENANT_ORG)
		require.NoError(t, err, "Failed to add unification rule")
	})

	t.Run("Get_all_unification_rules", func(t *testing.T) {
		rules, err := unificationRuleService.GetUnificationRules(SUPER_TENANT_ORG)
		require.NoError(t, err, "Failed to fetch unification rules")
		require.NotEmpty(t, rules, "Unification rule list is empty")
	})

	t.Run("Update_unification_rule", func(t *testing.T) {
		updates := map[string]interface{}{
			"is_active": false,
		}
		err := unificationRuleService.PatchUnificationRule(rule.RuleId, updates)
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
		rules, _ := unificationRuleService.GetUnificationRules(SUPER_TENANT_ORG)
		for _, r := range rules {
			_ = unificationRuleService.DeleteUnificationRule(r.RuleId)
		}
		_ = profileSchemaService.DeleteProfileSchema(SUPER_TENANT_ORG)
		_ = profileSchemaService.DeleteProfileSchemaAttributesByScope(SUPER_TENANT_ORG, constants.IdentityAttributes)
	})
}
