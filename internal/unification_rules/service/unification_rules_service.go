package service

import (
	"fmt"
	psService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
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
	PatchUnificationRule(ruleId string, updates map[string]interface{}) error
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
	if rule.Property == "user_id" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
			Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
			Description: fmt.Sprintf("Unification rule with property %s already exists", rule.Property),
		}, http.StatusConflict)
	}

	// Validate if the property name belongs in schema attributes
	filter := []string{fmt.Sprintf("attribute_name eq %s", rule.Property)}
	schemaAttributes, err := psService.GetProfileSchemaAttributesWithFilter(rule.TenantId, filter)

	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while checking for existingRule schema: %s", rule.Property)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	if len(schemaAttributes) == 0 { // captures nil as well
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: fmt.Sprintf("Attribute  %s is not found in schema", rule.Property),
		}, http.StatusBadRequest)
	} else {
		for _, schemaAttribute := range schemaAttributes {
			if schemaAttribute.ValueType == constants.ComplexDataType {
				return errors2.NewClientError(errors2.ErrorMessage{
					Code:    errors2.ADD_UNIFICATION_RULE.Code,
					Message: errors2.ADD_UNIFICATION_RULE.Message,
					Description: "Unification rule with property " + rule.Property + " is not allowed as it is a complex data type. " +
						"Choose the sub-attribute instead.",
				}, http.StatusBadRequest)
			}
			logger.Debug(fmt.Sprintf("Unification rule with property %s is valid", rule.Property))
		}
	}

	// Check if a similar unification rule already exists
	existingRules, err := store.GetUnificationRules(tenantId)
	if err != nil {
		return err
	}
	for _, existingRule := range existingRules {
		if existingRule.Property == rule.Property {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
				Description: fmt.Sprintf("Unification rule with property %s already exists", rule.Property),
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

// PatchResolutionRule Applies a partial update on a specific resolution rule.
func (urs *UnificationRuleService) PatchUnificationRule(ruleId string, updates map[string]interface{}) error {

	// Validate that all update fields are allowed
	for field := range updates {
		if !constants.AllowedFieldsForUnificationRulePatch[field] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_UPDATE_FAILED.Code,
				Message:     errors2.UNIFICATION_UPDATE_FAILED.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated. Rule Name, Active Status or Property can only be updated", field),
			}, http.StatusBadRequest)
		}
	}

	// Validate that the priority is not already in use
	existingRules, _ := store.GetUnificationRules("")
	for _, existingRule := range existingRules {
		if existingRule.Property == "user_id" {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UNIFICATION_RULE_ALREADY_EXISTS.Code,
				Message:     errors2.UNIFICATION_RULE_ALREADY_EXISTS.Message,
				Description: fmt.Sprintf("user_id based unification rule can not be updated."),
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
