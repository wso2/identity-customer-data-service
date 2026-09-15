/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

package engine

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	urModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

// legacyRule is a rule as it exists for a tenant that predates typed matching: no
// attribute_type, no unification_method, no strengths — every field the feature added is
// empty and seeded at load time.
func legacyRule(property string, priority int) urModel.UnificationRule {
	return urModel.ApplyDefaults(urModel.UnificationRule{
		PropertyName: property,
		Priority:     priority,
		IsActive:     true,
	}, constants.AttributeTypePrimitiveExact, constants.UnificationMethodDeterministic,
		constants.DefaultMatchStrength, constants.DefaultMismatchStrength)
}

// TestLegacyDeterministicRulesStillMerge is the upgrade contract.
//
// Before typed matching, unification walked the rules in priority order and merged on the
// first exact match, whatever the other rules held. A tenant upgrading with only
// deterministic rules must keep seeing exactly that: any single rule matching exactly still
// merges, regardless of which priority it sits at or how many other rules are configured.
func TestLegacyDeterministicRulesStillMerge(t *testing.T) {
	if err := log.Init("error"); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	thresholds := model.Thresholds{AutoMergeEnabled: true, AutoMerge: 0.95, ManualReview: 0.75}
	ctx := ScoringContext{OrgHandle: "legacy", Thresholds: thresholds}

	rules := []urModel.UnificationRule{
		legacyRule("identity_attributes.email", 1),
		legacyRule("traits.phone", 2),
		legacyRule("identity_attributes.nic", 3),
	}

	// Each rule in turn is the only attribute the two profiles share.
	for _, property := range []string{
		"identity_attributes.email",
		"traits.phone",
		"identity_attributes.nic",
	} {
		t.Run(property, func(t *testing.T) {
			shared := map[string]interface{}{property: "shared-value-123"}
			candidate := &model.ProfileData{ProfileID: "candidate", Attributes: shared}

			score, breakdown := ScoreCandidate(shared, candidate, rules, ctx)
			if decision := model.Decide(score, thresholds); decision != constants.DecisionAutoMerge {
				t.Errorf("exact match on %s gave %s (score %.4f, breakdown %v); "+
					"before typed matching this merged", property, decision, score, breakdown)
			}
		})
	}
}

// TestLegacyRuleWithOneRuleConfigured covers the smallest legacy setup.
func TestLegacyRuleWithOneRuleConfigured(t *testing.T) {
	if err := log.Init("error"); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	thresholds := model.Thresholds{AutoMergeEnabled: true, AutoMerge: 0.95, ManualReview: 0.75}
	ctx := ScoringContext{OrgHandle: "legacy", Thresholds: thresholds}

	rules := []urModel.UnificationRule{legacyRule("identity_attributes.email", 1)}
	shared := map[string]interface{}{"identity_attributes.email": "a@acme.com"}
	candidate := &model.ProfileData{ProfileID: "candidate", Attributes: shared}

	score, _ := ScoreCandidate(shared, candidate, rules, ctx)
	if decision := model.Decide(score, thresholds); decision != constants.DecisionAutoMerge {
		t.Errorf("single legacy rule matching exactly gave %s, want %s", decision, constants.DecisionAutoMerge)
	}
}

// TestLegacyRulesDoNotMergeOnMismatch checks the other half of the contract: exact matching
// stays exact, so values that merely resemble each other must not merge.
func TestLegacyRulesDoNotMergeOnMismatch(t *testing.T) {
	if err := log.Init("error"); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	thresholds := model.Thresholds{AutoMergeEnabled: true, AutoMerge: 0.95, ManualReview: 0.75}
	ctx := ScoringContext{OrgHandle: "legacy", Thresholds: thresholds}

	rules := []urModel.UnificationRule{legacyRule("identity_attributes.email", 1)}

	incoming := map[string]interface{}{"identity_attributes.email": "a.smith@acme.com"}
	candidate := &model.ProfileData{ProfileID: "candidate",
		Attributes: map[string]interface{}{"identity_attributes.email": "a.smyth@acme.com"}}

	score, _ := ScoreCandidate(incoming, candidate, rules, ctx)
	if decision := model.Decide(score, thresholds); decision == constants.DecisionAutoMerge {
		t.Errorf("a deterministic rule merged two different values (score %.4f)", score)
	}
}
