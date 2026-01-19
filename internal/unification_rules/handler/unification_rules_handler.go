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
	"github.com/wso2/identity-customer-data-service/internal/system/security"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/provider"
	"net/http"
	"sync"
	"time"

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

	var ruleInRequest model.UnificationRuleAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&ruleInRequest); err != nil {
		utils.HandleDecodeError(err, "unification rule")
		return
	}

	orgId := utils.ExtractOrgHandleFromPath(r)
	// Set timestamps
	now := time.Now().UTC().Unix()
	rule := model.UnificationRule{
		RuleId:       uuid.New().String(),
		TenantId:     orgId,
		RuleName:     ruleInRequest.RuleName,
		PropertyName: ruleInRequest.PropertyName,
		Priority:     ruleInRequest.Priority,
		IsActive:     ruleInRequest.IsActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	err := ruleService.AddUnificationRule(rule, orgId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	addedRule, err := ruleService.GetUnificationRule(rule.RuleId)
	addedRuleResponse := model.UnificationRuleAPIResponse{
		RuleId:       addedRule.RuleId,
		RuleName:     addedRule.RuleName,
		PropertyName: addedRule.PropertyName,
		Priority:     addedRule.Priority,
		IsActive:     addedRule.IsActive,
	}
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(addedRuleResponse)
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
	tenantId := utils.ExtractOrgHandleFromPath(r)
	rules, err := ruleService.GetUnificationRules(tenantId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	// Convert rules to API response format
	var rulesResponse []model.UnificationRuleAPIResponse
	for _, rule := range rules {
		tempRule := model.UnificationRuleAPIResponse{
			RuleId:       rule.RuleId,
			RuleName:     rule.RuleName,
			PropertyName: rule.PropertyName,
			Priority:     rule.Priority,
			IsActive:     rule.IsActive,
		}
		rulesResponse = append(rulesResponse, tempRule)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rulesResponse)
}

// GetUnificationRule Fetches a specific resolution rule.
func (urh *UnificationRulesHandler) GetUnificationRule(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "unification_rules:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleId := r.PathValue("ruleId")
	if ruleId == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
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
		RuleId:       rule.RuleId,
		RuleName:     rule.RuleName,
		PropertyName: rule.PropertyName,
		Priority:     rule.Priority,
		IsActive:     rule.IsActive,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ruleResponse)
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

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.HandleDecodeError(err, "unification rule")
		return
	}
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	orgId := utils.ExtractOrgHandleFromPath(r)
	err = ruleService.PatchUnificationRule(ruleId, orgId, updates)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	rule, err := ruleService.GetUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	ruleResponse := model.UnificationRuleAPIResponse{
		RuleId:       rule.RuleId,
		RuleName:     rule.RuleName,
		PropertyName: rule.PropertyName,
		Priority:     rule.Priority,
		IsActive:     rule.IsActive,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ruleResponse)
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
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	err = ruleService.DeleteUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
