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
	"fmt"
	"sort"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/sharing/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// The engine computes the share state of shared resources in an organization tree. It has no side
// effects.

// TreeOrg is an organization in the tree. The engine computes levels from the parent links.
type TreeOrg struct {
	Id       string
	Handle   string
	ParentId string
	Active   bool
	// NotEnabled is true when the organization access does not include the organization. A share
	// does not apply in such an organization, but it applies to the organizations below it.
	NotEnabled bool
}

// AttributeInfo is a profile schema attribute. AppId is set only for application data attributes.
type AttributeInfo struct {
	Id         string
	Name       string
	ValueType  string
	OwnerOrgId string
	AppId      string
}

// nameKey returns the key for name conflicts. Application data attribute names are scoped to their
// application.
func (a AttributeInfo) nameKey() string {

	if a.AppId == "" {
		return a.Name
	}
	return a.AppId + "\x00" + a.Name
}

// RuleInfo is a unification rule.
type RuleInfo struct {
	Id           string
	PropertyName string
	PropertyId   string
	OwnerOrgId   string
}

// EngineInput is the input of the engine for one tree. IdentitySourceOrgId is the organization
// whose identity attributes the other organizations inherit. When it is empty, no attributes are
// inherited.
type EngineInput struct {
	Orgs                []TreeOrg
	Attributes          []AttributeInfo
	Rules               []RuleInfo
	Policies            []model.Policy
	IdentitySourceOrgId string
}

type tree struct {
	byId     map[string]TreeOrg
	children map[string][]string
}

// level returns the depth of the organization in the tree.
func (t tree) level(orgId string) int {

	level := 0
	for id := t.byId[orgId].ParentId; id != "" && level < 64; id = t.byId[id].ParentId {
		if _, ok := t.byId[id]; !ok {
			break
		}
		level++
	}
	return level
}

// isTargetOf reports whether the policy reaches the organization through a named target.
func (t tree) isTargetOf(orgId string, p model.Policy) bool {
	return t.byId[orgId].ParentId == p.InitiatingOrgId
}

func newTree(orgs []TreeOrg) tree {

	t := tree{byId: map[string]TreeOrg{}, children: map[string][]string{}}
	for _, o := range orgs {
		t.byId[o.Id] = o
	}
	for _, o := range orgs {
		if o.ParentId != "" {
			t.children[o.ParentId] = append(t.children[o.ParentId], o.Id)
		}
	}
	for id := range t.children {
		sort.Strings(t.children[id])
	}
	return t
}

// subtree returns the organization and all organizations below it.
func (t tree) subtree(orgId string) []string {

	result := []string{orgId}
	for i := 0; i < len(result); i++ {
		result = append(result, t.children[result[i]]...)
	}
	return result
}

// descendants returns all organizations below the organization.
func (t tree) descendants(orgId string) []string {
	return t.subtree(orgId)[1:]
}

// Reach returns the active, enabled organizations that the policy reaches, sorted by level and ID.
func Reach(orgs []TreeOrg, p model.Policy) []string {
	return reach(newTree(orgs), p)
}

func reach(t tree, p model.Policy) []string {

	reached := map[string]bool{}
	for _, target := range p.Targets {
		switch target.Scope {
		case model.ScopeAllChildren:
			for _, id := range t.descendants(p.InitiatingOrgId) {
				reached[id] = true
			}
		case model.ScopeOrg:
			if t.isTargetOf(target.OrgId, p) {
				reached[target.OrgId] = true
			}
		case model.ScopeOrgSubtree:
			if t.isTargetOf(target.OrgId, p) {
				for _, id := range t.subtree(target.OrgId) {
					reached[id] = true
				}
			}
		}
	}
	result := make([]string, 0, len(reached))
	for id := range reached {
		if org, ok := t.byId[id]; ok && org.Active && !org.NotEnabled {
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := t.level(result[i]), t.level(result[j])
		if a != b {
			return a < b
		}
		return result[i] < result[j]
	})
	return result
}

// ValidatePolicy checks a policy against the tree and returns the problems. The result is empty
// when the policy is valid.
func ValidatePolicy(orgs []TreeOrg, p model.Policy) []string {

	t := newTree(orgs)
	var problems []string
	initiator, ok := t.byId[p.InitiatingOrgId]
	if !ok {
		return []string{fmt.Sprintf("the initiating organization '%s' is not known", p.InitiatingOrgId)}
	}
	if len(p.Targets) == 0 {
		problems = append(problems, "at least one target is required")
	}
	for _, target := range p.Targets {
		switch target.Scope {
		case model.ScopeAllChildren:
			if target.OrgId != initiator.Id {
				problems = append(problems, "the ALL_CHILDREN scope must name the initiating organization")
			}
		case model.ScopeOrg, model.ScopeOrgSubtree:
			org, known := t.byId[target.OrgId]
			switch {
			case target.OrgId == "":
				problems = append(problems, fmt.Sprintf("the %s scope requires an org_id", target.Scope))
			case !known:
				problems = append(problems, fmt.Sprintf("the organization '%s' is not known", target.OrgId))
			case !t.isTargetOf(target.OrgId, p):
				problems = append(problems, fmt.Sprintf(
					"the organization '%s' is not a direct child of the initiating organization", target.OrgId))
			case !org.Active:
				problems = append(problems, fmt.Sprintf("the organization '%s' is not active", target.OrgId))
			}
		default:
			problems = append(problems, fmt.Sprintf("the scope '%s' is not supported", target.Scope))
		}
	}
	return problems
}

// EvaluateOrg returns the state of every shared resource that reaches the organization.
func EvaluateOrg(in EngineInput, orgId string) []model.State {
	return evaluate(in, orgId)
}

