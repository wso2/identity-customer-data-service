package handler

import (
	"encoding/json"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/utils"
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

// CreateEnrichmentRule handles creating new profile enrichment rule
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
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// GetEnrichmentRules handles retrieve of all rules with or without filters
func (erh *EnrichmentRulesHandler) GetEnrichmentRules(w http.ResponseWriter, r *http.Request) {

	filters := r.URL.Query()[constants.Filter] // Handles multiple `filter=...` parameters

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	if len(filters) > 0 {
		rules, err := ruleService.GetEnrichmentRulesByFilter(filters)
		if err != nil {
			utils.HandleHTTPError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(rules)
		return
	}

	// fallback: all rules
	rules, err := ruleService.GetEnrichmentRules()
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rules)
}

// GetEnrichmentRule handles retrieivng a specific rule
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
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rule)
}

func (erh *EnrichmentRulesHandler) PutEnrichmentRule(w http.ResponseWriter, r *http.Request) {

	var rules model.ProfileEnrichmentRule
	// fetch and validate if it exists already

	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	err := ruleService.PutEnrichmentRule(rules)
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rules)
}

// DeleteEnrichmentRule handles DELETE /unification_rules/:rule_name
func (erh *EnrichmentRulesHandler) DeleteEnrichmentRule(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	ruleId := pathParts[len(pathParts)-1]
	if ruleId == "" {
		http.Error(w, "rule_name is required", http.StatusBadRequest)
		return
	}

	ruleProvider := provider.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	err := ruleService.DeleteEnrichmentRule(ruleId)
	if err != nil {
		utils.HandleHTTPError(w, err)
	}
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w)
}
