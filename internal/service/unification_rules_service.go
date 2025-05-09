package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	"net/http"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/database"
	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/models"
	repositories "github.com/wso2/identity-customer-data-service/internal/repository"
)

// AddUnificationRule Adds a new unification rule.
func AddUnificationRule(rule models.UnificationRule) error {

	postgresDB := database.GetPostgresInstance()
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

	postgresDB := database.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.GetUnificationRules()
}

// GetUnificationRule Fetches a specific resolution rule.
func GetUnificationRule(ruleId string) (models.UnificationRule, error) {

	postgresDB := database.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.GetUnificationRule(ruleId)
}

// PatchUnificationRule Applies a partial update on a specific resolution rule.
func PatchUnificationRule(ruleId string, rawUpdates map[string]interface{}) error {

	var update models.UnificationRule
	// Validate and build UnificationRule object
	for field, value := range rawUpdates {
		if !constants.AllowedFieldsForUnificationRulePatch[field] {
			badReq := errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrOnlyStatusUpdatePossible.Code,
				Message:     errors.ErrOnlyStatusUpdatePossible.Message,
				Description: fmt.Sprintf("Field '%s' cannot be updated.", field),
			}, http.StatusBadRequest)
			return badReq
		}

		// Apply updates based on allowed field name
		switch field {
		case "rule_name":
			if strVal, ok := value.(string); ok {
				update.RuleName = strVal
			}
		case "priority":
			if floatVal, ok := value.(float64); ok {
				update.Priority = int(floatVal)
			}
		case "is_active":
			if boolVal, ok := value.(bool); ok {
				update.IsActive = boolVal
			}
		}
	}
	update.UpdatedAt = time.Now().UTC().Unix()

	postgresDB := database.GetPostgresInstance()
	unificationRulesRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)

	return unificationRulesRepo.PatchUnificationRule(ruleId, update)
}

// DeleteUnificationRule Removes a unification rule.
func DeleteUnificationRule(ruleId string) error {
	postgresDB := database.GetPostgresInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(postgresDB.DB)
	return unificationRepo.DeleteUnificationRule(ruleId)
}
