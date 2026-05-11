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

// ScoreCandidate evaluates rules in priority order (waterfall cascade). Rules are sorted by
// priority ascending so lower priority number = evaluated first. For each rule where both
// profiles have data, the match score is computed. The first rule that produces a score at
// or above the manual review threshold is decisive — its score is returned immediately and
// lower-priority rules are not evaluated. If no rule reaches the manual review threshold,
// 0.0 is returned (UNIQUE). This means priority is a true gate: a high-priority deterministic
// match is sufficient on its own, and lower-priority rules never dilute it.
func ScoreCandidate(
	inputAttrs map[string]interface{},
	candidate *model.ProfileData,
	rules []urModel.UnificationRule,
	thresholds model.Thresholds,
) (float64, map[string]float64) {
	logger := log.GetLogger()

	breakdown := make(map[string]float64)

	if len(rules) == 0 {
		logger.Warn("Scorer: no rules — returning score 0.0",
			log.String("candidateID", candidate.ProfileID))
		return 0.0, breakdown
	}

	for _, rule := range rules {
		vals1 := getStringValues(inputAttrs, rule.PropertyName)
		if len(vals1) == 0 {
			continue
		}
		vals2 := candidate.GetAllAttributeValues(rule.PropertyName)
		if len(vals2) == 0 {
			// Candidate has no data for this rule — cannot decide, fall to next priority.
			continue
		}

		effectiveMode := constants.UnificationModeStrict
		if rule.UnificationMethod == constants.UnificationMethodFuzzy {
			effectiveMode = constants.UnificationModeSmart
		}

		score := bestMatchScore(vals1, vals2, rule.AttributeType, effectiveMode)
		breakdown[rule.PropertyName] = score

		if score >= thresholds.ManualReview {
			// Decisive match at this priority — stop, do not evaluate lower priorities.
			return score, breakdown
		}
		// Score below threshold — this rule could not decide, fall through to next priority.
	}

	return 0.0, breakdown
}
