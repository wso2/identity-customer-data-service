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

package service

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
	"time"
)

type EnrichmentRuleServiceInterface interface {
	AddEnrichmentRule(rule model.ProfileEnrichmentRule) error
	GetEnrichmentRules() ([]model.ProfileEnrichmentRule, error)
	GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error)
	GetEnrichmentRule(ruleId string) (*model.ProfileEnrichmentRule, error)
	PutEnrichmentRule(rule model.ProfileEnrichmentRule) error
	DeleteEnrichmentRule(ruleId string) error
}

// EnrichmentRuleService is the default implementation of the EnrichmentRuleServiceInterface.
type EnrichmentRuleService struct{}

// GetEnrichmentRuleService creates a new instance of EnrichmentRuleService.
func GetEnrichmentRuleService() EnrichmentRuleServiceInterface {

	return &EnrichmentRuleService{}
}

func (ers *EnrichmentRuleService) AddEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	rule.RuleId = uuid.New().String()

	err, isValid := validateEnrichmentRule(rule)
	if !isValid {
		return err
	}

	currentTime := time.Now().UTC().Unix()
	rule.CreatedAt = currentTime
	rule.UpdatedAt = currentTime

	return store.AddEnrichmentRule(rule)
}

func (ers *EnrichmentRuleService) GetEnrichmentRules() ([]model.ProfileEnrichmentRule, error) {

	return store.GetProfileEnrichmentRules()
}

func (ers *EnrichmentRuleService) GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error) {

	return store.GetEnrichmentRulesByFilter(filters)
}

func (ers *EnrichmentRuleService) GetEnrichmentRule(ruleId string) (*model.ProfileEnrichmentRule, error) {

	return store.GetProfileEnrichmentRule(ruleId)
}

func (ers *EnrichmentRuleService) PutEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	err, isValid := validateEnrichmentRule(rule)
	// todo: DEfine allowed updatable fields - it has to become a Patch then
	if !isValid {
		return err
	}
	return store.UpdateEnrichmentRule(rule)
}

func (ers *EnrichmentRuleService) DeleteEnrichmentRule(ruleId string) error {

	rule, _ := store.GetProfileEnrichmentRule(ruleId)
	if rule.RuleId == "" {
		return nil
	}
	return store.DeleteProfileEnrichmentRule(rule)
}

// validateEnrichmentRule validates the enrichment rule.
func validateEnrichmentRule(rule model.ProfileEnrichmentRule) (error, bool) {

	//  Required: property Name
	if rule.PropertyName == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "Enrichment rule must include a valid property name.",
		}, http.StatusBadRequest)
		return clientError, false
	}

	if rule.ComputationMethod == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "Computation method is required for the enrichment rule.",
		}, http.StatusBadRequest)
		return clientError, false
	}

	if !constants.AllowedComputationMethods[rule.ComputationMethod] {
		badReq := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: fmt.Sprintf("'%s' is not an expected computation method.", rule.ComputationMethod),
		}, http.StatusBadRequest)
		return badReq, false
	}

	//  Required for Static: Value
	if rule.ComputationMethod == "static" && rule.Value == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "For static computation method, 'value' must be provided.",
		}, http.StatusBadRequest)
		return clientError, false
	}

	if rule.ComputationMethod == "extract" && rule.SourceField == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "For copy computation method, 'source field' must be provided",
		}, http.StatusBadRequest), false
	}

	if rule.ComputationMethod == "count" {
		if rule.TimeRange < 0 || rule.TimeRange > 2592000 {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
				Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
				Description: "Time range should be from least 15 minutes to 30 days.",
			}, http.StatusBadRequest), false
		}
	}

	if rule.ComputationMethod != "count" && rule.TimeRange != 0 {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "Time range is only applicable for count computation.",
		}, http.StatusBadRequest), false
	}

	logger := log.GetLogger()
	// Validate Time Range
	if rule.TimeRange == 0 {
		logger.Debug("Time range is not provided, defaulting to infinite denoted by -1.")
		rule.TimeRange = -1
	}

	//  Validate Trigger
	if rule.Trigger.EventType == "" || rule.Trigger.EventName == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: "Both 'event_type' and 'event_name' are must for the trigger condition.",
		}, http.StatusBadRequest), false
	}

	//  Validate Trigger Conditions
	for _, cond := range rule.Trigger.Conditions {
		if cond.Field == "" || cond.Operator == "" {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
				Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
				Description: "Each trigger condition must have a field and operator defined.",
			}, http.StatusBadRequest), false
		}
		if !constants.AllowedConditionOperators[strings.ToLower(cond.Operator)] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
				Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
				Description: fmt.Sprintf("Operator '%s' is not supported.", cond.Operator),
			}, http.StatusBadRequest), false
		}
	}

	// Validate Merge Strategy
	if rule.MergeStrategy != "" && !constants.AllowedMergeStrategies[strings.ToLower(rule.MergeStrategy)] {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ENRICHMENT_RULE_VALIDATION.Code,
			Message:     errors2.ENRICHMENT_RULE_VALIDATION.Message,
			Description: fmt.Sprintf("Merge strategy '%s' is not allowed.", rule.MergeStrategy),
		}, http.StatusBadRequest), false
	}

	return nil, true
}
