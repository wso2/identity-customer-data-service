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
	"context"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/test/setup"
	"testing"
)

func TestEnrichmentRule_CreateUpdateDelete(t *testing.T) {
	ctx := context.Background()
	testDB, err := setup.SetupTestDB(ctx)
	assert.NoError(t, err)
	defer testDB.Container.Terminate(ctx)

	// Override the provider DB
	provider.SetTestDB(testDB.DB)

	svc := service.GetEnrichmentRuleService()

	// Step 1: Create Rule
	rule := model.ProfileEnrichmentRule{
		PropertyName:      "traits.test_interest",
		ValueType:         "arrayOfString",
		MergeStrategy:     "overwrite",
		Value:             "music",
		ComputationMethod: "static",
		Trigger: model.RuleTrigger{
			EventType:  "track",
			EventName:  "interest_selected",
			Conditions: []model.RuleCondition{},
		},
	}

	err = svc.AddEnrichmentRule(rule)
	assert.NoError(t, err)

	// Step 2: Fetch it to get assigned rule_id
	rules, err := svc.GetEnrichmentRules()
	assert.NoError(t, err)
	var inserted model.ProfileEnrichmentRule
	for _, r := range rules {
		if r.PropertyName == rule.PropertyName {
			inserted = r
			break
		}
	}
	assert.NotEmpty(t, inserted.RuleId)

	// Step 3: Update the rule
	inserted.MergeStrategy = "combine"
	inserted.ComputationMethod = "extract"
	inserted.SourceField = "genre"

	err = svc.PutEnrichmentRule(inserted)
	assert.NoError(t, err)

	// Step 4: Verify the update
	updated, err := svc.GetEnrichmentRule(inserted.RuleId)
	assert.NoError(t, err)
	assert.Equal(t, "combine", updated.MergeStrategy)
	assert.Equal(t, "extract", updated.ComputationMethod)
	assert.Equal(t, "genre", updated.SourceField)
	assert.Equal(t, "value", "")

	// Step 5: Delete
	err = svc.DeleteEnrichmentRule(inserted.RuleId)
	assert.NoError(t, err)

	// Verify deletion
	deleted, err := svc.GetEnrichmentRule(inserted.RuleId)
	assert.NoError(t, err)
	assert.Empty(t, deleted.RuleId)
}
