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
	UnificationModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

// TestRuleValuesChanged decides whether an admin's "these are different people" survives a
// profile edit. Clearing rejections on every update meant an unrelated change — a marketing
// preference, a last-seen timestamp — resurrected every pair the admin had dismissed, so
// the same comparisons came back indefinitely.
func TestRuleValuesChanged(t *testing.T) {
	rules := []UnificationModel.UnificationRule{
		{PropertyName: "identity_attributes.email", IsActive: true},
		{PropertyName: "traits.phone", IsActive: true},
		{PropertyName: "traits.retired", IsActive: false},
	}

	tests := []struct {
		name   string
		before map[string]interface{}
		after  map[string]interface{}
		want   bool
	}{
		{
			name:   "unrelated attribute changed",
			before: map[string]interface{}{"identity_attributes.email": "a@acme.com", "traits.city": "Colombo"},
			after:  map[string]interface{}{"identity_attributes.email": "a@acme.com", "traits.city": "Kandy"},
			want:   false,
		},
		{
			name:   "matched attribute changed",
			before: map[string]interface{}{"identity_attributes.email": "a@acme.com"},
			after:  map[string]interface{}{"identity_attributes.email": "b@acme.com"},
			want:   true,
		},
		{
			name:   "matched attribute added",
			before: map[string]interface{}{},
			after:  map[string]interface{}{"traits.phone": "0771234567"},
			want:   true,
		},
		{
			name:   "matched attribute removed",
			before: map[string]interface{}{"traits.phone": "0771234567"},
			after:  map[string]interface{}{},
			want:   true,
		},
		{
			name:   "inactive rule's attribute changed",
			before: map[string]interface{}{"traits.retired": "yes"},
			after:  map[string]interface{}{"traits.retired": "no"},
			want:   false,
		},
		{
			name:   "multi-valued attribute reordered",
			before: map[string]interface{}{"traits.phone": []interface{}{"0771234567", "0712345678"}},
			after:  map[string]interface{}{"traits.phone": []interface{}{"0712345678", "0771234567"}},
			want:   false,
		},
		{
			name:   "multi-valued attribute extended",
			before: map[string]interface{}{"traits.phone": []interface{}{"0771234567"}},
			after:  map[string]interface{}{"traits.phone": []interface{}{"0771234567", "0712345678"}},
			want:   true,
		},
		{
			name:   "nothing changed",
			before: map[string]interface{}{"identity_attributes.email": "a@acme.com", "traits.phone": "0771234567"},
			after:  map[string]interface{}{"identity_attributes.email": "a@acme.com", "traits.phone": "0771234567"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleValuesChanged(tt.before, tt.after, rules); got != tt.want {
				t.Errorf("ruleValuesChanged = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasAttributeMatchingAnyRule covers the enqueue gate. A profile with no rule-relevant
// attribute is skipped, but the caller must still enqueue it when it carries a userId,
// because same-userId merging holds with no rules configured at all.
func TestHasAttributeMatchingAnyRule(t *testing.T) {
	rules := []UnificationModel.UnificationRule{
		{PropertyName: "identity_attributes.email", IsActive: true},
		{PropertyName: "traits.retired", IsActive: false},
	}

	if hasAttributeMatchingAnyRule(map[string]interface{}{"traits.city": "Colombo"}, rules) {
		t.Error("a profile with no rule attribute should not match")
	}
	if hasAttributeMatchingAnyRule(map[string]interface{}{"traits.retired": "yes"}, rules) {
		t.Error("an inactive rule should not cause a match")
	}
	if hasAttributeMatchingAnyRule(map[string]interface{}{"identity_attributes.email": nil}, rules) {
		t.Error("a nil value should not count as present")
	}
	if !hasAttributeMatchingAnyRule(map[string]interface{}{"identity_attributes.email": "a@acme.com"}, rules) {
		t.Error("an active rule's attribute should match")
	}
}

func TestToComparableStrings(t *testing.T) {
	if got := toComparableStrings(nil); got != nil {
		t.Errorf("nil => %v, want nil", got)
	}
	if got := toComparableStrings("one"); len(got) != 1 || got[0] != "one" {
		t.Errorf("string => %v", got)
	}
	if got := toComparableStrings([]interface{}{"a", "b"}); len(got) != 2 {
		t.Errorf("slice => %v", got)
	}
	if got := toComparableStrings(42); len(got) != 1 || got[0] != "42" {
		t.Errorf("int => %v", got)
	}
	_ = constants.AttributeTypeEmail
}
