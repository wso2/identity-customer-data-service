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
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	urModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

// ScoreCandidate computes a match score using a weighted approach derived from
// unification-rule priorities. The algorithm:
//
//  1. Each rule's weight comes from its priority rank (lower priority number = higher weight).
//  2. Only rules where the input actually has data count toward the "applicable weight",
//     avoiding score dilution from missing fields.
//  3. An anchor penalty is applied: if no high-priority ("anchor") rule was matched,
//     the score is capped below autoMergeThreshold to prevent weak-only matches
//     from triggering an auto-merge.
func ScoreCandidate(
	inputAttrs map[string]interface{},
	candidate *model.ProfileData,
	rules []urModel.UnificationRule,
	mode string,
	autoMergeThreshold float64,
) (float64, map[string]float64) {
	logger := log.GetLogger()

	breakdown := make(map[string]float64)

	n := len(rules)
	if n == 0 {
		logger.Warn("Scorer: no rules — returning score 0.0",
			log.String("candidateID", candidate.ProfileID))
		return 0.0, breakdown
	}

	// Determine the anchor threshold: rules whose weight is in the top third
	// of the priority range are considered "anchors".
	maxWeight := float64(n) // weight of the highest-priority rule
	anchorThreshold := maxWeight * 2.0 / 3.0

	weightedSum := 0.0
	applicableWeight := 0.0
	anchorMatched := false

	for i, rule := range rules {
		weight := float64(n - i) // highest-priority rule gets weight = n

		val1 := getStringValue(inputAttrs, rule.PropertyName)
		if val1 == "" {
			logger.Debug(fmt.Sprintf("Scorer: skipping rule '%s' — input has no value", rule.RuleName))
			continue
		}

		// This rule is applicable (input has data for it).
		applicableWeight += weight

		val2 := candidate.GetAttribute(rule.PropertyName)
		if val2 == "" {
			logger.Debug(fmt.Sprintf("Scorer: rule '%s' — candidate missing value (score=0, weight=%.0f)",
				rule.RuleName, weight))
			// Counts toward applicable weight but contributes 0 to the sum.
			continue
		}

		effectiveMode := mode
		switch rule.UnificationMethod {
		case constants.UnificationMethodFuzzy:
			effectiveMode = constants.UnificationModeSmart
		case constants.UnificationMethodDeterministic:
			effectiveMode = constants.UnificationModeStrict
		}

		score := MatchAttribute(val1, val2, rule.AttributeType, effectiveMode)

		breakdown[rule.PropertyName] = score
		weightedSum += score * weight

		if score > 0 && weight >= anchorThreshold {
			anchorMatched = true
		}

		logger.Info(fmt.Sprintf("Scorer: rule '%s' (%s) — matchScore=%.4f, weight=%.0f, weightedSum=%.4f",
			rule.RuleName, rule.AttributeType, score, weight, weightedSum))
	}

	if applicableWeight == 0 {
		logger.Info(fmt.Sprintf("Scorer: candidate '%s' — no applicable rules, returning 0.0", candidate.ProfileID))
		return 0.0, breakdown
	}

	finalScore := weightedSum / applicableWeight

	// Count applicable rules upfront (used by multiple constraints below).
	applicableCount := 0
	nonMatchCount := 0
	for _, rule := range rules {
		val1 := getStringValue(inputAttrs, rule.PropertyName)
		if val1 == "" {
			continue
		}
		applicableCount++
		if breakdown[rule.PropertyName] <= 0 {
			nonMatchCount++
		}
	}

	// Minimum coverage constraint: A single matching attribute should not trigger auto-merge.
	if applicableCount > 0 && applicableCount*3 < n && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - 0.01
		logger.Info(fmt.Sprintf("Scorer: coverage penalty for candidate '%s' — only %d/%d rules applicable, capped to %.4f",
			candidate.ProfileID, applicableCount, n, finalScore))
	}

	// Anchor penalty: if only weak (low-priority) rules matched, cap the score
	// just below the auto-merge threshold so it can still trigger manual review
	// but never auto-merge.
	if !anchorMatched && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - 0.01
		logger.Info(fmt.Sprintf("Scorer: anchor penalty applied for candidate '%s' — capped to %.4f",
			candidate.ProfileID, finalScore))
	}

	// Rule majority constraint: if 2/3 or more of the applicable rules
	// scored <= 0, cap below auto-merge threshold to prevent weak matches.
	if applicableCount > 0 && nonMatchCount*3 >= applicableCount*2 && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - 0.01
		logger.Info(fmt.Sprintf("Scorer: rule majority penalty for candidate '%s' — %d/%d rules non-matching, capped to %.4f",
			candidate.ProfileID, nonMatchCount, applicableCount, finalScore))
	}

	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0.0 {
		finalScore = 0.0
	}

	logger.Info(fmt.Sprintf(
		"Scorer: candidate '%s' — achievedWeight=%.2f, applicableWeight=%.2f, anchorMatched=%v, final=%.4f, mode=%s",
		candidate.ProfileID, weightedSum, applicableWeight, anchorMatched, finalScore, mode))

	return finalScore, breakdown
}
