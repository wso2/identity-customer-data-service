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
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	orgModel "github.com/wso2/identity-customer-data-service/internal/organization/model"
	orgStore "github.com/wso2/identity-customer-data-service/internal/organization/store"
	"github.com/wso2/identity-customer-data-service/internal/organization/suborg"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/identitysource"
	"github.com/wso2/identity-customer-data-service/internal/sharing/model"
	"github.com/wso2/identity-customer-data-service/internal/sharing/store"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// The share state is not stored. It is computed on each read.

// orgView is the result of the read for one organization.
type orgView struct {
	states     []model.State
	attributes map[string]store.ChainAttribute
	rules      map[string]store.ChainRule
}

// resolveOrg computes the state of each shared resource that reaches the organization.
func resolveOrg(ctx context.Context, org orgModel.Organization) (*orgView, error) {

	view := &orgView{attributes: map[string]store.ChainAttribute{}, rules: map[string]store.ChainRule{}}
	if enabled, err := IsOrgEnabled(ctx, org); err != nil || !enabled {
		return view, err
	}
	policies, err := store.GetPoliciesReachingOrg(ctx, org.OrgId)
	if err != nil || len(policies) == 0 {
		return view, err
	}
	chain, err := orgStore.GetOrganizationChain(ctx, org.OrgId)
	if err != nil {
		return nil, err
	}
	attrs, err := store.GetAttributesOfChain(ctx, org.OrgId)
	if err != nil {
		return nil, err
	}
	rules, err := store.GetRulesOfChain(ctx, org.OrgId)
	if err != nil {
		return nil, err
	}

	input := EngineInput{Orgs: treeOrgs(chain), Policies: policies,
		IdentitySourceOrgId: identitySourceOrgId(ctx, chain, org.OrgHandle)}
	for _, a := range attrs {
		view.attributes[a.Attribute.AttributeId] = a
		input.Attributes = append(input.Attributes, AttributeInfo{Id: a.Attribute.AttributeId,
			Name: a.Attribute.AttributeName, ValueType: a.Attribute.ValueType, OwnerOrgId: a.OwnerOrgId,
			AppId: appIdOf(a.Attribute.Scope, a.Attribute.ApplicationIdentifier)})
	}
	for _, r := range rules {
		view.rules[r.Rule.RuleId] = r
		input.Rules = append(input.Rules, RuleInfo{Id: r.Rule.RuleId, PropertyName: r.Rule.PropertyName,
			PropertyId: r.Rule.PropertyId, OwnerOrgId: r.OwnerOrgId})
	}
	view.states = EvaluateOrg(input, org.OrgId)
	return view, nil
}

// appIdOf returns the application identifier of an application data attribute, or empty.
func appIdOf(scope, applicationIdentifier string) string {

	if scope != constants.ApplicationData {
		return ""
	}
	return applicationIdentifier
}

// identitySourceOrgId returns the ID of the organization that holds the identity attributes of the
// organization.
func identitySourceOrgId(ctx context.Context, orgs []orgModel.Organization, orgHandle string) string {

	source := identitysource.HandleOf(ctx, orgHandle)
	for _, o := range orgs {
		if o.OrgHandle == source {
			return o.OrgId
		}
	}
	return ""
}

func treeOrgs(orgs []orgModel.Organization) []TreeOrg {

	result := make([]TreeOrg, 0, len(orgs))
	for _, o := range orgs {
		result = append(result, TreeOrg{Id: o.OrgId, Handle: o.OrgHandle, ParentId: o.ParentOrgId,
			Active: o.Status == orgModel.StatusActive})
	}
	return result
}

func viewOfHandle(ctx context.Context, orgHandle string) (*orgView, error) {

	org, err := orgStore.GetOrganizationByHandle(ctx, orgHandle)
	if err != nil || org == nil {
		return nil, err
	}
	return resolveOrg(ctx, *org)
}

// ActiveSharedAttributes returns the shared attributes that are active in the organization of the
// handle.
func ActiveSharedAttributes(ctx context.Context, orgHandle string) ([]store.SharedAttribute, error) {

	view, err := viewOfHandle(ctx, orgHandle)
	if err != nil || view == nil {
		return nil, err
	}
	var result []store.SharedAttribute
	for _, s := range view.states {
		if s.ResourceType == model.ResourceSchemaAttribute && s.State == model.StateActive {
			result = append(result, view.attributes[s.ResourceId].SharedAttribute)
		}
	}
	return result, nil
}

