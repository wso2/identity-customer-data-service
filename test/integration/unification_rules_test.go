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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	enrModel "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	enrService "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

func Test_UnificationRule(t *testing.T) {

	t.Run("Pre-requisite: Add_enrichment_rule", func(t *testing.T) {
		enrichmentRule := enrModel.ProfileEnrichmentRule{
			PropertyName:      "identity.email",
			ValueType:         "string",
			MergeStrategy:     "overwrite",
			Value:             "test",
			ComputationMethod: "extract",
			SourceField:       "email",
			Trigger: enrModel.RuleTrigger{
				EventType: "identify",
				EventName: "user_logged_in",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}
		err := enrService.GetEnrichmentRuleService().AddEnrichmentRule(enrichmentRule)
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

	t.Run("Add_unification_rule ", func(t *testing.T) {
		err := svc.AddUnificationRule(rule)
		require.NoError(t, err, "Failed to add unification rule")
	})

	t.Run("Get_all_unification_rules", func(t *testing.T) {
		rules, err := svc.GetUnificationRules()
		require.NoError(t, err, "Failed to fetch unification rules")
		require.NotEmpty(t, rules, "Unification rule list is empty")
	})

	t.Run("Update_unification_rule", func(t *testing.T) {
		updates := map[string]interface{}{
			"is_active": false,
		}
		err := svc.PatchResolutionRule(rule.RuleId, updates)
		require.NoError(t, err, "Failed to patch unification rule")

		updated, err := svc.GetUnificationRule(rule.RuleId)
		require.NoError(t, err, "Failed to fetch updated rule")
		require.False(t, updated.IsActive, "Expected is_active to be false")
	})

	t.Run("Delete_unification_rule", func(t *testing.T) {
		err := svc.DeleteUnificationRule(rule.RuleName)
		require.NoError(t, err, "Failed to delete unification rule")
	})
}
