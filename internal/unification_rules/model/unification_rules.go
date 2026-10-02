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

package model

import (
	"sort"
	"time"
)

// UnificationRule represents rules for merging user profiles
type UnificationRule struct {
	RuleId            string `json:"rule_id" bson:"rule_id" binding:"required"`
	OrgHandle         string `json:"org_handle" bson:"org_handle" binding:"required"`
	RuleName          string `json:"rule_name" bson:"rule_name" binding:"required"`
	PropertyName      string `json:"property_name" bson:"property_name" binding:"required"`
	PropertyId        string `json:"property_id" bson:"property_id" binding:"required"`
	Priority          int    `json:"priority" bson:"priority" binding:"required"`
	IsActive          bool   `json:"is_active" bson:"is_active" binding:"required"`
	AttributeType     string `json:"attribute_type" bson:"attribute_type"`
	UnificationMethod string `json:"unification_method" bson:"unification_method"`
	// MatchStrength is how much an agreement on this attribute supports a merge, and
	// MismatchStrength how much a disagreement opposes one. They are independent: an
	// email match is strong evidence of sameness while an email mismatch is weak evidence
	// of difference, and a date of birth is the other way round.
	MatchStrength    string    `json:"match_strength" bson:"match_strength"`
	MismatchStrength string    `json:"mismatch_strength" bson:"mismatch_strength"`
	CreatedAt        time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" bson:"updated_at"`
}

// ApplyDefaults fills in the fields that older rules predate, deriving each from the
// attribute type so a rule stored before a field existed scores identically to a freshly
// created equivalent. Callers that load rules for matching must apply this, otherwise a
// legacy rule silently matches with an empty method and no evidence weights.
func ApplyDefaults(rule UnificationRule, defaultAttributeType, defaultMethod string,
	matchDefaults, mismatchDefaults map[string]string) UnificationRule {

	if rule.AttributeType == "" {
		rule.AttributeType = defaultAttributeType
	}
	if rule.UnificationMethod == "" {
		rule.UnificationMethod = defaultMethod
	}
	if rule.MatchStrength == "" {
		rule.MatchStrength = matchDefaults[rule.AttributeType]
	}
	if rule.MismatchStrength == "" {
		rule.MismatchStrength = mismatchDefaults[rule.AttributeType]
	}
	return rule
}

// ActiveSortedByPriority returns the active rules in evaluation order, with the fields that
// older rules predate filled in.
//
// Priority order is not cosmetic: the first rule that agrees sets the match score and is the
// one recorded as the reason for the merge, so anything that reads rules for matching must
// see them in the same order. Keeping one implementation is what stops the worker, the
// search path and the review path disagreeing about which rule was responsible.
func ActiveSortedByPriority(rules []UnificationRule, defaultAttributeType, defaultMethod string,
	matchDefaults, mismatchDefaults map[string]string) []UnificationRule {

	active := make([]UnificationRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}
		active = append(active, ApplyDefaults(rule, defaultAttributeType, defaultMethod,
			matchDefaults, mismatchDefaults))
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})
	return active
}

// PrimaryRuleName returns the name of the rule that drove a match: the highest-priority rule
// whose score reaches the agreement bar.
//
// That is the rule the scorer held accountable, so it is what belongs on the merge as its
// reason. Deriving it from the breakdown rather than from the final score matters because
// the final score may have been capped on its way to a review task — comparing against it
// would find no rule at all and fall back to a generic reason.
func PrimaryRuleName(breakdown map[string]float64, rules []UnificationRule,
	agreementThreshold float64) (string, bool) {

	for _, rule := range rules {
		if score, scored := breakdown[rule.PropertyName]; scored && score >= agreementThreshold {
			return rule.RuleName, true
		}
	}
	return "", false
}
