package service

import (
	"fmt"
	enrStore "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/store"

	"net/http"
	"time"
)

type UnificationRuleServiceInterface interface {
	AddUnificationRule(rule model.UnificationRule) error
	GetUnificationRules() ([]model.UnificationRule, error)
	GetUnificationRule(ruleId string) (*model.UnificationRule, error)
	PatchResolutionRule(ruleId string, updates map[string]interface{}) error
	DeleteUnificationRule(ruleId string) error
}

// UnificationRuleService is the default implementation of the UnificationRuleServiceInterface.
type UnificationRuleService struct{}

// GetUnificationRuleService creates a new instance of UnificationRuleService.
func GetUnificationRuleService() UnificationRuleServiceInterface {

	return &UnificationRuleService{}
}

// AddUnificationRule Adds a new unification rule.
func (urs *UnificationRuleService) AddUnificationRule(rule model.UnificationRule) error {

	// Check if a similar unification rule already exists
	existingRule, err := store.GetUnificationRules()
	logger := log.GetLogger()
	if err != nil {
		return err
	}

	for _, existing := range existingRule {
		if existing.Property == rule.Property {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.PROP_ALREADY_EXISTS.Code,
				Message:     errors2.PROP_ALREADY_EXISTS.Message,
				Description: fmt.Sprintf("Unification rule with property %s already exists", rule.Property),
			}, http.StatusConflict)
		}
	}

	// Validate if the property name belongs in enrichment rules
	filter := []string{fmt.Sprintf("property_name eq %s", rule.Property)}
	resolutionRules, err := enrStore.GetEnrichmentRulesByFilter(filter)

	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while checking for existing enrichment rule: %s", rule.Property)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	if len(resolutionRules) == 0 { // captures nil as well
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: fmt.Sprintf("Unification rule with property %s not found in enrichment rules", rule.Property),
		}, http.StatusBadRequest)

	}

	// Set timestamps
	now := time.Now().UTC().Unix()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	return store.AddUnificationRule(rule)
}

// GetUnificationRules Fetches all resolution rules.
func (urs *UnificationRuleService) GetUnificationRules() ([]model.UnificationRule, error) {

	return store.GetUnificationRules()
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
func (urs *UnificationRuleService) PatchResolutionRule(ruleId string, updates map[string]interface{}) error {

	// Validate that all update fields are allowed
	for field := range updates {
		if !constants.AllowedFieldsForUnificationRulePatch[field] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:    errors2.UNIFICATION_UPDATE_FAILED.Code,
				Message: errors2.UNIFICATION_UPDATE_FAILED.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated. Rule Name, Active Status or Property"+
					" can only be updated", field),
			}, http.StatusBadRequest)
		}
	}

	return store.PatchUnificationRule(ruleId, updates)
}

// DeleteUnificationRule Removes a unification rule.
func (urs *UnificationRuleService) DeleteUnificationRule(ruleId string) error {

	return store.DeleteUnificationRule(ruleId)
}
