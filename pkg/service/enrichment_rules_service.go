package service

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/repository"
	"net/http"
	"strings"
	"time"
)

func AddEnrichmentRule(rule models.ProfileEnrichmentRule) error {

	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)

	rule.RuleId = uuid.New().String()

	err, isValid := validateEnrichmentRule(rule)
	if !isValid {
		return err
	}

	rule.CreatedAt = time.Now().UTC().Unix()
	rule.UpdatedAt = time.Now().UTC().Unix()

	return schemaRepo.UpsertEnrichmentRule(rule)
}

func GetEnrichmentRules() ([]models.ProfileEnrichmentRule, error) {
	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)
	return schemaRepo.GetProfileEnrichmentRules()
}

func GetEnrichmentRulesByFilter(filters []string) ([]models.ProfileEnrichmentRule, error) {
	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)
	return schemaRepo.GetEnrichmentRulesByFilter(filters)
}

func GetEnrichmentRule(ruleId string) (models.ProfileEnrichmentRule, error) {
	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)
	return schemaRepo.GetSchemaRule(ruleId)
}

func PutEnrichmentRule(rule models.ProfileEnrichmentRule) error {
	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)

	err, isValid := validateEnrichmentRule(rule)
	if !isValid {
		return err
	}
	return schemaRepo.UpsertEnrichmentRule(rule)
}

func DeleteEnrichmentRule(ruleId string) error {
	mongoDB := locks.GetMongoDBInstance()
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)
	return schemaRepo.DeleteSchemaRule(ruleId)
}

// validateEnrichmentRule validates the enrichment rule.
func validateEnrichmentRule(rule models.ProfileEnrichmentRule) (error, bool) {

	//  Required: Trait Name
	if rule.PropertyName == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrPropertyNameValidation.Code,
			Message:     errors.ErrPropertyNameValidation.Message,
			Description: errors.ErrPropertyNameValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	//  Required: Rule Type
	if rule.PropertyType != "static" && rule.PropertyType != "computed" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrPropertyTypeValidation.Code,
			Message:     errors.ErrPropertyTypeValidation.Message,
			Description: errors.ErrPropertyTypeValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	//  Required for Static: Value
	if rule.PropertyType == "static" && rule.Value == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrEnrichmentRuleValueValidation.Code,
			Message:     errors.ErrEnrichmentRuleValueValidation.Message,
			Description: errors.ErrEnrichmentRuleValueValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	//  Required for Computed: Computation logic
	if rule.PropertyType == "computed" && rule.Computation == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrComputationValidation.Code,
			Message:     errors.ErrComputationValidation.Message,
			Description: errors.ErrComputationValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	if rule.Computation == "copy" && len(rule.SourceFields) != 1 {
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrSourceFieldValidation.Code,
			Message:     errors.ErrSourceFieldValidation.Message,
			Description: errors.ErrSourceFieldValidation.Description,
		}, http.StatusBadRequest), false
	}

	//  Validate Trigger
	if rule.Trigger.EventType == "" || rule.Trigger.EventName == "" {
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrTriggerValidation.Code,
			Message:     errors.ErrTriggerValidation.Message,
			Description: "Both 'event_type' and 'event_name' must be provided inside trigger",
		}, http.StatusBadRequest), false
	}

	//  Validate Trigger Conditions
	for _, cond := range rule.Trigger.Conditions {
		if cond.Field == "" || cond.Operator == "" {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrTriggerValidation.Code,
				Message:     errors.ErrTriggerValidation.Message,
				Description: "Each condition must have a field and operator defined.",
			}, http.StatusBadRequest), false
		}
		if !constants.AllowedConditionOperators[strings.ToLower(cond.Operator)] {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrConditionOpValidation.Code,
				Message:     errors.ErrConditionOpValidation.Message,
				Description: fmt.Sprintf("Operator '%s' is not supported.", cond.Operator),
			}, http.StatusBadRequest), false
		}
	}

	// Validate Merge Strategy
	if rule.MergeStrategy != "" && !constants.AllowedMergeStrategies[strings.ToLower(rule.MergeStrategy)] {
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrMergeStratValidation.Code,
			Message:     errors.ErrMergeStratValidation.Message,
			Description: fmt.Sprintf("Merge strategy '%s' is not allowed.", rule.MergeStrategy),
		}, http.StatusBadRequest), false
	}

	//  Validate Masking
	if rule.MaskingRequired {
		if rule.MaskingStrategy == "" {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrMaskingStratValidation.Code,
				Message:     errors.ErrMaskingStratValidation.Message,
				Description: "Masking is required, but no strategy was provided.",
			}, http.StatusBadRequest), false
		}
		if !constants.AllowedMaskingStrategies[strings.ToLower(rule.MaskingStrategy)] {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrMaskingStratValidation.Code,
				Message:     errors.ErrMaskingStratValidation.Message,
				Description: fmt.Sprintf("Masking strategy '%s' is not supported.", rule.MaskingStrategy),
			}, http.StatusBadRequest), false
		}
	}
	return nil, true
}
