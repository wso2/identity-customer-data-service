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
	"strings"

	"github.com/google/uuid"
	adminConfigStore "github.com/wso2/identity-customer-data-service/internal/admin_config/store"
	"github.com/wso2/identity-customer-data-service/internal/organization/model"
	"github.com/wso2/identity-customer-data-service/internal/organization/store"
	shareModel "github.com/wso2/identity-customer-data-service/internal/sharing/model"
	sharingService "github.com/wso2/identity-customer-data-service/internal/sharing/service"
	shareStore "github.com/wso2/identity-customer-data-service/internal/sharing/store"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// The organization access of a primary org selects the sub orgs that can use CDS. It is a policy
// with resource type ORGANIZATION_ACCESS and resource ID CDS, and a primary org has a maximum of
// one. Without it, no sub org can use CDS. child_orgs.orgs names direct children of the primary
// org.

// IsSubOrgEnabled reports whether the organization access of the primary org of the org reaches it.
// The caller checks the enablement of the primary org.
func IsSubOrgEnabled(ctx context.Context, orgHandle string) bool {

	org, err := store.GetOrganizationByHandle(ctx, orgHandle)
	if err != nil || org == nil {
		return false
	}
	enabled, err := sharingService.IsOrgEnabled(ctx, *org)
	return err == nil && enabled
}

// CreateOrgAccess creates the organization access of the primary org.
func CreateOrgAccess(ctx context.Context, primary model.Organization,
	req shareModel.PolicyRequest) (*shareModel.OrgAccessResponse, error) {

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
	orgs, err := validateOrgAccess(ctx, primary, p)
	if err != nil {
		return nil, err
	}
	if err := shareStore.CreatePolicy(ctx, p); err != nil {
		return nil, err
	}
	initializeSelected(ctx, orgs, p)
	resp := toOrgAccessResponse(p)
	return &resp, nil
}

// ListOrgAccess returns the organization access policies of the primary org. The list has a maximum
// of one policy.
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

// GetOrgAccess returns the organization access policy of the path.
func GetOrgAccess(ctx context.Context, primary model.Organization,
	policyId string) (*shareModel.OrgAccessResponse, error) {

	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return nil, err
	}
	resp := toOrgAccessResponse(*p)
	return &resp, nil
}

// UpdateOrgAccess replaces the selected orgs. An update cannot change the mode of child_orgs.
func UpdateOrgAccess(ctx context.Context, primary model.Organization, policyId string,
	req shareModel.PolicyRequest) (*shareModel.OrgAccessResponse, error) {

	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return nil, err
	}
	targets, problems := sharingService.ToTargets(req.ChildOrgs, primary.OrgId)
	if len(problems) > 0 {
		return nil, sharingService.BadRequest(strings.Join(problems, "; "))
	}
	wasBlanket, isBlanket := sharingService.IsBlanket(p.Targets), sharingService.IsBlanket(targets)
	switch {
	case wasBlanket && isBlanket:
		resp := toOrgAccessResponse(*p)
		return &resp, nil
	case wasBlanket != isBlanket:
		return nil, sharingService.BadRequest("An update cannot change child_orgs.mode. Delete the policy, and " +
			"then create a new one.")
	}
	p.Targets = targets
	orgs, err := validateOrgAccess(ctx, primary, *p)
	if err != nil {
		return nil, err
	}
	if err := shareStore.ReplaceTargets(ctx, p.PolicyId, targets); err != nil {
		return nil, err
	}
	initializeSelected(ctx, orgs, *p)
	resp := toOrgAccessResponse(*p)
	return &resp, nil
}

// DeleteOrgAccess deletes the organization access of the primary org. No sub org can use CDS after
// it. CDS keeps the data of the sub orgs.
func DeleteOrgAccess(ctx context.Context, primary model.Organization, policyId string) error {

	p, err := orgAccessById(ctx, primary, policyId)
	if err != nil {
		return err
	}
	return shareStore.DeletePolicy(ctx, p.PolicyId)
}

func orgAccessOf(ctx context.Context, primary model.Organization) (*shareModel.Policy, error) {
	return shareStore.GetPolicy(ctx, shareModel.ResourceOrganizationAccess, shareModel.OrganizationAccessResourceId,
		primary.OrgId)
}

// orgAccessById returns the organization access policy of the primary org. A policy of another
// primary org, or a share policy, is not found.
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

func validateOrgAccess(ctx context.Context, primary model.Organization,
	p shareModel.Policy) ([]model.Organization, error) {

	orgs, err := store.GetOrganizationsByPrimaryOrg(ctx, primary.OrgId)
	if err != nil {
		return nil, err
	}
	if problems := sharingService.ValidatePolicy(treeOf(orgs), p); len(problems) > 0 {
		return nil, sharingService.BadRequest(strings.Join(problems, "; "))
	}
	return orgs, nil
}

// initializeSelected initializes each org that the selection reaches and that is not initialized
// yet.
func initializeSelected(ctx context.Context, orgs []model.Organization, p shareModel.Policy) {

	byId := map[string]model.Organization{}
	for _, o := range orgs {
		byId[o.OrgId] = o
	}
	for _, id := range sharingService.Reach(treeOf(orgs), p) {
		org := byId[id]
		config, err := adminConfigStore.GetAdminConfig(ctx, org.OrgHandle)
		if err != nil {
			log.GetLogger().Warn(fmt.Sprintf("Failed to read the configuration of organization '%s'.",
				org.OrgHandle), log.Error(err))
			continue
		}
		if config == nil || !config.InitialSchemaSyncDone {
			initializeOrg(ctx, org)
		}
	}
}

func treeOf(orgs []model.Organization) []sharingService.TreeOrg {

	result := make([]sharingService.TreeOrg, 0, len(orgs))
	for _, o := range orgs {
		result = append(result, sharingService.TreeOrg{Id: o.OrgId, Handle: o.OrgHandle, ParentId: o.ParentOrgId,
			Active: o.Status == model.StatusActive})
	}
	return result
}

func toOrgAccessResponse(p shareModel.Policy) shareModel.OrgAccessResponse {
	return shareModel.OrgAccessResponse{
		Id:              p.PolicyId,
		OwningOrgId:     p.OwningOrgId,
		InitiatingOrgId: p.InitiatingOrgId,
		ChildOrgs:       sharingService.ToChildOrgs(p.Targets),
	}
}
