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
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/store"
)

type UnificationRuleServiceInterface interface {
	AddUnificationRule(rule model.UnificationRule, orgHandle string) error
	GetUnificationRules(orgHandle string) ([]model.UnificationRule, error)
	GetUnificationRule(ruleId string) (*model.UnificationRule, error)
	PatchUnificationRule(ruleId, orgHandle string, updatedRule model.UnificationRule) error
	DeleteUnificationRule(ruleId string) error
	GetUnificationOptions() model.UnificationOptionsResponse
}

// UnificationRuleService is the default implementation of the UnificationRuleServiceInterface.
type UnificationRuleService struct{}

// GetUnificationRuleService creates a new instance of UnificationRuleService.
func GetUnificationRuleService() UnificationRuleServiceInterface {

	return &UnificationRuleService{}
}

// AddUnificationRule Adds a new unification rule.
func (urs *UnificationRuleService) AddUnificationRule(rule model.UnificationRule, orgHandle string) error {

	logger := log.GetLogger()
	// Need to specifically prevent
	if rule.PropertyName == "user_id" || rule.PropertyName == "identity_attributes.user_id" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: "user_id based unification rule can not be created.",
		}, http.StatusBadRequest)
	}

	if strings.HasPrefix(rule.PropertyName, constants.ApplicationData+".") {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: "Creating unification rules based on application data is not supported.",
		}, http.StatusBadRequest)
	}

	profileSchemaService := provider.NewProfileSchemaProvider().GetProfileSchemaService()
	schemaAttribute, err := profileSchemaService.GetProfileSchemaAttributeByName(rule.PropertyName, rule.OrgHandle)

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
	existingRules, err := store.GetUnificationRules(orgHandle)
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
	return store.AddUnificationRule(rule, orgHandle)
}

// GetUnificationRules Fetches all resolution rules.
func (urs *UnificationRuleService) GetUnificationRules(orgHandle string) ([]model.UnificationRule, error) {
	return store.GetUnificationRules(orgHandle)
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
			Description: fmt.Sprintf("Unification rule: '%s' not found", ruleId),
		}, http.StatusNotFound)
	}
	return unificationRule, err
}

// PatchUnificationRule Applies a partial update on a specific resolution rule.
func (urs *UnificationRuleService) PatchUnificationRule(ruleId, orgHandle string, updatedRule model.UnificationRule) error {

	if updatedRule.PropertyName == "user_id" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
			Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
			Description: "user_id based unification rule can not be updated.",
		}, http.StatusBadRequest)
	}

	// Validate that the priority is not already in use
	existingRules, err := store.GetUnificationRules(orgHandle)
	if err != nil {
		return err
	}
	for _, existingRule := range existingRules {
		if existingRule.RuleId != ruleId && existingRule.Priority == updatedRule.Priority {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_PRIORITY_EXISTS.Message,
				Description: "Unification rule with same priority exist.",
			}, http.StatusBadRequest)
		}
	}
	return store.PatchUnificationRule(ruleId, updatedRule)
}

// DeleteUnificationRule Removes a unification rule.
func (urs *UnificationRuleService) DeleteUnificationRule(ruleId string) error {

	return store.DeleteUnificationRule(ruleId)
}

// attributeTypeOptions is the ordered list of all supported attribute types and their allowed
// matching methods, derived entirely from system constants — no database access required.
var attributeTypeOptions = []model.AttributeTypeOption{
	{
		Value: constants.AttributeTypePrimitiveExact,
		Label: constants.AttributeTypeLabelPrimitiveExact,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
		},
	},
	{
		Value: constants.AttributeTypeFuzzyString,
		Label: constants.AttributeTypeLabelFuzzyString,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
			{Value: constants.UnificationMethodFuzzy, Label: constants.UnificationMethodLabelFuzzyGeneral},
		},
	},
	{
		Value: constants.AttributeTypeName,
		Label: constants.AttributeTypeLabelName,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
			{Value: constants.UnificationMethodFuzzy, Label: constants.UnificationMethodLabelFuzzyPhonetic},
		},
	},
	{
		Value: constants.AttributeTypeEmail,
		Label: constants.AttributeTypeLabelEmail,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
			{Value: constants.UnificationMethodFuzzy, Label: constants.UnificationMethodLabelFuzzyGeneral},
		},
	},
	{
		Value: constants.AttributeTypePhone,
		Label: constants.AttributeTypeLabelPhone,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
			{Value: constants.UnificationMethodFuzzy, Label: constants.UnificationMethodLabelFuzzyFormat},
		},
	},
	{
		Value: constants.AttributeTypeLocation,
		Label: constants.AttributeTypeLabelLocation,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
			{Value: constants.UnificationMethodFuzzy, Label: constants.UnificationMethodLabelFuzzyGeneral},
		},
	},
	{
		Value: constants.AttributeTypeDate,
		Label: constants.AttributeTypeLabelDate,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
		},
	},
	{
		Value: constants.AttributeTypeUniqueID,
		Label: constants.AttributeTypeLabelUniqueID,
		AllowedMethods: []model.MethodOption{
			{Value: constants.UnificationMethodDeterministic, Label: constants.UnificationMethodLabelDeterministic},
		},
	},
}

// matchStrengthOptions and mismatchStrengthOptions are the evidence-strength choices,
// ordered strongest first.
var matchStrengthOptions = []model.StrengthOption{
	{Value: constants.EvidenceStrengthHigh, Label: constants.MatchStrengthLabelHigh,
		Description: constants.MatchStrengthDescriptionHigh},
	{Value: constants.EvidenceStrengthMedium, Label: constants.MatchStrengthLabelMedium,
		Description: constants.MatchStrengthDescriptionMedium},
	{Value: constants.EvidenceStrengthLow, Label: constants.MatchStrengthLabelLow,
		Description: constants.MatchStrengthDescriptionLow},
}

var mismatchStrengthOptions = []model.StrengthOption{
	{Value: constants.EvidenceStrengthHigh, Label: constants.MismatchStrengthLabelHigh,
		Description: constants.MismatchStrengthDescriptionHigh},
	{Value: constants.EvidenceStrengthMedium, Label: constants.MismatchStrengthLabelMedium,
		Description: constants.MismatchStrengthDescriptionMedium},
	{Value: constants.EvidenceStrengthLow, Label: constants.MismatchStrengthLabelLow,
		Description: constants.MismatchStrengthDescriptionLow},
}

// GetUnificationOptions returns everything a client needs to build the rule form: the
// supported attribute types, the matching methods each one allows, the evidence-strength
// choices, and the strengths an attribute type takes when the operator does not pick any.
//
// The per-type defaults are read from the same tables the scorer uses, so what the form
// offers cannot drift from what the engine applies.
func (urs *UnificationRuleService) GetUnificationOptions() model.UnificationOptionsResponse {

	types := make([]model.AttributeTypeOption, 0, len(attributeTypeOptions))
	for _, option := range attributeTypeOptions {
		option.DefaultMatchStrength = constants.DefaultMatchStrength[option.Value]
		option.DefaultMismatchStrength = constants.DefaultMismatchStrength[option.Value]
		types = append(types, option)
	}

	return model.UnificationOptionsResponse{
		AttributeTypes:    types,
		MatchStrengths:    matchStrengthOptions,
		MismatchStrengths: mismatchStrengthOptions,
	}
}
