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

package service

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// TestGetUnificationOptionsCoversEveryAttributeType guards the rule form against drifting
// from the engine: an attribute type the scorer knows but the options endpoint omits cannot
// be configured through the UI at all.
func TestGetUnificationOptionsCoversEveryAttributeType(t *testing.T) {
	options := (&UnificationRuleService{}).GetUnificationOptions()

	offered := make(map[string]bool, len(options.AttributeTypes))
	for _, attributeType := range options.AttributeTypes {
		offered[attributeType.Value] = true
	}

	for attributeType := range constants.AllowedAttributeTypes {
		if !offered[attributeType] {
			t.Errorf("attribute type %q is accepted by the API but not offered by the options endpoint",
				attributeType)
		}
	}
	if len(offered) != len(constants.AllowedAttributeTypes) {
		t.Errorf("options endpoint offers %d attribute types, API accepts %d",
			len(offered), len(constants.AllowedAttributeTypes))
	}
}

// TestGetUnificationOptionsSuppliesUsableDefaults checks that every attribute type arrives
// with both strengths pre-filled and valid. A client that trusts these and posts them back
// must produce a rule the handler accepts.
func TestGetUnificationOptionsSuppliesUsableDefaults(t *testing.T) {
	options := (&UnificationRuleService{}).GetUnificationOptions()

	for _, attributeType := range options.AttributeTypes {
		if !constants.AllowedEvidenceStrengths[attributeType.DefaultMatchStrength] {
			t.Errorf("%s: default_match_strength %q is not an allowed strength",
				attributeType.Value, attributeType.DefaultMatchStrength)
		}
		if !constants.AllowedEvidenceStrengths[attributeType.DefaultMismatchStrength] {
			t.Errorf("%s: default_mismatch_strength %q is not an allowed strength",
				attributeType.Value, attributeType.DefaultMismatchStrength)
		}
	}
}

// TestGetUnificationOptionsOffersBothDirections checks the two strength lists are complete
// and worded separately — HIGH means something different on a match than on a mismatch, and
// presenting one shared list would mislead whoever is configuring the rule.
func TestGetUnificationOptionsOffersBothDirections(t *testing.T) {
	options := (&UnificationRuleService{}).GetUnificationOptions()

	check := func(direction string, values []string, labels []string, descriptions []string) {
		if len(values) != len(constants.AllowedEvidenceStrengths) {
			t.Errorf("%s: offers %d strengths, %d are allowed",
				direction, len(values), len(constants.AllowedEvidenceStrengths))
		}
		for i, value := range values {
			if !constants.AllowedEvidenceStrengths[value] {
				t.Errorf("%s: %q is not an allowed strength", direction, value)
			}
			if labels[i] == "" || descriptions[i] == "" {
				t.Errorf("%s: %q has no label or description for the operator to read",
					direction, value)
			}
		}
	}

	var mValues, mLabels, mDescriptions []string
	for _, option := range options.MatchStrengths {
		mValues = append(mValues, option.Value)
		mLabels = append(mLabels, option.Label)
		mDescriptions = append(mDescriptions, option.Description)
	}
	check("match_strengths", mValues, mLabels, mDescriptions)

	var xValues, xLabels, xDescriptions []string
	for _, option := range options.MismatchStrengths {
		xValues = append(xValues, option.Value)
		xLabels = append(xLabels, option.Label)
		xDescriptions = append(xDescriptions, option.Description)
	}
	check("mismatch_strengths", xValues, xLabels, xDescriptions)

	// The wording must actually differ between directions, or the distinction is lost.
	if len(options.MatchStrengths) > 0 && len(options.MismatchStrengths) > 0 {
		if options.MatchStrengths[0].Description == options.MismatchStrengths[0].Description {
			t.Error("match and mismatch strengths share a description; the two directions mean different things")
		}
	}
}
