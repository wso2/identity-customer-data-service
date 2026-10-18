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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/sharing/model"
)

// The test tree:
//
//	R
//	├── A
//	│   ├── A1
//	│   │   └── A1x
//	│   └── A2 (disabled)
//	└── B
func testTree() []TreeOrg {
	return []TreeOrg{
		{Id: "R", Handle: "r", Active: true},
		{Id: "A", Handle: "a", ParentId: "R", Active: true},
		{Id: "B", Handle: "b", ParentId: "R", Active: true},
		{Id: "A1", Handle: "a1", ParentId: "A", Active: true},
		{Id: "A2", Handle: "a2", ParentId: "A", Active: false},
		{Id: "A1x", Handle: "a1x", ParentId: "A1", Active: true},
	}
}

func policy(resourceType, resourceId, initiator string, targets []model.Target) model.Policy {
	return model.Policy{PolicyId: resourceId + "-p", ResourceType: resourceType, ResourceId: resourceId,
		OwningOrgId: initiator, InitiatingOrgId: initiator, Targets: targets}
}

func allChildren(initiator string) []model.Target {
	return []model.Target{{Scope: model.ScopeAllChildren, OrgId: initiator}}
}

func TestReach(t *testing.T) {

	cases := []struct {
		name     string
		policy   model.Policy
		expected []string
	}{
		{"all children skips inactive orgs",
			policy(model.ResourceSchemaAttribute, "x", "R", allChildren("R")),
			[]string{"A", "B", "A1", "A1x"}},
		{"one child only",
			policy(model.ResourceSchemaAttribute, "x", "R", []model.Target{{Scope: model.ScopeOrg, OrgId: "A"}}),
			[]string{"A"}},
		{"child subtree",
			policy(model.ResourceSchemaAttribute, "x", "R", []model.Target{{Scope: model.ScopeOrgSubtree, OrgId: "A"}}),
			[]string{"A", "A1", "A1x"}},
		{"a named target that is not a direct child reaches nothing",
			policy(model.ResourceSchemaAttribute, "x", "R", []model.Target{{Scope: model.ScopeOrg, OrgId: "A1"}}),
			[]string{}},
		{"a sub org shares with its own subtree",
			policy(model.ResourceSchemaAttribute, "x", "A", allChildren("A")),
			[]string{"A1", "A1x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Reach(testTree(), c.policy); !reflect.DeepEqual(got, c.expected) {
				t.Fatalf("expected %v, got %v", c.expected, got)
			}
		})
	}
}

func TestValidatePolicy(t *testing.T) {

	cases := []struct {
		name    string
		policy  model.Policy
		problem string
	}{
		{"valid", policy(model.ResourceSchemaAttribute, "x", "R",
			[]model.Target{{Scope: model.ScopeOrgSubtree, OrgId: "A"}}), ""},
		{"no targets", policy(model.ResourceSchemaAttribute, "x", "R", nil), "at least one target"},
		{"not a direct child", policy(model.ResourceSchemaAttribute, "x", "R",
			[]model.Target{{Scope: model.ScopeOrg, OrgId: "A1"}}), "not a direct child"},
		{"inactive child", policy(model.ResourceSchemaAttribute, "x", "A",
			[]model.Target{{Scope: model.ScopeOrg, OrgId: "A2"}}), "not active"},
		{"unknown scope", policy(model.ResourceSchemaAttribute, "x", "R",
			[]model.Target{{Scope: "ALL_OUS"}}), "not supported"},
		{"all children of another org", policy(model.ResourceSchemaAttribute, "x", "R",
			allChildren("A")), "must name the initiating organization"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := ValidatePolicy(testTree(), c.policy)
			if c.problem == "" {
				if len(problems) != 0 {
					t.Fatalf("expected no problems, got %v", problems)
				}
				return
			}
			if len(problems) == 0 || !strings.Contains(strings.Join(problems, ";"), c.problem) {
				t.Fatalf("expected a problem with %q, got %v", c.problem, problems)
			}
		})
	}
}

func statesByOrg(states []model.State, resourceId string) map[string]model.State {
	result := map[string]model.State{}
	for _, s := range states {
		if s.ResourceId == resourceId {
			result[s.OrgId] = s
		}
	}
	return result
}

