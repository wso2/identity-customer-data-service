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
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
	"strings"
	"sync"
)

type EnrichmentRulesHandler struct {
	store map[string]model.ProfileEnrichmentRule
	mu    *sync.RWMutex
}

func NewEnrichmentRulesHandler() *EnrichmentRulesHandler {

	return &EnrichmentRulesHandler{
		store: make(map[string]model.ProfileEnrichmentRule),
		mu:    &sync.RWMutex{},
	}
}

// CreateEnrichmentRule handles POST /unification_rules
func (erh *EnrichmentRulesHandler) CreateEnrichmentRule(w http.ResponseWriter, r *http.Request) {

	var rule model.ProfileEnrichmentRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	err := ruleService.AddEnrichmentRule(rule)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Enrichment rule: %s for property: %s created successfully", rule.RuleId,
		rule.PropertyName))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

// GetEnrichmentRules handles GET /unification_rules
func (erh *EnrichmentRulesHandler) GetEnrichmentRules(w http.ResponseWriter, r *http.Request) {

	filters := r.URL.Query()[constants.Filter] // Handles multiple `filter=...` parameters

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	if len(filters) > 0 {
		rules, err := ruleService.GetEnrichmentRulesByFilter(filters)
		if err != nil {
			utils.HandleError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(rules)
		return
	}

	// fallback: all rules
	rules, err := ruleService.GetEnrichmentRules()
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	logger := log.GetLogger()
	logger.Info("Enrichment rules retrieved successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rules)
}

// GetEnrichmentRule handles GET /unification_rules/:rule_id
func (erh *EnrichmentRulesHandler) GetEnrichmentRule(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	ruleId := pathParts[len(pathParts)-1]

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	rule, err := ruleService.GetEnrichmentRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Enrichment rule: %s retrieved successfully", ruleId))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rule)
}

// UpdateEnrichmentRule handles PUT /unification_rules/:rule_id
func (erh *EnrichmentRulesHandler) UpdateEnrichmentRule(w http.ResponseWriter, r *http.Request) {

	var rules model.ProfileEnrichmentRule
	// fetch and validate if it exists already

	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	err := ruleService.UpdateEnrichmentRule(rules)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Enrichment rule: %s updated successfully.", rules.RuleId))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rules)
}

// DeleteEnrichmentRule handles DELETE /unification_rules/:rule_id
func (erh *EnrichmentRulesHandler) DeleteEnrichmentRule(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	ruleId := pathParts[len(pathParts)-1]
	if ruleId == "" {
		http.Error(w, "rule id is required", http.StatusBadRequest)
		return
	}

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	err := ruleService.DeleteEnrichmentRule(ruleId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Enrichment rule: %s deleted successfully.", ruleId))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w)
}