func evaluate(in EngineInput, only string) []model.State {

	t := newTree(in.Orgs)
	attributesById := map[string]AttributeInfo{}
	localAttributes := map[string]map[string]AttributeInfo{} // org -> name key -> attribute
	for _, a := range in.Attributes {
		attributesById[a.Id] = a
		if localAttributes[a.OwnerOrgId] == nil {
			localAttributes[a.OwnerOrgId] = map[string]AttributeInfo{}
		}
		localAttributes[a.OwnerOrgId][a.nameKey()] = a
	}
	inherited := map[string]AttributeInfo{} // name -> identity attribute of the source org
	for key, a := range localAttributes[in.IdentitySourceOrgId] {
		if in.IdentitySourceOrgId != "" && strings.HasPrefix(a.Name, constants.IdentityAttributes+".") {
			inherited[key] = a
		}
	}
	rulesById := map[string]RuleInfo{}
	localRules := map[string]map[string]RuleInfo{} // org -> property name -> rule
	for _, r := range in.Rules {
		rulesById[r.Id] = r
		if localRules[r.OwnerOrgId] == nil {
			localRules[r.OwnerOrgId] = map[string]RuleInfo{}
		}
		localRules[r.OwnerOrgId][r.PropertyName] = r
	}

	var states []model.State

	// Precedence. For attributes, a local attribute wins, then the shared attribute of the nearest
	// owner. For rules, a local rule wins, then the shared rule of the farthest owner.
	policies := append([]model.Policy(nil), in.Policies...)
	sort.SliceStable(policies, func(i, j int) bool {
		pi, pj := policies[i], policies[j]
		if pi.ResourceType != pj.ResourceType {
			return pi.ResourceType < pj.ResourceType
		}
		a, b := t.level(pi.OwningOrgId), t.level(pj.OwningOrgId)
		if a != b {
			if pi.ResourceType == model.ResourceUnificationRule {
				return a < b
			}
			return a > b
		}
		return pi.PolicyId < pj.PolicyId
	})

	// Attributes first, because rule states depend on the effective schema.
	activeShared := map[string]map[string]AttributeInfo{} // org -> name key -> active shared attribute
	for _, p := range policies {
		if p.ResourceType != model.ResourceSchemaAttribute {
			continue
		}
		attr, ok := attributesById[p.ResourceId]
		if !ok {
			continue
		}
		for _, orgId := range reach(t, p) {
			if only != "" && orgId != only {
				continue
			}
			state := model.State{ResourceType: p.ResourceType, ResourceId: p.ResourceId, OrgId: orgId,
				OrgHandle: t.byId[orgId].Handle, State: model.StateActive}
			key := attr.nameKey()
			if local, exists := localAttributes[orgId][key]; exists {
				state.State, state.Reason, state.ConflictingResourceId = model.StateConflicted,
					model.ReasonLocalNameConflict, local.Id
			} else if other, exists := activeShared[orgId][key]; exists && other.Id != attr.Id {
				state.State, state.Reason, state.ConflictingResourceId = model.StateConflicted,
					model.ReasonSharedNameConflict, other.Id
			} else {
				if activeShared[orgId] == nil {
					activeShared[orgId] = map[string]AttributeInfo{}
				}
				activeShared[orgId][key] = attr
			}
			states = append(states, state)
		}
	}

	activeSharedRules := map[string]map[string]RuleInfo{} // org -> property name -> active shared rule
	for _, p := range policies {
		if p.ResourceType != model.ResourceUnificationRule {
			continue
		}
		rule, ok := rulesById[p.ResourceId]
		if !ok {
			continue
		}
		for _, orgId := range reach(t, p) {
			if only != "" && orgId != only {
				continue
			}
			state := model.State{ResourceType: p.ResourceType, ResourceId: p.ResourceId, OrgId: orgId,
				OrgHandle: t.byId[orgId].Handle, State: model.StateActive}
			if local, exists := localRules[orgId][rule.PropertyName]; exists {
				state.State, state.Reason, state.ConflictingResourceId = model.StateConflicted,
					model.ReasonLocalNameConflict, local.Id
			} else if other, exists := activeSharedRules[orgId][rule.PropertyName]; exists && other.Id != rule.Id {
				state.State, state.Reason, state.ConflictingResourceId = model.StateConflicted,
					model.ReasonSharedNameConflict, other.Id
			} else if !hasCompatibleAttribute(rule, attributesById, localAttributes[orgId], inherited,
				activeShared[orgId]) {
				state.State, state.Reason = model.StateInactiveMissingAttribute, model.ReasonMissingAttribute
			} else {
				if activeSharedRules[orgId] == nil {
					activeSharedRules[orgId] = map[string]RuleInfo{}
				}
				activeSharedRules[orgId][rule.PropertyName] = rule
			}
			states = append(states, state)
		}
	}
	return states
}

// hasCompatibleAttribute reports whether the organization sees an attribute with the name and the
// value type of the rule property.
func hasCompatibleAttribute(rule RuleInfo, attributesById map[string]AttributeInfo,
	local, inherited, shared map[string]AttributeInfo) bool {

	candidate, ok := local[rule.PropertyName]
	if !ok {
		candidate, ok = inherited[rule.PropertyName]
	}
	if !ok {
		candidate, ok = shared[rule.PropertyName]
	}
	if !ok || candidate.ValueType == constants.ComplexDataType {
		return false
	}
	ownerAttribute, known := attributesById[rule.PropertyId]
	if !known {
		return true
	}
	return strings.EqualFold(ownerAttribute.ValueType, candidate.ValueType)
}
