package service

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	"github.com/wso2/identity-customer-data-service/internal/database"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/logger"
	"net/http"
	"strings"
	"time"
)

type EnrichmentRuleServiceInterface interface {
	AddEnrichmentRule(rule model.ProfileEnrichmentRule) error
	GetEnrichmentRules() ([]model.ProfileEnrichmentRule, error)
	GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error)
	GetEnrichmentRule(ruleId string) (model.ProfileEnrichmentRule, error)
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

	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	rule.RuleId = uuid.New().String()

	err, isValid := validateEnrichmentRule(rule)
	if !isValid {
		return err
	}

	currentTime := time.Now().UTC().Unix()
	rule.CreatedAt = currentTime
	rule.UpdatedAt = currentTime

	return schemaRepo.AddEnrichmentRule(rule)
}

func (ers *EnrichmentRuleService) GetEnrichmentRules() ([]model.ProfileEnrichmentRule, error) {
	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	return schemaRepo.GetProfileEnrichmentRules()
}

func (ers *EnrichmentRuleService) GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error) {
	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	return schemaRepo.GetEnrichmentRulesByFilter(filters)
}

func (ers *EnrichmentRuleService) GetEnrichmentRule(ruleId string) (model.ProfileEnrichmentRule, error) {
	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	return schemaRepo.GetProfileEnrichmentRule(ruleId)
}

func (ers *EnrichmentRuleService) PutEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)

	err, isValid := validateEnrichmentRule(rule)
	if !isValid {
		return err
	}
	return schemaRepo.UpdateEnrichmentRule(rule)
}

func (ers *EnrichmentRuleService) DeleteEnrichmentRule(ruleId string) error {
	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	rule, _ := schemaRepo.GetProfileEnrichmentRule(ruleId)
	if rule.RuleId == "" {
		return nil
	}
	return schemaRepo.DeleteProfileEnrichmentRule(rule)
}

// validateEnrichmentRule validates the enrichment rule.
func validateEnrichmentRule(rule model.ProfileEnrichmentRule) (error, bool) {

	//  Required: Trait Name
	if rule.PropertyName == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrPropertyNameValidation.Code,
			Message:     errors.ErrPropertyNameValidation.Message,
			Description: errors.ErrPropertyNameValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	//  Required for Static: Value
	if rule.ComputationMethod == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrComputationValidation.Code,
			Message:     errors.ErrComputationValidation.Message,
			Description: "ComputationMethod type is required.",
		}, http.StatusBadRequest)
		return clientError, false
	}

	if rule.ComputationMethod == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrComputationValidation.Code,
			Message:     errors.ErrComputationValidation.Message,
			Description: "ComputationMethod type is required.",
		}, http.StatusBadRequest)
		return clientError, false
	}

	if !constants.AllowedComputationMethods[rule.ComputationMethod] {
		badReq := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrComputationValidation.Code,
			Message:     errors.ErrComputationValidation.Message,
			Description: fmt.Sprintf("'%s' is not an expected computation method.", rule.ComputationMethod),
		}, http.StatusBadRequest)
		return badReq, false
	}
	//  Required for Static: Value
	if rule.ComputationMethod == "static" && rule.Value == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrEnrichmentRuleValueValidation.Code,
			Message:     errors.ErrEnrichmentRuleValueValidation.Message,
			Description: errors.ErrEnrichmentRuleValueValidation.Description,
		}, http.StatusBadRequest)
		return clientError, false
	}

	if rule.ComputationMethod == "extract" && rule.SourceField == "" {
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrSourceFieldValidation.Code,
			Message:     errors.ErrSourceFieldValidation.Message,
			Description: errors.ErrSourceFieldValidation.Description,
		}, http.StatusBadRequest), false
	}

	if rule.ComputationMethod == "count" {
		if rule.TimeRange < 0 || rule.TimeRange > 2592000 {
			return errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrInvalidTime.Code,
				Message:     errors.ErrInvalidTime.Message,
				Description: "Time range should be from least 15 minutes to 30 days.",
			}, http.StatusBadRequest), false
		}
	}

	if rule.ComputationMethod != "count" && rule.TimeRange != 0 {
		return errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrInvalidTime.Code,
			Message:     errors.ErrInvalidTime.Message,
			Description: "Time range is only applicable for count computation.",
		}, http.StatusBadRequest), false
	}

	// Validate Time Range
	if rule.TimeRange == 0 {
		logger.Debug("Time range is not provided, defaulting to infinite denoted by -1.")
		rule.TimeRange = -1
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

	return nil, true
}
