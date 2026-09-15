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

package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

// AddUnificationRule adds a new unification rule to the database
func AddUnificationRule(rule model.UnificationRule, orgId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for adding unification rule: %s", rule.RuleName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.InsertUnificationRule[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query, rule.RuleId, orgId, rule.RuleName, rule.PropertyName, rule.PropertyId, rule.Priority, rule.IsActive,
		rule.AttributeType, rule.UnificationMethod, rule.MatchStrength, rule.MismatchStrength, rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while adding unification rule: %s", rule.RuleName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_UNIFICATION_RULE.Code,
			Message:     errors2.ADD_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info(fmt.Sprintf("Unification rule : '%s' added successfully", rule.RuleName))
	return nil
}

// GetUnificationRules fetches all unification rules from the database
func GetUnificationRules(orgHandle string) ([]model.UnificationRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching unification rules for organization: %s", orgHandle)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_UNIFICATION_RULE.Code,
			Message:     errors2.GET_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetUnificationRules[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, orgHandle)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed in fetching all unification rules for organization: %s", orgHandle)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_UNIFICATION_RULE.Code,
			Message:     errors2.GET_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var rules []model.UnificationRule
	for _, row := range results {
		rules = append(rules, scanUnificationRule(row))
	}

	logger.Info(fmt.Sprintf("Successfully fetched all unification rules for organization: %s", orgHandle))
	return rules, nil
}

// GetUnificationRule fetches a specific unification rule by its Id
func GetUnificationRule(ruleId string) (*model.UnificationRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching unification rule: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_UNIFICATION_RULE.Code,
			Message:     errors2.GET_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetUnificationRule[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, ruleId)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debug(fmt.Sprintf("No unification rule found for rule_id: %s ", ruleId))
			return nil, nil
		}
		errorMsg := fmt.Sprintf("Failed in fetching unification rule with rule_id: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_UNIFICATION_RULE.Code,
			Message:     errors2.GET_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No unification rule found for rule_id: %s ", ruleId))
		return nil, nil
	}

	rule := scanUnificationRule(results[0])

	logger.Info("Successfully fetched unification rule for rule_id: " + ruleId)
	return &rule, nil
}

// PatchUnificationRule applies partial updates to a unification rule.
func PatchUnificationRule(ruleId string, updatedRule model.UnificationRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating unification rule: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_UNIFICATION_RULE.Code,
			Message:     errors2.UPDATE_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.UpdateUnificationRule[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, updatedRule.RuleName, updatedRule.Priority, updatedRule.IsActive, updatedRule.AttributeType,
		updatedRule.UnificationMethod, updatedRule.MatchStrength, updatedRule.MismatchStrength, time.Now().UTC(), ruleId)

	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while updating unification rule for rule_id: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_UNIFICATION_RULE.Code,
			Message:     errors2.UPDATE_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info("Successfully updated unification rule for rule_id: " + ruleId)
	return nil
}

// DeleteUnificationRule deletes a unification rule by its Id
func DeleteUnificationRule(ruleId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating unification rule: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_UNIFICATION_RULE.Code,
			Message:     errors2.DELETE_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.DeleteUnificationRule[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, ruleId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to delete unification rule: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_UNIFICATION_RULE.Code,
			Message:     errors2.DELETE_UNIFICATION_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info("Successfully deleted unification rule with rule_id: " + ruleId)
	return nil
}

// scanUnificationRule reads one row without asserting column types outright.
//
// A bare assertion panics on an unexpected NULL, and this runs on the unification worker's
// goroutine where a panic would take the process down rather than fail one request.
// property_id in particular is a nullable foreign key, so the value really can be nil.
func scanUnificationRule(row map[string]interface{}) model.UnificationRule {
	var rule model.UnificationRule

	rule.RuleId = stringColumn(row, "rule_id")
	rule.RuleName = stringColumn(row, "rule_name")
	rule.PropertyName = stringColumn(row, "property_name")
	rule.PropertyId = stringColumn(row, "property_id")
	rule.AttributeType = stringColumn(row, "attribute_type")
	rule.UnificationMethod = stringColumn(row, "unification_method")
	rule.MatchStrength = stringColumn(row, "match_strength")
	rule.MismatchStrength = stringColumn(row, "mismatch_strength")
	rule.Priority = intColumn(row, "priority")

	if value, ok := row["is_active"].(bool); ok {
		rule.IsActive = value
	}
	if value, ok := row["created_at"].(time.Time); ok {
		rule.CreatedAt = value
	}
	if value, ok := row["updated_at"].(time.Time); ok {
		rule.UpdatedAt = value
	}

	return rule
}

func stringColumn(row map[string]interface{}, column string) string {
	switch value := row[column].(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}

func intColumn(row map[string]interface{}, column string) int {
	switch value := row[column].(type) {
	case int64:
		return int(value)
	case int32:
		return int(value)
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}
