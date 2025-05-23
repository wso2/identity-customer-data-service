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

	"github.com/stretchr/testify/assert"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
)

func Test_EnrichmentRule(t *testing.T) {

	svc := service.GetEnrichmentRuleService()

	// Define enrichment rules
	rule1 := model.ProfileEnrichmentRule{
		PropertyName:      "traits.hobby",
		ValueType:         "string",
		MergeStrategy:     "overwrite",
		Value:             "reading",
		ComputationMethod: "extract",
		SourceField:       "category",
		Trigger: model.RuleTrigger{
			EventType:  "track",
			EventName:  "hobby_selected",
			Conditions: []model.RuleCondition{},
		},
	}

	rule2 := model.ProfileEnrichmentRule{
		PropertyName:      "traits.favorite_classical_artist",
		ValueType:         "string",
		MergeStrategy:     "combine",
		Value:             "Mozart",
		ComputationMethod: "static",
		Trigger: model.RuleTrigger{
			EventType: "track",
			EventName: "music_selected",
			Conditions: []model.RuleCondition{
				{Field: "genre", Operator: "equals", Value: "classical"},
			},
		},
	}

	var addedRules []model.ProfileEnrichmentRule

	t.Run("Add_enrichment_rule", func(t *testing.T) {
		err := svc.AddEnrichmentRule(rule1)
		assert.NoError(t, err, "Failed to add rule1")

		err = svc.AddEnrichmentRule(rule2)
		assert.NoError(t, err, "Failed to add rule2")

		rules, err := svc.GetEnrichmentRules()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(rules), "Expected 2 enrichment rules")

		addedRules = rules
	})

	t.Run("Update_enrichment_rule", func(t *testing.T) {
		var toUpdate model.ProfileEnrichmentRule
		for _, r := range addedRules {
			if r.PropertyName == rule1.PropertyName {
				r.MergeStrategy = "combine"
				toUpdate = r
				break
			}
		}
		// todo: Need to define what fields can be updated
		err := svc.UpdateEnrichmentRule(toUpdate)
		assert.NoError(t, err, "Failed to update rule1")

		updated, err := svc.GetEnrichmentRule(toUpdate.RuleId)
		assert.NoError(t, err)
		assert.Equal(t, "combine", updated.MergeStrategy)
	})

	t.Run("Delete_enrichment_rule", func(t *testing.T) {
		for _, r := range addedRules {
			err := svc.DeleteEnrichmentRule(r.RuleId)
			assert.NoError(t, err, "Failed to delete rule %s", r.RuleId)
		}

		remaining, err := svc.GetEnrichmentRules()
		assert.NoError(t, err)
		assert.Empty(t, remaining, "Expected no remaining enrichment rules")
	})
}