func TestEvaluateAttributeConflicts(t *testing.T) {

	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "primary-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "R"},
			{Id: "b-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "B"},
			{Id: "a-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "A"},
		},
		// In A1 and A1x both shared attributes reach. The nearest owner (A) wins.
		Policies: []model.Policy{
			policy(model.ResourceSchemaAttribute, "primary-tier", "R", allChildren("R")),
			policy(model.ResourceSchemaAttribute, "a-tier", "A", allChildren("A")),
		},
	}
	states := Evaluate(in)

	primary := statesByOrg(states, "primary-tier")
	if primary["A"].State != model.StateConflicted || primary["A"].Reason != model.ReasonLocalNameConflict ||
		primary["A"].ConflictingResourceId != "a-tier" {
		t.Errorf("expected a local conflict in A, got %+v", primary["A"])
	}
	if primary["B"].State != model.StateConflicted || primary["B"].ConflictingResourceId != "b-tier" {
		t.Errorf("expected a local conflict in B, got %+v", primary["B"])
	}
	// The nearest owner wins: in A1 and A1x, the attribute of A wins over the attribute of the
	// primary organization.
	if primary["A1"].State != model.StateConflicted || primary["A1"].Reason != model.ReasonSharedNameConflict ||
		primary["A1"].ConflictingResourceId != "a-tier" {
		t.Errorf("expected the attribute of the primary org to lose to the nearer owner in A1, got %+v", primary["A1"])
	}
	a := statesByOrg(states, "a-tier")
	if a["A1"].State != model.StateActive || a["A1x"].State != model.StateActive {
		t.Errorf("expected the attribute of A to be active in A1 and A1x, got %+v %+v", a["A1"], a["A1x"])
	}
}

func TestEvaluateRuleDependencies(t *testing.T) {

	all := allChildren("R")
	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "primary-email", Name: "identity_attributes.email", ValueType: "string", OwnerOrgId: "R"},
			{Id: "a-email", Name: "identity_attributes.email", ValueType: "string", OwnerOrgId: "A"},
			{Id: "a1-email", Name: "identity_attributes.email", ValueType: "string", OwnerOrgId: "A1"},
			// B has no email attribute, and A1x has one with another type.
			{Id: "a1x-email", Name: "identity_attributes.email", ValueType: "integer", OwnerOrgId: "A1x"},
			{Id: "primary-loyalty", Name: "traits.loyalty_id", ValueType: "string", OwnerOrgId: "R"},
		},
		Rules: []RuleInfo{
			{Id: "email-rule", PropertyName: "identity_attributes.email", PropertyId: "primary-email", OwnerOrgId: "R"},
			{Id: "loyalty-rule", PropertyName: "traits.loyalty_id", PropertyId: "primary-loyalty", OwnerOrgId: "R"},
			{Id: "a1-local-email", PropertyName: "identity_attributes.email", PropertyId: "a1-email", OwnerOrgId: "A1"},
		},
		Policies: []model.Policy{
			policy(model.ResourceSchemaAttribute, "primary-loyalty", "R", []model.Target{{Scope: model.ScopeOrg,
				OrgId: "A"}}),
			policy(model.ResourceUnificationRule, "email-rule", "R", all),
			policy(model.ResourceUnificationRule, "loyalty-rule", "R", all),
		},
	}
	states := Evaluate(in)

	email := statesByOrg(states, "email-rule")
	if email["A"].State != model.StateActive {
		t.Errorf("expected the email rule to be active in A, got %+v", email["A"])
	}
	if email["B"].State != model.StateInactiveMissingAttribute {
		t.Errorf("expected a missing attribute in B, got %+v", email["B"])
	}
	if email["A1"].State != model.StateConflicted || email["A1"].ConflictingResourceId != "a1-local-email" {
		t.Errorf("expected a local rule conflict in A1, got %+v", email["A1"])
	}
	if email["A1x"].State != model.StateInactiveMissingAttribute {
		t.Errorf("expected an incompatible type in A1x to count as missing, got %+v", email["A1x"])
	}
	loyalty := statesByOrg(states, "loyalty-rule")
	if loyalty["A"].State != model.StateActive {
		t.Errorf("expected the loyalty rule to use the shared attribute in A, got %+v", loyalty["A"])
	}
	if loyalty["A1"].State != model.StateInactiveMissingAttribute {
		t.Errorf("expected a missing attribute in A1, because the attribute is shared with A only, got %+v",
			loyalty["A1"])
	}
}

