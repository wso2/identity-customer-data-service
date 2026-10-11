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
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/sharing/model"
)

// ToTargets maps the child_orgs of a request to the stored targets. It returns the problems of
// the request, which are empty when the request is valid.
func ToTargets(childOrgs *model.ChildOrgs, initiatingOrgId string) ([]model.Target, []string) {

	if childOrgs == nil {
		return nil, []string{"child_orgs is required"}
	}
	switch childOrgs.Mode {
	case "":
		return nil, []string{"child_orgs.mode is required"}
	case model.ChildOrgsModeAll:
		if len(childOrgs.Orgs) > 0 {
			return nil, []string{"child_orgs.orgs must be empty when child_orgs.mode is all"}
		}
		return []model.Target{{Scope: model.ScopeAllChildren, OrgId: initiatingOrgId}}, nil
	case model.ChildOrgsModeSelected:
		if len(childOrgs.Orgs) == 0 {
			return nil, []string{"child_orgs.orgs must have at least one organization when child_orgs.mode is " +
				"selected"}
		}
	default:
		return nil, []string{fmt.Sprintf("child_orgs.mode '%s' is not valid. Use all or selected",
			childOrgs.Mode)}
	}

	var problems []string
	seen := map[string]bool{}
	targets := make([]model.Target, 0, len(childOrgs.Orgs))
	for _, child := range childOrgs.Orgs {
		orgId := strings.TrimSpace(child.OrgId)
		if orgId == "" {
			problems = append(problems, "each child_orgs.orgs entry requires an org_id")
			continue
		}
		if seen[orgId] {
			problems = append(problems, fmt.Sprintf("the organization '%s' is in child_orgs.orgs more than once",
				orgId))
			continue
		}
		seen[orgId] = true
		var scope string
		switch child.Reach {
		case model.ReachOrg:
			scope = model.ScopeOrg
		case model.ReachSubtree:
			scope = model.ScopeOrgSubtree
		case "":
			problems = append(problems, fmt.Sprintf("the organization '%s' requires a reach", orgId))
			continue
		default:
			problems = append(problems, fmt.Sprintf("the reach '%s' of the organization '%s' is not valid. "+
				"Use org or subtree", child.Reach, orgId))
			continue
		}
		targets = append(targets, model.Target{Scope: scope, OrgId: orgId})
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return targets, nil
}

// ToChildOrgs maps the stored targets to the child_orgs of a response.
func ToChildOrgs(targets []model.Target) model.ChildOrgs {

	if IsBlanket(targets) {
		return model.ChildOrgs{Mode: model.ChildOrgsModeAll}
	}
	childOrgs := model.ChildOrgs{Mode: model.ChildOrgsModeSelected}
	for _, t := range targets {
		switch t.Scope {
		case model.ScopeOrg:
			childOrgs.Orgs = append(childOrgs.Orgs, model.ChildOrg{OrgId: t.OrgId, Reach: model.ReachOrg})
		case model.ScopeOrgSubtree:
			childOrgs.Orgs = append(childOrgs.Orgs, model.ChildOrg{OrgId: t.OrgId, Reach: model.ReachSubtree})
		}
	}
	return childOrgs
}

// IsBlanket reports whether the targets are the all mode of child_orgs.
func IsBlanket(targets []model.Target) bool {

	for _, t := range targets {
		if t.Scope == model.ScopeAllChildren {
			return true
		}
	}
	return false
}
