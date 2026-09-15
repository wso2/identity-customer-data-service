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

package model

// MethodOption describes a single matching method available for an attribute type.
type MethodOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// StrengthOption describes one evidence-strength choice, worded for the direction it
// applies to. The same value means different things either way round — HIGH on a match says
// the attribute can merge two profiles on its own, HIGH on a mismatch says two different
// values mean two different people — so the two directions are offered separately rather
// than as one shared list.
type StrengthOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AttributeTypeOption describes an attribute type, the matching methods it supports, and
// the evidence strengths it is given when the operator does not choose them.
type AttributeTypeOption struct {
	Value          string         `json:"value"`
	Label          string         `json:"label"`
	AllowedMethods []MethodOption `json:"allowed_methods"`
	// Defaults for this attribute type. Clients should pre-select these and let the
	// operator override, rather than presenting an empty choice.
	DefaultMatchStrength    string `json:"default_match_strength"`
	DefaultMismatchStrength string `json:"default_mismatch_strength"`
}

// UnificationOptionsResponse is the payload returned by GET /unification-rules/options.
type UnificationOptionsResponse struct {
	AttributeTypes    []AttributeTypeOption `json:"attribute_types"`
	MatchStrengths    []StrengthOption      `json:"match_strengths"`
	MismatchStrengths []StrengthOption      `json:"mismatch_strengths"`
}
