/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	adminConfigService "github.com/wso2/identity-customer-data-service/internal/admin_config/service"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/worker"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/security"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/provider"

	"github.com/google/uuid"
)

type UnificationRulesHandler struct {
	store map[string]model.UnificationRule
	mu    *sync.RWMutex
}

func NewUnificationRulesHandler() *UnificationRulesHandler {

	return &UnificationRulesHandler{
		store: make(map[string]model.UnificationRule),
		mu:    &sync.RWMutex{},
	}
}

// AddUnificationRule handles adding a new rule
func (urh *UnificationRulesHandler) AddUnificationRule(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:create")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	var ruleInRequest model.UnificationRuleAPIRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ruleInRequest); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: utils.HandleDecodeError(err, "unification rule"),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if !isCDSEnabled(orgHandle) {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}

	// Validate AttributeType.
	if !constants.AllowedAttributeTypes[ruleInRequest.AttributeType] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Invalid attribute_type: '%s'. Allowed values: PRIMITIVE_EXACT, FUZZY_STRING, NAME, EMAIL, PHONE, LOCATION, DATE, UNIQUE_ID.", ruleInRequest.AttributeType),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}

	// Validate UnificationMethod.
	if ruleInRequest.UnificationMethod != "fuzzy" && ruleInRequest.UnificationMethod != "deterministic" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Invalid unification_method: '%s'. Allowed values: fuzzy, deterministic.", ruleInRequest.UnificationMethod),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}

	// Reject fuzzy for attribute types that only support exact matching.
	if ruleInRequest.UnificationMethod == "fuzzy" && !constants.FuzzyCapableAttributeTypes[ruleInRequest.AttributeType] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Attribute type '%s' does not support fuzzy matching. Use 'deterministic' instead.", ruleInRequest.AttributeType),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}

	// Set timestamps
	now := time.Now().UTC()
	rule := model.UnificationRule{
		RuleId:            uuid.New().String(),
		OrgHandle:         orgHandle,
		RuleName:          ruleInRequest.RuleName,
		PropertyName:      ruleInRequest.PropertyName,
		Priority:          ruleInRequest.Priority,
		IsActive:          ruleInRequest.IsActive,
		AttributeType:     ruleInRequest.AttributeType,
		UnificationMethod: ruleInRequest.UnificationMethod,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	err = ruleService.AddUnificationRule(rule, orgHandle)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	addedRule, err := ruleService.GetUnificationRule(rule.RuleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	addedRuleResponse := model.UnificationRuleAPIResponse{
		RuleId:            addedRule.RuleId,
		RuleName:          addedRule.RuleName,
		PropertyName:      addedRule.PropertyName,
		Priority:          addedRule.Priority,
		IsActive:          addedRule.IsActive,
		AttributeType:     addedRule.AttributeType,
		UnificationMethod: addedRule.UnificationMethod,
	}

	// Trigger reindex if rule is active.
	if addedRule.IsActive {
		go worker.IndexNewAttribute(orgHandle, *addedRule)
	}

	utils.RespondJSON(w, http.StatusCreated, addedRuleResponse, constants.UnificationRuleResource)
}

// GetUnificationRules handles fetching all rules
func (urh *UnificationRulesHandler) GetUnificationRules(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if !isCDSEnabled(orgHandle) {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}
	rules, err := ruleService.GetUnificationRules(orgHandle)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	// Convert rules to API response format
	rulesResponse := make([]model.UnificationRuleAPIResponse, 0, len(rules))
	for _, rule := range rules {
		tempRule := model.UnificationRuleAPIResponse{
			RuleId:            rule.RuleId,
			RuleName:          rule.RuleName,
			PropertyName:      rule.PropertyName,
			Priority:          rule.Priority,
			IsActive:          rule.IsActive,
			AttributeType:     rule.AttributeType,
			UnificationMethod: rule.UnificationMethod,
		}
		rulesResponse = append(rulesResponse, tempRule)
	}
	utils.RespondJSON(w, http.StatusOK, rulesResponse, constants.UnificationRuleResource)
}

// GetUnificationRule Fetches a specific resolution rule.
func (urh *UnificationRulesHandler) GetUnificationRule(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if !isCDSEnabled(orgHandle) {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}
	ruleId := r.PathValue("ruleId")
	if ruleId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_UNIFICATION_RULE_WITHOUT_ID.Code,
			Message:     errors2.GET_UNIFICATION_RULE_WITHOUT_ID.Message,
			Description: "Invalid path for unification rule retrieval",
		}, http.StatusNotFound)
		utils.HandleError(w, clientError)
		return
	}
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	rule, err := ruleService.GetUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleResponse := model.UnificationRuleAPIResponse{
		RuleId:            rule.RuleId,
		RuleName:          rule.RuleName,
		PropertyName:      rule.PropertyName,
		Priority:          rule.Priority,
		IsActive:          rule.IsActive,
		AttributeType:     rule.AttributeType,
		UnificationMethod: rule.UnificationMethod,
	}
	utils.RespondJSON(w, http.StatusOK, ruleResponse, constants.UnificationRuleResource)
}

