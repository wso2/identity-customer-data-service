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
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/organization/model"
	"github.com/wso2/identity-customer-data-service/internal/organization/store"
	shareModel "github.com/wso2/identity-customer-data-service/internal/sharing/model"
	sharingService "github.com/wso2/identity-customer-data-service/internal/sharing/service"
	shareStore "github.com/wso2/identity-customer-data-service/internal/sharing/store"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// The organization access of a primary organization selects the sub organizations that can use CDS.
// A primary organization has at most one organization access policy.

// IsSubOrgEnabled reports whether the organization access of the primary organization includes the
// organization.
func IsSubOrgEnabled(ctx context.Context, orgHandle string) bool {

	org, err := store.GetOrganizationByHandle(ctx, orgHandle)
	if err != nil || org == nil {
		return false
	}
	enabled, err := sharingService.IsOrgEnabled(ctx, *org)
	return err == nil && enabled
}

// CreateOrgAccess creates the organization access of the primary organization.
func CreateOrgAccess(ctx context.Context, primary model.Organization,
	req shareModel.PolicyRequest) (*shareModel.OrgAccessResponse, error) {

	if err := requireSyncedTree(ctx, primary); err != nil {
		return nil, err
	}
	existing, err := orgAccessOf(ctx, primary)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, sharingService.PolicyExistsError(existing.PolicyId)
	}
	targets, problems := sharingService.ToTargets(req.ChildOrgs, primary.OrgId)
	if len(problems) > 0 {
		return nil, sharingService.BadRequest(strings.Join(problems, "; "))
	}
	p := shareModel.Policy{
		PolicyId:        uuid.New().String(),
		ResourceType:    shareModel.ResourceOrganizationAccess,
		ResourceId:      shareModel.OrganizationAccessResourceId,
		OwningOrgId:     primary.OrgId,
		InitiatingOrgId: primary.OrgId,
		Targets:         targets,
	}
	if err := sharingService.ValidateTargets(ctx, primary, p); err != nil {
		return nil, err
	}
	if err := shareStore.CreatePolicy(ctx, p); err != nil {
		return nil, err
	}
	resp := toOrgAccessResponse(p)
	return &resp, nil
}

// ListOrgAccess returns the organization access policies of the primary organization.
func ListOrgAccess(ctx context.Context, primary model.Organization) (*shareModel.OrgAccessList, error) {

	list := &shareModel.OrgAccessList{Policies: []shareModel.OrgAccessResponse{}}
	p, err := orgAccessOf(ctx, primary)
	if err != nil {
		return nil, err
	}
	if p != nil {
		list.Policies = append(list.Policies, toOrgAccessResponse(*p))
	}
	list.TotalResults = len(list.Policies)
	return list, nil
}

// GetOrgAccess returns an organization access policy of the primary organization.
func GetOrgAccess(ctx context.Context, primary model.Organization,
	policyId string) (*shareModel.OrgAccessResponse, error) {

	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return nil, err
	}
	resp := toOrgAccessResponse(*p)
	return &resp, nil
}

// UpdateOrgAccess replaces the selected organizations of an organization access policy.
func UpdateOrgAccess(ctx context.Context, primary model.Organization, policyId string,
	req shareModel.PolicyRequest) (*shareModel.OrgAccessResponse, error) {

	if err := requireSyncedTree(ctx, primary); err != nil {
		return nil, err
	}
	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return nil, err
	}
	targets, problems := sharingService.ToTargets(req.ChildOrgs, primary.OrgId)
	if len(problems) > 0 {
		return nil, sharingService.BadRequest(strings.Join(problems, "; "))
	}
	wasAll, isAll := sharingService.IsAllChildren(p.Targets), sharingService.IsAllChildren(targets)
	switch {
	case wasAll && isAll:
		resp := toOrgAccessResponse(*p)
		return &resp, nil
	case wasAll != isAll:
		return nil, sharingService.BadRequest("An update cannot change child_orgs.mode. Delete the policy, and " +
			"then create a new one.")
	}
	p.Targets = targets
	if err := sharingService.ValidateTargets(ctx, primary, *p); err != nil {
		return nil, err
	}
	if err := shareStore.ReplaceTargets(ctx, p.PolicyId, targets); err != nil {
		return nil, err
	}
	resp := toOrgAccessResponse(*p)
	return &resp, nil
}

// DeleteOrgAccess deletes the organization access of the primary organization.
func DeleteOrgAccess(ctx context.Context, primary model.Organization, policyId string) error {

	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return err
	}
	return shareStore.DeletePolicy(ctx, p.PolicyId)
}

// requireSyncedTree returns an error until the organization tree is synced at least once.
func requireSyncedTree(ctx context.Context, primary model.Organization) error {

	status, err := store.GetTreeSyncStatus(ctx, primary.OrgHandle)
	if err != nil {
		return err
	}
	if status.LastSyncedAt == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ORGANIZATIONS_NOT_SYNCED.Code,
			Message:     errors2.ORGANIZATIONS_NOT_SYNCED.Message,
			Description: errors2.ORGANIZATIONS_NOT_SYNCED.Description,
		}, http.StatusBadRequest)
	}
	return nil
}

func orgAccessOf(ctx context.Context, primary model.Organization) (*shareModel.Policy, error) {
	return shareStore.GetPolicy(ctx, shareModel.ResourceOrganizationAccess, shareModel.OrganizationAccessResourceId,
		primary.OrgId)
}

// orgAccessById returns the organization access policy with the ID when it belongs to the primary
// organization.
func orgAccessById(ctx context.Context, primary model.Organization, policyId string) (*shareModel.Policy, error) {

	p, err := shareStore.GetPolicyById(ctx, policyId)
	if err != nil {
		return nil, err
	}
	if p == nil || p.ResourceType != shareModel.ResourceOrganizationAccess || p.InitiatingOrgId != primary.OrgId {
		return nil, sharingService.PolicyNotFoundError(policyId)
	}
	return p, nil
}

func toOrgAccessResponse(p shareModel.Policy) shareModel.OrgAccessResponse {
	return shareModel.OrgAccessResponse{
		Id:              p.PolicyId,
		OwningOrgId:     p.OwningOrgId,
		InitiatingOrgId: p.InitiatingOrgId,
		ChildOrgs:       sharingService.ToChildOrgs(p.Targets),
	}
}