// EvaluateOrg must give the same result as Evaluate on the full tree.
func TestEvaluateOrgMatchesTree(t *testing.T) {

	attrs := []AttributeInfo{
		{Id: "primary-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "R"},
		{Id: "a-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "A"},
		{Id: "b-tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "B"},
	}
	policies := []model.Policy{
		policy(model.ResourceSchemaAttribute, "primary-tier", "R", allChildren("R")),
		policy(model.ResourceSchemaAttribute, "a-tier", "A", allChildren("A")),
	}
	full := Evaluate(EngineInput{Orgs: testTree(), Attributes: attrs, Policies: policies})

	chain := []TreeOrg{testTree()[0], testTree()[1], testTree()[3], testTree()[5]} // R, A, A1, A1x
	chainAttrs := attrs[:2]                                                        // B is not in the chain
	got := EvaluateOrg(EngineInput{Orgs: chain, Attributes: chainAttrs, Policies: policies}, "A1x")

	var expected []model.State
	for _, s := range full {
		if s.OrgId == "A1x" {
			expected = append(expected, s)
		}
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %+v, got %+v", expected, got)
	}
}

func TestToTargets(t *testing.T) {

	selected := func(orgs ...model.ChildOrg) *model.ChildOrgs {
		return &model.ChildOrgs{Mode: model.ChildOrgsModeSelected, Orgs: orgs}
	}
	cases := []struct {
		name      string
		childOrgs *model.ChildOrgs
		expected  []model.Target
		problems  []string
	}{
		{"all", &model.ChildOrgs{Mode: model.ChildOrgsModeAll},
			[]model.Target{{Scope: model.ScopeAllChildren, OrgId: "R"}}, nil},
		{"selected org", selected(model.ChildOrg{OrgId: "B", Reach: model.ReachOrg}),
			[]model.Target{{Scope: model.ScopeOrg, OrgId: "B"}}, nil},
		{"selected subtree", selected(model.ChildOrg{OrgId: "A", Reach: model.ReachSubtree}),
			[]model.Target{{Scope: model.ScopeOrgSubtree, OrgId: "A"}}, nil},
		{"selected subtree and org", selected(model.ChildOrg{OrgId: "A", Reach: model.ReachSubtree},
			model.ChildOrg{OrgId: "B", Reach: model.ReachOrg}),
			[]model.Target{{Scope: model.ScopeOrgSubtree, OrgId: "A"}, {Scope: model.ScopeOrg, OrgId: "B"}}, nil},
		{"no child_orgs", nil, nil, []string{"child_orgs is required"}},
		{"no mode", &model.ChildOrgs{Orgs: []model.ChildOrg{{OrgId: "A", Reach: model.ReachOrg}}}, nil,
			[]string{"child_orgs.mode is required"}},
		{"unknown mode", &model.ChildOrgs{Mode: "some"}, nil,
			[]string{"child_orgs.mode 'some' is not valid. Use all or selected"}},
		{"orgs with mode all", &model.ChildOrgs{Mode: model.ChildOrgsModeAll,
			Orgs: []model.ChildOrg{{OrgId: "A", Reach: model.ReachOrg}}}, nil,
			[]string{"child_orgs.orgs must be empty when child_orgs.mode is all"}},
		{"no orgs with mode selected", selected(), nil,
			[]string{"child_orgs.orgs must have at least one organization when child_orgs.mode is selected"}},
		{"no reach", selected(model.ChildOrg{OrgId: "A"}), nil, []string{"the organization 'A' requires a reach"}},
		{"unknown reach", selected(model.ChildOrg{OrgId: "A", Reach: "all"}), nil,
			[]string{"the reach 'all' of the organization 'A' is not valid. Use org or subtree"}},
		{"no org_id", selected(model.ChildOrg{OrgId: " ", Reach: model.ReachOrg}), nil,
			[]string{"each child_orgs.orgs entry requires an org_id"}},
		{"one org twice", selected(model.ChildOrg{OrgId: "A", Reach: model.ReachOrg},
			model.ChildOrg{OrgId: "A", Reach: model.ReachSubtree}), nil,
			[]string{"the organization 'A' is in child_orgs.orgs more than once"}},
		{"all problems of the entries", selected(model.ChildOrg{Reach: model.ReachOrg},
			model.ChildOrg{OrgId: "A"}, model.ChildOrg{OrgId: "B", Reach: model.ReachOrg},
			model.ChildOrg{OrgId: "B", Reach: model.ReachOrg}), nil,
			[]string{"each child_orgs.orgs entry requires an org_id", "the organization 'A' requires a reach",
				"the organization 'B' is in child_orgs.orgs more than once"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, problems := ToTargets(c.childOrgs, "R")
			if c.problems != nil {
				if got != nil || !reflect.DeepEqual(problems, c.problems) {
					t.Fatalf("expected the problems %v, got %v %v", c.problems, got, problems)
				}
				return
			}
			if len(problems) > 0 || !reflect.DeepEqual(got, c.expected) {
				t.Fatalf("expected %v, got %v %v", c.expected, got, problems)
			}
			if back := ToChildOrgs(got); !reflect.DeepEqual(&back, c.childOrgs) {
				t.Fatalf("expected the same child_orgs back, got %+v", back)
			}
		})
	}
}

// Two shared rules on one property: the rule of the farthest owner wins, as in the run order.
func TestEvaluateRuleFarthestOwnerWins(t *testing.T) {

	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "primary-email", Name: "identity_attributes.email", ValueType: "string", OwnerOrgId: "R"},
			{Id: "a1-email", Name: "identity_attributes.email", ValueType: "string", OwnerOrgId: "A1"},
		},
		Rules: []RuleInfo{
			{Id: "primary-rule", PropertyName: "identity_attributes.email", PropertyId: "primary-email",
				OwnerOrgId: "R"},
			{Id: "a-rule", PropertyName: "identity_attributes.email", PropertyId: "primary-email", OwnerOrgId: "A"},
		},
		Policies: []model.Policy{
			policy(model.ResourceUnificationRule, "a-rule", "A", allChildren("A")),
			policy(model.ResourceUnificationRule, "primary-rule", "R", allChildren("R")),
		},
	}
	states := Evaluate(in)
	if s := statesByOrg(states, "primary-rule")["A1"]; s.State != model.StateActive {
		t.Errorf("expected the rule of the primary org to win in A1, got %+v", s)
	}
	if s := statesByOrg(states, "a-rule")["A1"]; s.State != model.StateConflicted ||
		s.ConflictingResourceId != "primary-rule" {
		t.Errorf("expected the rule of A to lose in A1, got %+v", s)
	}
}

