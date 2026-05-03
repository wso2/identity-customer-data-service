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

// bestMatchScore returns the highest MatchAttribute score across all pairs of
// (input element) × (candidate element).
func bestMatchScore(vals1, vals2 []string, attrType, mode string) float64 {
	best := 0.0
	for _, v1 := range vals1 {
		for _, v2 := range vals2 {
			s := MatchAttribute(v1, v2, attrType, mode)
			if s > best {
				best = s
			}
		}
	}
	return best
}

// ScoreCandidate computes a match score using a weighted approach derived from
// unification rule priorities. The algorithm:
//
//  1. Each rule's weight comes from its priority rank (lower priority number = higher weight).
//  2. Only rules where BOTH the input and the candidate have data are "mutually applicable"
//     and count toward the weighted average. A rule where the candidate has no data is skipped
//     entirely, absence of data cannot confirm or deny a match, so it must not dilute the score.
//  3. The anchor threshold is computed from the highest weight among mutually applicable rules,
//     not from all rules. This prevents penalizing a lower-priority attribute when it is the
//     only one both profiles share.
//  4. Three penalties cap the score below autoMergeThreshold when the match is not trustworthy:
//     coverage (too few rules shared), anchor (only weak rules matched), majority (most rules
//     scored zero).
//  5. Multi-value attributes (arrays) are scored by taking the best element level match across
//     all pairs so any element match counts as a match for the rule.
func ScoreCandidate(
	inputAttrs map[string]interface{},
	candidate *model.ProfileData,
	rules []urModel.UnificationRule,
	autoMergeThreshold float64,
) (float64, map[string]float64) {
	logger := log.GetLogger()

	breakdown := make(map[string]float64)

	numOfRules := len(rules)
	if numOfRules == 0 {
		logger.Warn("Scorer: no rules — returning score 0.0",
			log.String("candidateID", candidate.ProfileID))
		return 0.0, breakdown
	}

	// Find the maximum weight among mutually applicable rules that means rules where BOTH the
	// input and the candidate have data. Rules are sorted by priority, so the first
	// mutually applicable rule carries the highest weight and sets the anchor threshold.
	maxApplicableWeight := 0.0
	for i, rule := range rules {
		if len(getStringValues(inputAttrs, rule.PropertyName)) > 0 &&
			len(candidate.GetAllAttributeValues(rule.PropertyName)) > 0 {
			maxApplicableWeight = float64(numOfRules - i)
			break
		}
	}
	if maxApplicableWeight == 0 {
		logger.Info(fmt.Sprintf("Scorer: candidate '%s' — no mutually applicable rules, returning 0.0", candidate.ProfileID))
		return 0.0, breakdown
	}

	// Anchor threshold relative to the applicable rules, not all rules.
	// This prevents penalizing a lower-priority attribute when it is the only
	// one both profiles share.
	anchorThreshold := maxApplicableWeight * constants.ScoreAnchorFraction

	weightedSum := 0.0
	applicableWeight := 0.0
	anchorMatched := false

	for i, rule := range rules {
		weight := float64(numOfRules - i)

		vals1 := getStringValues(inputAttrs, rule.PropertyName)
		if len(vals1) == 0 {
			continue
		}
		vals2 := candidate.GetAllAttributeValues(rule.PropertyName)
		if len(vals2) == 0 {
			// Candidate has no data for this rule so skip entirely.
			continue
		}

		// Both profiles have data, this rule is mutually applicable.
		applicableWeight += weight

		effectiveMode := constants.UnificationModeStrict
		switch rule.UnificationMethod {
		case constants.UnificationMethodFuzzy:
			effectiveMode = constants.UnificationModeSmart
		case constants.UnificationMethodDeterministic:
			effectiveMode = constants.UnificationModeStrict
		}

		// For multi-value attributes, take the best score across all element pairs.
		score := bestMatchScore(vals1, vals2, rule.AttributeType, effectiveMode)

		breakdown[rule.PropertyName] = score
		weightedSum += score * weight

		if score > constants.AnchorMatchMinScore && weight >= anchorThreshold {
			anchorMatched = true
		}

		logger.Info(fmt.Sprintf("Scorer: rule '%s' (%s) — matchScore=%.4f, weight=%.0f",
			rule.RuleName, rule.AttributeType, score, weight))
	}

	if applicableWeight == 0 {
		logger.Info(fmt.Sprintf("Scorer: candidate '%s' — no mutually applicable rules after scoring, returning 0.0", candidate.ProfileID))
		return 0.0, breakdown
	}

	finalScore := weightedSum / applicableWeight

	// Count mutually applicable rules (used by the penalty constraints below).
	applicableCount := 0
	nonMatchCount := 0
	for _, rule := range rules {
		if len(getStringValues(inputAttrs, rule.PropertyName)) == 0 {
			continue
		}
		if len(candidate.GetAllAttributeValues(rule.PropertyName)) == 0 {
			continue
		}
		applicableCount++
		if breakdown[rule.PropertyName] <= 0 {
			nonMatchCount++
		}
	}

	// Minimum coverage constraint: if fewer than 1/ScoreCoverageDenominator of all rules
	// have input data, a high score is likely an artifact of sparse data rather than a
	// genuine match so cap it below auto-merge threshold.
	if applicableCount > 0 && applicableCount*constants.ScoreCoverageDenominator < numOfRules && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - constants.ScorePenaltyOffset
		logger.Info(fmt.Sprintf("Scorer: coverage penalty for candidate '%s' — only %d/%d rules applicable, capped to %.4f",
			candidate.ProfileID, applicableCount, numOfRules, finalScore))
	}

	// Anchor penalty: if only weak (low-priority) rules matched, cap the score
	// just below the auto-merge threshold so it can still trigger manual review
	// but never auto-merge.
	if !anchorMatched && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - constants.ScorePenaltyOffset
		logger.Info(fmt.Sprintf("Scorer: anchor penalty applied for candidate '%s' — capped to %.4f",
			candidate.ProfileID, finalScore))
	}

	// Rule majority constraint: if ScoreMajorityNumerator/ScoreMajorityDenominator or more
	// of the applicable rules scored zero, the overall score is unreliable so cap it.
	if applicableCount > 0 && nonMatchCount*constants.ScoreMajorityDenominator >= applicableCount*constants.ScoreMajorityNumerator && finalScore >= autoMergeThreshold {
		finalScore = autoMergeThreshold - constants.ScorePenaltyOffset
		logger.Info(fmt.Sprintf("Scorer: rule majority penalty for candidate '%s' — %d/%d rules non-matching, capped to %.4f",
			candidate.ProfileID, nonMatchCount, applicableCount, finalScore))
	}

	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0.0 {
		finalScore = 0.0
	}

	return finalScore, breakdown
}
