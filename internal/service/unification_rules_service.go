package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/locks"
	"github.com/wso2/identity-customer-data-service/internal/models"
	repositories "github.com/wso2/identity-customer-data-service/internal/repository"
)

// AddUnificationRule Adds a new unification rule.
func AddUnificationRule(rule models.UnificationRule) error {

	postgresDB := locks.GetPostgresInstance()
	unificationRuleRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)

	// Check if a similar unification rule already exists
	existingRule, err := unificationRuleRepo.GetUnificationRule(rule.RuleId)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileFetchingUnificationRules, err)
	}
	if existingRule.RuleId != "" {
		// Resolution rule already exists
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrResolutionRuleAlreadyExists.Code,
			Message:     errors.ErrResolutionRuleAlreadyExists.Message,
			Description: fmt.Sprintf("Unification rule with property %s already exists", rule.Property),
		}, http.StatusConflict)
	}

	// Set timestamps
	now := time.Now().UTC().Unix()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	return unificationRuleRepo.AddUnificationRule(rule)
}

// GetUnificationRules Fetches all resolution rules.
func GetUnificationRules() ([]models.UnificationRule, error) {

	postgresDB := locks.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.GetUnificationRules()
}

// GetUnificationRule Fetches a specific resolution rule.
func GetUnificationRule(ruleId string) (models.UnificationRule, error) {

	postgresDB := locks.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.GetUnificationRule(ruleId)
}

// PatchResolutionRule Applies a partial update on a specific resolution rule.
func PatchResolutionRule(ruleId string, updates map[string]interface{}) error {

	postgresDB := locks.GetPostgresInstance()
	unificationRulesRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)

	// Only allow patching specific fields
	allowedFields := map[string]bool{
		"is_active": true,
		"priority":  true,
		"rule_name": true,
	}

	// Validate that all update fields are allowed
	for field := range updates {
		if !allowedFields[field] {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrOnlyStatusUpdatePossible.Code,
				Message:     errors.ErrOnlyStatusUpdatePossible.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated.", field),
			}, http.StatusBadRequest)
		}
	}

	return unificationRulesRepo.PatchUnificationRule(ruleId, updates)
}

// DeleteUnificationRule Removes a unification rule.
func DeleteUnificationRule(ruleId string) error {
	postgresDB := locks.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.DeleteUnificationRule(ruleId)
}