// ActiveSharedRules returns the active shared rules of the organization in evaluation order.
func ActiveSharedRules(ctx context.Context, orgHandle string) ([]store.SharedRule, error) {

	view, err := viewOfHandle(ctx, orgHandle)
	if err != nil || view == nil {
		return nil, err
	}
	var rules []store.SharedRule
	for _, s := range view.states {
		if s.ResourceType == model.ResourceUnificationRule && s.State == model.StateActive {
			rules = append(rules, view.rules[s.ResourceId].SharedRule)
		}
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].OwnerHops != rules[j].OwnerHops {
			return rules[i].OwnerHops > rules[j].OwnerHops
		}
		return rules[i].Rule.Priority < rules[j].Rule.Priority
	})
	return rules, nil
}

// StatesForOrg returns the states of the shared resources of one type in the organization of the
// handle.
func StatesForOrg(ctx context.Context, resourceType, orgHandle string) (map[string]model.State, error) {

	result := map[string]model.State{}
	view, err := viewOfHandle(ctx, orgHandle)
	if err != nil || view == nil {
		return result, err
	}
	for _, s := range view.states {
		if s.ResourceType == resourceType {
			result[s.ResourceId] = s
		}
	}
	return result, nil
}

// CreatePolicy creates the policy of the initiating organization for a resource that it owns.
func CreatePolicy(ctx context.Context, resourceType, resourceId string, initiator orgModel.Organization,
	req model.PolicyRequest) (*model.PolicyResponse, error) {

	if !initiator.IsPrimary() {
		return nil, suborg.WriteNotAllowed()
	}
	existing, err := store.GetPolicy(ctx, resourceType, resourceId, initiator.OrgId)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, PolicyExistsError(existing.PolicyId)
	}
	targets, problems := ToTargets(req.ChildOrgs, initiator.OrgId)
	if len(problems) > 0 {
		return nil, badRequest(errors2.SHARE_BAD_REQUEST, strings.Join(problems, "; "))
	}
	candidate := model.Policy{
		PolicyId:        uuid.New().String(),
		ResourceType:    resourceType,
		ResourceId:      resourceId,
		OwningOrgId:     initiator.OrgId,
		InitiatingOrgId: initiator.OrgId,
		Targets:         targets,
	}
	if err := ValidateTargets(ctx, initiator, candidate); err != nil {
		return nil, err
	}
	if err := store.CreatePolicy(ctx, candidate); err != nil {
		return nil, err
	}
	resp := ToResponse(candidate)
	return &resp, nil
}

// ListPolicies returns the policies of the initiating organization for the resource.
func ListPolicies(ctx context.Context, resourceType, resourceId string,
	initiator orgModel.Organization) (*model.PolicyList, error) {

	list := &model.PolicyList{Policies: []model.PolicyResponse{}}
	p, err := store.GetPolicy(ctx, resourceType, resourceId, initiator.OrgId)
	if err != nil {
		return nil, err
	}
	if p != nil {
		list.Policies = append(list.Policies, ToResponse(*p))
	}
	list.TotalResults = len(list.Policies)
	return list, nil
}

// GetPolicy returns the policy.
func GetPolicy(ctx context.Context, resourceType, resourceId, policyId string,
	initiator orgModel.Organization) (*model.PolicyResponse, error) {

	p, err := policyOf(ctx, resourceType, resourceId, policyId, initiator)
	if err != nil {
		return nil, err
	}
	resp := ToResponse(*p)
	return &resp, nil
}

// UpdatePolicy replaces the targets of the policy.
func UpdatePolicy(ctx context.Context, resourceType, resourceId, policyId string, initiator orgModel.Organization,
	req model.PolicyRequest) (*model.PolicyResponse, error) {

	if !initiator.IsPrimary() {
		return nil, suborg.WriteNotAllowed()
	}
	p, err := policyOf(ctx, resourceType, resourceId, policyId, initiator)
	if err != nil {
		return nil, err
	}
	targets, problems := ToTargets(req.ChildOrgs, initiator.OrgId)
	if len(problems) > 0 {
		return nil, badRequest(errors2.SHARE_BAD_REQUEST, strings.Join(problems, "; "))
	}
	p.Targets = targets
	if err := ValidateTargets(ctx, initiator, *p); err != nil {
		return nil, err
	}
	if err := store.ReplaceTargets(ctx, p.PolicyId, targets); err != nil {
		return nil, err
	}
	resp := ToResponse(*p)
	return &resp, nil
}

// DeletePolicy deletes the policy.
func DeletePolicy(ctx context.Context, resourceType, resourceId, policyId string,
	initiator orgModel.Organization) error {

	if !initiator.IsPrimary() {
		return suborg.WriteNotAllowed()
	}
	p, err := policyOf(ctx, resourceType, resourceId, policyId, initiator)
	if err != nil {
		return err
	}
	return store.DeletePolicy(ctx, p.PolicyId)
}

