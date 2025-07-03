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
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"strconv"
	"strings"
	"time"
)

// AddUnificationRule adds a new unification rule to the database
func AddUnificationRule(rule model.UnificationRule, orgId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for adding unification rule: %s", rule.RuleName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DB_CLIENT_INIT.Code,
			Message:     errors2.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.InsertUnificationRule[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query, rule.RuleId, orgId, rule.RuleName, rule.Property, rule.Priority, rule.IsActive,
		rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while adding unification rule: %s", rule.RuleName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.EXECUTE_QUERY.Code,
			Message:     errors2.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info(fmt.Sprintf("Unification rule : %s added successfully", rule.RuleName))
	return nil
}

// GetUnificationRules fetches all unification rules from the database
func GetUnificationRules(tenantId string) ([]model.UnificationRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching unification rules for organization: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DB_CLIENT_INIT.Code,
			Message:     errors2.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetUnificationRules[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, tenantId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed in fetching all unification rules for organization: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.EXECUTE_QUERY.Code,
			Message:     errors2.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var rules []model.UnificationRule
	for _, row := range results {
		var rule model.UnificationRule
		rule.RuleId = row["rule_id"].(string)
		rule.RuleName = row["rule_name"].(string)
		rule.Property = row["property_name"].(string)
		rule.Priority = int(row["priority"].(int64))
		rule.IsActive = row["is_active"].(bool)
		rule.CreatedAt = row["created_at"].(int64)
		rule.UpdatedAt = row["updated_at"].(int64)

		rules = append(rules, rule)
	}

	logger.Info(fmt.Sprintf("Successfully fetched all unification rules for organization: %s", tenantId))
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
			Code:        errors2.DB_CLIENT_INIT.Code,
			Message:     errors2.DB_CLIENT_INIT.Message,
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
			Code:        errors2.EXECUTE_QUERY.Code,
			Message:     errors2.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No unification rule found for rule_id: %s ", ruleId))
		return nil, nil
	}

	row := results[0]
	var rule model.UnificationRule
	rule.RuleId = row["rule_id"].(string)
	rule.RuleName = row["rule_name"].(string)
	rule.Property = row["property_name"].(string)
	rule.Priority = int(row["priority"].(int64))
	rule.IsActive = row["is_active"].(bool)
	rule.CreatedAt = row["created_at"].(int64)
	rule.UpdatedAt = row["updated_at"].(int64)

	logger.Info("Successfully fetched unification rule for rule_id: " + ruleId)
	return &rule, nil
}

// PatchUnificationRule applies partial updates to a unification rule.
func PatchUnificationRule(ruleId string, updates map[string]interface{}) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating unification rule: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DB_CLIENT_INIT.Code,
			Message:     errors2.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1
	for key, value := range updates {
		setClauses = append(setClauses, key+" = $"+strconv.Itoa(argIndex))
		args = append(args, value)
		argIndex++
	}
	args = append(args, time.Now().Unix(), ruleId)

	query := `UPDATE unification_rules SET ` + strings.Join(setClauses, ", ") + `, updated_at = $` + strconv.Itoa(argIndex) + ` WHERE rule_id = $` + strconv.Itoa(argIndex+1)
	_, err = dbClient.ExecuteQuery(query, args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while updating unification rule for rule_id: %s", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.EXECUTE_QUERY.Code,
			Message:     errors2.EXECUTE_QUERY.Message,
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
			Code:        errors2.DB_CLIENT_INIT.Code,
			Message:     errors2.DB_CLIENT_INIT.Message,
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
			Code:        errors2.EXECUTE_QUERY.Code,
			Message:     errors2.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info("Successfully deleted unification rule with rule_id: " + ruleId)
	return nil
}