// A share applies only in enabled organizations, and passes down through an organization that is
// not enabled.
func TestReachSkipsOrgsThatAreNotEnabled(t *testing.T) {

	orgs := testTree()
	for i := range orgs {
		if orgs[i].Id == "A" {
			orgs[i].NotEnabled = true
		}
	}
	got := Reach(orgs, policy(model.ResourceSchemaAttribute, "x", "R", allChildren("R")))
	if expected := []string{"B", "A1", "A1x"}; !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

// Organization access names direct children of the primary organization, the same as a share
// policy.
func TestValidateOrgAccess(t *testing.T) {

	access := func(targets ...model.Target) model.Policy {
		return model.Policy{PolicyId: "oa", ResourceType: model.ResourceOrganizationAccess,
			ResourceId: model.OrganizationAccessResourceId, OwningOrgId: "R", InitiatingOrgId: "R", Targets: targets}
	}
	cases := []struct {
		name    string
		policy  model.Policy
		problem string
	}{
		{"a child", access(model.Target{Scope: model.ScopeOrg, OrgId: "A"}), ""},
		{"a child with its subtree", access(model.Target{Scope: model.ScopeOrgSubtree, OrgId: "A"}), ""},
		{"a grandchild with its parent", access(model.Target{Scope: model.ScopeOrg, OrgId: "A"},
			model.Target{Scope: model.ScopeOrg, OrgId: "A1"}), "'A1' is not a direct child"},
		{"a grandchild below a selected subtree", access(model.Target{Scope: model.ScopeOrgSubtree, OrgId: "A"},
			model.Target{Scope: model.ScopeOrg, OrgId: "A1x"}), "'A1x' is not a direct child"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := ValidatePolicy(testTree(), c.policy)
			if c.problem == "" {
				if len(problems) != 0 {
					t.Fatalf("expected no problems, got %v", problems)
				}
				return
			}
			if !strings.Contains(strings.Join(problems, ";"), c.problem) {
				t.Fatalf("expected a problem with %q, got %v", c.problem, problems)
			}
		})
	}
	got := Reach(testTree(), access(model.Target{Scope: model.ScopeOrg, OrgId: "A"},
		model.Target{Scope: model.ScopeOrgSubtree, OrgId: "B"}))
	if expected := []string{"A", "B"}; !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

// An application data attribute is active in each organization that its policy reaches.
func TestEvaluateAppDataFollowsThePolicyOnly(t *testing.T) {

	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "cart", Name: "application_data.cart", ValueType: "string", OwnerOrgId: "R", AppId: "shop"},
		},
		Policies: []model.Policy{policy(model.ResourceSchemaAttribute, "cart", "R", allChildren("R"))},
	}
	states := statesByOrg(Evaluate(in), "cart")
	for _, org := range []string{"A", "A1", "B", "A1x"} {
		if states[org].State != model.StateActive {
			t.Errorf("expected ACTIVE in %s, got %+v", org, states[org])
		}
	}
}

