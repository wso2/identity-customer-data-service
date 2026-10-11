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

// Resource types of a policy.
const (
	ResourceSchemaAttribute = "SCHEMA_ATTRIBUTE"
	ResourceUnificationRule = "UNIFICATION_RULE"
	// ResourceOrganizationAccess is the policy that selects the sub orgs of a primary org that can
	// use CDS. Its resource ID is always OrganizationAccessResourceId.
	ResourceOrganizationAccess   = "ORGANIZATION_ACCESS"
	OrganizationAccessResourceId = "CDS"
)

// Stored target scopes.
const (
	// ScopeAllChildren reaches all current and future orgs below the initiating org. The target
	// row stores the initiating org.
	ScopeAllChildren = "ALL_CHILDREN"
	// ScopeOrg reaches one direct child of the initiating org.
	ScopeOrg = "ORG"
	// ScopeOrgSubtree reaches one direct child and all current and future orgs below it.
	ScopeOrgSubtree = "ORG_SUBTREE"
)

// Share states of a resource in one org.
const (
	StateActive                   = "ACTIVE"
	StateConflicted               = "CONFLICTED"
	StateInactiveMissingAttribute = "INACTIVE_MISSING_ATTRIBUTE"
)

// Reasons for a state that is not ACTIVE.
const (
	ReasonLocalNameConflict  = "LOCAL_NAME_CONFLICT"
	ReasonSharedNameConflict = "SHARED_NAME_CONFLICT"
	ReasonMissingAttribute   = "MISSING_ATTRIBUTE"
)

// Origin of a resource in the view of an org.
const (
	OriginOwned  = "OWNED"
	OriginShared = "SHARED"
)

// Target is one stored target of a policy.
type Target struct {
	Scope string
	OrgId string
}

// Policy says which orgs can see one resource. There is one policy for each resource and
// initiating org.
type Policy struct {
	PolicyId        string
	ResourceType    string
	ResourceId      string
	OwningOrgId     string
	InitiatingOrgId string
	Targets         []Target
}

// State is the result of the policies for one resource in one org.
type State struct {
	ResourceType          string `json:"-"`
	ResourceId            string `json:"-"`
	OrgId                 string `json:"org_id"`
	OrgHandle             string `json:"org_handle,omitempty"`
	State                 string `json:"state"`
	Reason                string `json:"reason,omitempty"`
	ConflictingResourceId string `json:"conflicting_resource_id,omitempty"`
}

// Modes of child_orgs.
const (
	// ChildOrgsModeAll reaches all current and future orgs below the initiating org.
	ChildOrgsModeAll = "all"
	// ChildOrgsModeSelected reaches the orgs that orgs names.
	ChildOrgsModeSelected = "selected"
)

// Reaches of a selected org.
const (
	// ReachOrg reaches the selected org only.
	ReachOrg = "org"
	// ReachSubtree reaches the selected org and all current and future orgs below it.
	ReachSubtree = "subtree"
)

// ChildOrg is one selected org of child_orgs, with its reach.
type ChildOrg struct {
	OrgId string `json:"org_id"`
	Reach string `json:"reach"`
}

// ChildOrgs is the reach of a policy in the API. Mode states the decision: all orgs below the
// initiating org, or the selected orgs.
type ChildOrgs struct {
	Mode string     `json:"mode"`
	Orgs []ChildOrg `json:"orgs,omitempty"`
}

// PolicyRequest is the body of a POST or a PUT on a sharing policy.
type PolicyRequest struct {
	ChildOrgs *ChildOrgs `json:"child_orgs"`
}

// PolicyResponse is a sharing policy in the API.
type PolicyResponse struct {
	Id              string    `json:"id"`
	ResourceType    string    `json:"resource_type"`
	ResourceId      string    `json:"resource_id"`
	OwningOrgId     string    `json:"owning_org_id"`
	InitiatingOrgId string    `json:"initiating_org_id"`
	ChildOrgs       ChildOrgs `json:"child_orgs"`
}

// PolicyList is the list of the policies of a resource.
type PolicyList struct {
	TotalResults int              `json:"total_results"`
	Policies     []PolicyResponse `json:"policies"`
}

// OrgAccessResponse is the organization access policy of a primary org in the API.
type OrgAccessResponse struct {
	Id              string    `json:"id"`
	OwningOrgId     string    `json:"owning_org_id"`
	InitiatingOrgId string    `json:"initiating_org_id"`
	ChildOrgs       ChildOrgs `json:"child_orgs"`
}

// OrgAccessList is the list of the organization access policies of a primary org.
type OrgAccessList struct {
	TotalResults int                 `json:"total_results"`
	Policies     []OrgAccessResponse `json:"policies"`
}