// PatchUnificationRule applies partial updates to a unification rule.
func (urh *UnificationRulesHandler) PatchUnificationRule(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:update")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleId := r.PathValue("ruleId")
	if ruleId == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if !isCDSEnabled(orgHandle) {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}
	var ruleUpdateRequest model.UnificationRuleUpdateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ruleUpdateRequest); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: utils.HandleDecodeError(err, "unification rule"),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()

	// Fetch old rule to detect is_active changes.
	oldRule, err := ruleService.GetUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	wasActive := oldRule.IsActive

	updatedRule := *oldRule

	if ruleUpdateRequest.RuleName != nil {
		updatedRule.RuleName = *ruleUpdateRequest.RuleName
	}

	if ruleUpdateRequest.Priority != nil {
		updatedRule.Priority = *ruleUpdateRequest.Priority
	}

	if ruleUpdateRequest.IsActive != nil {
		updatedRule.IsActive = *ruleUpdateRequest.IsActive
	}

	if ruleUpdateRequest.AttributeType != nil {
		if !constants.AllowedAttributeTypes[*ruleUpdateRequest.AttributeType] {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.BAD_REQUEST.Code,
				Message:     errors2.BAD_REQUEST.Message,
				Description: fmt.Sprintf("Invalid attribute_type: '%s'. Allowed values: PRIMITIVE_EXACT, FUZZY_STRING, NAME, EMAIL, PHONE, LOCATION, DATE, UNIQUE_ID.", *ruleUpdateRequest.AttributeType),
			}, http.StatusBadRequest)
			utils.WriteErrorResponse(w, clientError)
			return
		}
		updatedRule.AttributeType = *ruleUpdateRequest.AttributeType
	}

	if ruleUpdateRequest.UnificationMethod != nil {
		if *ruleUpdateRequest.UnificationMethod != "fuzzy" && *ruleUpdateRequest.UnificationMethod != "deterministic" {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.BAD_REQUEST.Code,
				Message:     errors2.BAD_REQUEST.Message,
				Description: fmt.Sprintf("Invalid unification_method: '%s'. Allowed values: fuzzy, deterministic.", *ruleUpdateRequest.UnificationMethod),
			}, http.StatusBadRequest)
			utils.WriteErrorResponse(w, clientError)
			return
		}
		updatedRule.UnificationMethod = *ruleUpdateRequest.UnificationMethod
	}

	// Cross-validate: reject fuzzy for attribute types that only support exact matching.
	if updatedRule.UnificationMethod == "fuzzy" && !constants.FuzzyCapableAttributeTypes[updatedRule.AttributeType] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Attribute type '%s' does not support fuzzy matching. Use 'deterministic' instead.", updatedRule.AttributeType),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}

	err = ruleService.PatchUnificationRule(ruleId, orgHandle, updatedRule)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	newRule, err := ruleService.GetUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Detect activation / deactivation and trigger reindex.
	nowActive := newRule.IsActive
	if !wasActive && nowActive {
		go worker.IndexNewAttribute(orgHandle, *newRule)
	}
	if wasActive && !nowActive {
		go worker.RemoveAttributeIndex(orgHandle, newRule.PropertyName)
	}

	ruleResponse := model.UnificationRuleAPIResponse{
		RuleId:            newRule.RuleId,
		RuleName:          newRule.RuleName,
		PropertyName:      newRule.PropertyName,
		Priority:          newRule.Priority,
		IsActive:          newRule.IsActive,
		AttributeType:     newRule.AttributeType,
		UnificationMethod: newRule.UnificationMethod,
	}
	utils.RespondJSON(w, http.StatusOK, ruleResponse, constants.UnificationRuleResource)
}

// DeleteUnificationRule removes a resolution rule.
func (urh *UnificationRulesHandler) DeleteUnificationRule(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:delete")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleId := r.PathValue("ruleId")
	if ruleId == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if !isCDSEnabled(orgHandle) {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()

	// Fetch the rule before deleting so we can trigger reindex cleanup.
	rule, fetchErr := ruleService.GetUnificationRule(ruleId)
	if fetchErr != nil {
		logger := log.GetLogger()
		logger.Warn(fmt.Sprintf("DeleteUnificationRule: could not fetch rule %s before deletion", ruleId))
	}

	err = ruleService.DeleteUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// If the deleted rule was active, trigger cleanup of its blocking keys.
	if rule != nil && rule.IsActive {
		go worker.RemoveAttributeIndex(orgHandle, rule.PropertyName)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// isCDSEnabled checks if CDS is enabled for the given tenant
func isCDSEnabled(orgHandle string) bool {
	return adminConfigService.GetAdminConfigService().IsCDSEnabled(orgHandle)
}