// Two applications can use the same application data attribute name. A name conflicts only in one
// application.
func TestEvaluateAppDataNamesAreScopedToTheApp(t *testing.T) {

	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "shop-cart", Name: "application_data.cart", ValueType: "string", OwnerOrgId: "R", AppId: "shop"},
			{Id: "a-other-cart", Name: "application_data.cart", ValueType: "string", OwnerOrgId: "A", AppId: "other"},
			{Id: "b-shop-cart", Name: "application_data.cart", ValueType: "string", OwnerOrgId: "B", AppId: "shop"},
		},
		Policies: []model.Policy{policy(model.ResourceSchemaAttribute, "shop-cart", "R", allChildren("R"))},
	}
	states := statesByOrg(Evaluate(in), "shop-cart")
	if states["A"].State != model.StateActive {
		t.Errorf("expected ACTIVE in A, where the local attribute belongs to another app, got %+v", states["A"])
	}
	if states["B"].State != model.StateConflicted || states["B"].ConflictingResourceId != "b-shop-cart" {
		t.Errorf("expected a local conflict in B, where the local attribute belongs to the same app, got %+v",
			states["B"])
	}
}

// A shared rule on an inherited identity attribute is active in the sub organizations.
func TestEvaluateRuleOnInheritedIdentityAttribute(t *testing.T) {

	in := EngineInput{
		Orgs: testTree(),
		Attributes: []AttributeInfo{
			{Id: "email", Name: "identity_attributes.emailaddress", ValueType: "string", OwnerOrgId: "R"},
			{Id: "tier", Name: "traits.tier", ValueType: "string", OwnerOrgId: "R"},
		},
		Rules: []RuleInfo{
			{Id: "email-rule", PropertyName: "identity_attributes.emailaddress", PropertyId: "email", OwnerOrgId: "R"},
			{Id: "tier-rule", PropertyName: "traits.tier", PropertyId: "tier", OwnerOrgId: "R"},
		},
		Policies: []model.Policy{
			policy(model.ResourceUnificationRule, "email-rule", "R", allChildren("R")),
			policy(model.ResourceUnificationRule, "tier-rule", "R", allChildren("R")),
		},
		IdentitySourceOrgId: "R",
	}
	for org, s := range statesByOrg(Evaluate(in), "email-rule") {
		if s.State != model.StateActive {
			t.Errorf("expected ACTIVE in %s, which inherits the identity attribute, got %+v", org, s)
		}
	}
	// Only identity attributes are inherited. A trait needs a share.
	for org, s := range statesByOrg(Evaluate(in), "tier-rule") {
		if s.State != model.StateInactiveMissingAttribute {
			t.Errorf("expected INACTIVE_MISSING_ATTRIBUTE in %s for a trait that is not shared, got %+v", org, s)
		}
	}
	// Without a source organization, the sub organizations see no identity attributes.
	in.IdentitySourceOrgId = ""
	for org, s := range statesByOrg(Evaluate(in), "email-rule") {
		if s.State != model.StateInactiveMissingAttribute {
			t.Errorf("expected INACTIVE_MISSING_ATTRIBUTE in %s with no source org, got %+v", org, s)
		}
	}
}
