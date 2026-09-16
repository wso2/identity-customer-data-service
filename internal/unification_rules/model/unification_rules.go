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

import "time"

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
