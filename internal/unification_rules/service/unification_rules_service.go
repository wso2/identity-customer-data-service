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
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/store"
	"net/http"
)

type UnificationRuleServiceInterface interface {
	AddUnificationRule(rule model.UnificationRule, tenantId string) error
	GetUnificationRules(tenantId string) ([]model.UnificationRule, error)
	GetUnificationRule(ruleId string) (*model.UnificationRule, error)
	PatchUnificationRule(ruleId, tenantId string, updates map[string]interface{}) error
	DeleteUnificationRule(ruleId string) error
}

// UnificationRuleService is the default implementation of the UnificationRuleServiceInterface.
type UnificationRuleService struct{}

// GetUnificationRuleService creates a new instance of UnificationRuleService.
func GetUnificationRuleService() UnificationRuleServiceInterface {

	return &UnificationRuleService{}
}

// AddUnificationRule Adds a new unification rule.
func (urs *UnificationRuleService) AddUnificationRule(rule model.UnificationRule, tenantId string) error {

	logger := log.GetLogger()
	// Need to specifically prevent
	if rule.PropertyName == "user_id" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
			Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
			Description: fmt.Sprintf("Unification rule with property %s already exists", rule.PropertyName),
		}, http.StatusConflict)
	}

	profileSchemaService := provider.NewProfileSchemaProvider().GetProfileSchemaService()
	schemaAttribute, err := profileSchemaService.GetProfileSchemaAttributeByName(rule.PropertyName, rule.TenantId)

	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while checking for the property: %s", rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	if schemaAttribute == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: fmt.Sprintf("PropertyName  '%s' is not found in schema", rule.PropertyName),
		}, http.StatusBadRequest)
	}
	if schemaAttribute.ValueType == constants.ComplexDataType {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:    errors2.ADD_UNIFICATION_RULE.Code,
			Message: errors2.ADD_UNIFICATION_RULE.Message,
			Description: "Unification rule with property " + rule.PropertyName + " is not allowed as it is a complex data type. " +
				"Choose the sub-attribute instead.",
		}, http.StatusBadRequest)
	}

	// Check if a similar unification rule already exists
	existingRules, err := store.GetUnificationRules(tenantId)
	if err != nil {
		return err
	}
	for _, existingRule := range existingRules {
		if existingRule.PropertyName == rule.PropertyName {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
				Description: fmt.Sprintf("Unification rule with property %s already exists", rule.PropertyName),
			}, http.StatusConflict)
		}
		if existingRule.Priority == rule.Priority {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Message,
				Description: "Unification rule with same priority exist.",
			}, http.StatusBadRequest)
		}
	}
	rule.PropertyId = schemaAttribute.AttributeId
	return store.AddUnificationRule(rule, tenantId)
}

// GetUnificationRules Fetches all resolution rules.
func (urs *UnificationRuleService) GetUnificationRules(tenantId string) ([]model.UnificationRule, error) {

	return store.GetUnificationRules(tenantId)
}

// GetUnificationRule Fetches a specific resolution rule.
func (urs *UnificationRuleService) GetUnificationRule(ruleId string) (*model.UnificationRule, error) {

	unificationRule, err := store.GetUnificationRule(ruleId)
	if err != nil {
		return nil, err
	}
	if unificationRule == nil {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UNIFICATION_RULE_NOT_FOUND.Code,
			Message:     errors2.UNIFICATION_RULE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Unification rule: %s not found", ruleId),
		}, http.StatusNotFound)
	}
	return unificationRule, err
}

// PatchUnificationRule Applies a partial update on a specific resolution rule.
func (urs *UnificationRuleService) PatchUnificationRule(ruleId, tenantId string, updates map[string]interface{}) error {

	// Validate that all update fields are allowed
	for field := range updates {
		if !constants.AllowedFieldsForUnificationRulePatch[field] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_UPDATE_FAILED.Code,
				Message:     errors2.UNIFICATION_UPDATE_FAILED.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated. Rule Name, Active Status or PropertyName can only be updated", field),
			}, http.StatusBadRequest)
		}
	}

	// Validate that the priority is not already in use
	existingRules, _ := store.GetUnificationRules(tenantId)
	for _, existingRule := range existingRules {
		if existingRule.PropertyName == "user_id" {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
				Description: "user_id based unification rule can not be updated.",
			}, http.StatusConflict)
		}
		if existingRule.RuleId != ruleId && existingRule.Priority == updates["priority"] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Message,
				Description: "Unification rule with same priority exist.",
			}, http.StatusBadRequest)
		}
	}

	return store.PatchUnificationRule(ruleId, updates)
}

// DeleteUnificationRule Removes a unification rule.
func (urs *UnificationRuleService) DeleteUnificationRule(ruleId string) error {

	return store.DeleteUnificationRule(ruleId)
}
