package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/repository"
	"go.mongodb.org/mongo-driver/bson"
	"net/http"
	"time"
)

// AddUnificationRule Adds a new unification rule.
func AddUnificationRule(rule models.UnificationRule) error {

	mongoDB := locks.GetMongoDBInstance()
	unificationRuleRepo := repositories.NewUnificationRuleRepository(mongoDB.Database, constants.UnificationRulesCollection)

	// Check if the attribute exists in profile schema enrichment rules
	filter := fmt.Sprintf("property_name eq %s", rule.Property)
	profileEnrichmentRules, err := GetEnrichmentRulesByFilter([]string{filter})

	if err != nil {
		errors.ErrWhileAddingUnificationRules.Description = "Failed when validating if the attribute exists in " +
			"enrichment rules."
		return errors.NewServerError(errors.ErrWhileFetchingEnrichmentRules, err)
	}

	if len(profileEnrichmentRules) == 0 {
		// Property does not exist in profile enrichment rules
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrPropDoesntExists.Code,
			Message:     errors.ErrPropDoesntExists.Message,
			Description: fmt.Sprintf("Property %s does not exist as a profile enrichment rule", rule.Property),
		}, http.StatusConflict)
	}

	// Check if a similar unification rule already exists
	existingRule, err := unificationRuleRepo.GetUnificationRuleByPropertyName(rule.Property)
	if err != nil {
		errors.ErrWhileAddingUnificationRules.Description = "Failed to fetch existing unification rules"
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

	mongoDB := locks.GetMongoDBInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(mongoDB.Database, constants.UnificationRulesCollection)
	rules, err := unificationRepo.GetUnificationRules()
	return rules, err
}

// GetUnificationRule Fetches a specific resolution rule.
func GetUnificationRule(ruleId string) (models.UnificationRule, error) {

	mongoDB := locks.GetMongoDBInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(mongoDB.Database, constants.UnificationRulesCollection)
	rule, err := unificationRepo.GetUnificationRule(ruleId)
	if rule.RuleId == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrResolutionRuleNotFound.Code,
			Message:     errors.ErrResolutionRuleNotFound.Message,
			Description: errors.ErrResolutionRuleNotFound.Description,
		}, http.StatusNotFound)

		return rule, clientError
	}
	return rule, err
}

// PatchResolutionRule Applies a partial update on a specific resolution rule.
func PatchResolutionRule(ruleId string, updates bson.M) error {

	mongoDB := locks.GetMongoDBInstance()
	unificationRulesRepo := repositories.NewUnificationRuleRepository(mongoDB.Database, constants.UnificationRulesCollection)

	// Only allow patching specific fields
	allowedFields := map[string]bool{
		"is_active": true,
		"priority":  true,
		"rule_name": true,
	}

	// Validate that all update fields are allowed
	for field := range updates {
		if !allowedFields[field] {
			clientError := errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrOnlyStatusUpdatePossible.Code,
				Message:     errors.ErrOnlyStatusUpdatePossible.Message,
				Description: fmt.Sprintf("Field '%s' can not be updated.", field),
			}, http.StatusBadRequest)
			return clientError
		}
	}

	return unificationRulesRepo.PatchUnificationRule(ruleId, updates)
}

// DeleteUnificationRule Removes a  unification rule.
func DeleteUnificationRule(ruleId string) error {
	mongoDB := locks.GetMongoDBInstance()
	unificationRepo := repositories.NewUnificationRuleRepository(mongoDB.Database, constants.UnificationRulesCollection)
	return unificationRepo.DeleteUnificationRule(ruleId)
}
