package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/database"
	repositories "github.com/wso2/identity-customer-data-service/internal/events/store"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/store"
	"net/http"
	"time"
)

type UnificationRuleServiceInterface interface {
	AddUnificationRule(rule model.UnificationRule) error
	GetUnificationRules() ([]model.UnificationRule, error)
	GetUnificationRule(ruleId string) (model.UnificationRule, error)
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
	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories(postgresDB.DB)
	existingRule, err := store.GetUnificationRule(rule.RuleId)
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileFetchingUnificationRules, err)
	}
	if existingRule.RuleId != "" {
		// Resolution rule already exists
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrResolutionRuleAlreadyExists.Code,
			Message:     errors2.ErrResolutionRuleAlreadyExists.Message,
			Description: fmt.Sprintf("Unification rule with property %s already exists", rule.Property),
		}, http.StatusConflict)
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
func (urs *UnificationRuleService) GetUnificationRule(ruleId string) (model.UnificationRule, error) {

	return store.GetUnificationRule(ruleId)
}

// PatchResolutionRule Applies a partial update on a specific resolution rule.
func (urs *UnificationRuleService) PatchResolutionRule(ruleId string, updates map[string]interface{}) error {

	// Only allow patching specific fields
	allowedFields := map[string]bool{
		"is_active": true,
		"priority":  true,
		"rule_name": true,
	}

	// Validate that all update fields are allowed
	for field := range updates {
		if !allowedFields[field] {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.ErrOnlyStatusUpdatePossible.Code,
				Message:     errors2.ErrOnlyStatusUpdatePossible.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated.", field),
			}, http.StatusBadRequest)
		}
	}

	return store.PatchUnificationRule(ruleId, updates)
}

// DeleteUnificationRule Removes a unification rule.
func (urs *UnificationRuleService) DeleteUnificationRule(ruleId string) error {

	return store.DeleteUnificationRule(ruleId)
}