// DeletePoliciesOfResource deletes all policies of the resource. It does nothing when organization
// management is disabled.
func DeletePoliciesOfResource(ctx context.Context, resourceType, resourceId string) error {
	if !config.OrganizationManagementEnabled() {
		return nil
	}
	return store.DeletePoliciesOfResource(ctx, resourceType, resourceId)
}

// policyOf returns the policy with the ID when it belongs to the resource and the initiating
// organization.
func policyOf(ctx context.Context, resourceType, resourceId, policyId string,
	initiator orgModel.Organization) (*model.Policy, error) {

	p, err := store.GetPolicyById(ctx, policyId)
	if err != nil {
		return nil, err
	}
	if p == nil || p.ResourceType != resourceType || p.ResourceId != resourceId ||
		p.InitiatingOrgId != initiator.OrgId {
		return nil, PolicyNotFoundError(policyId)
	}
	return p, nil
}

// ValidateTargets checks the targets of the policy against the initiating organization and the
// target organizations.
func ValidateTargets(ctx context.Context, initiator orgModel.Organization, p model.Policy) error {

	orgs := []orgModel.Organization{initiator}
	if !IsAllChildren(p.Targets) {
		ids := make([]string, 0, len(p.Targets))
		for _, t := range p.Targets {
			ids = append(ids, t.OrgId)
		}
		targets, err := orgStore.GetOrganizationsByIds(ctx, ids)
		if err != nil {
			return err
		}
		orgs = append(orgs, targets...)
	}
	if problems := ValidatePolicy(treeOrgs(orgs), p); len(problems) > 0 {
		return BadRequest(strings.Join(problems, "; "))
	}
	return nil
}

// ToResponse maps a policy to the API.
func ToResponse(p model.Policy) model.PolicyResponse {
	return model.PolicyResponse{
		Id:              p.PolicyId,
		ResourceType:    p.ResourceType,
		ResourceId:      p.ResourceId,
		OwningOrgId:     p.OwningOrgId,
		InitiatingOrgId: p.InitiatingOrgId,
		ChildOrgs:       ToChildOrgs(p.Targets),
	}
}

// IsOrgEnabled reports whether the organization access includes the sub organization.
func IsOrgEnabled(ctx context.Context, org orgModel.Organization) (bool, error) {

	if org.IsPrimary() {
		return true, nil
	}
	policies, err := store.GetOrgAccessPoliciesReachingOrg(ctx, org.OrgId)
	if err != nil || len(policies) == 0 {
		return false, err
	}
	chain, err := orgStore.GetOrganizationChain(ctx, org.OrgId)
	if err != nil {
		return false, err
	}
	orgs := treeOrgs(chain)
	for _, p := range policies {
		for _, id := range Reach(orgs, p) {
			if id == org.OrgId {
				return true, nil
			}
		}
	}
	return false, nil
}

// PolicyExistsError is the error for a second policy of the same organization.
func PolicyExistsError(policyId string) error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:    errors2.SHARE_POLICY_EXISTS.Code,
		Message: errors2.SHARE_POLICY_EXISTS.Message,
		Description: fmt.Sprintf("The organization already has the policy '%s' for this resource. Use PUT on that "+
			"policy to change it.", policyId),
	}, http.StatusConflict)
}

// PolicyNotFoundError is the error for an unknown policy ID.
func PolicyNotFoundError(policyId string) error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.SHARE_POLICY_NOT_FOUND.Code,
		Message:     errors2.SHARE_POLICY_NOT_FOUND.Message,
		Description: fmt.Sprintf("The policy '%s' is not found for this resource and organization.", policyId),
	}, http.StatusNotFound)
}

// ConflictError is the error for a local resource whose name is used by an active shared
// resource.
func ConflictError(description string) error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.SHARED_NAME_CONFLICT.Code,
		Message:     errors2.SHARED_NAME_CONFLICT.Message,
		Description: description,
	}, http.StatusConflict)
}

// ReadOnlyError is the error for a write to a shared resource from a target organization.
func ReadOnlyError() error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.SHARED_RESOURCE_READ_ONLY.Code,
		Message:     errors2.SHARED_RESOURCE_READ_ONLY.Message,
		Description: errors2.SHARED_RESOURCE_READ_ONLY.Description,
	}, http.StatusForbidden)
}

// BadRequest is the error for a request that is not valid.
func BadRequest(description string) error {
	return badRequest(errors2.SHARE_BAD_REQUEST, description)
}

func badRequest(msg errors2.ErrorMessage, description string) error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        msg.Code,
		Message:     msg.Message,
		Description: description,
	}, http.StatusBadRequest)
}
