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
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
	"time"
)

// AddEnrichmentRule adds a new enrichment rule
func AddEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for adding enrichment rule for property: %s",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_ENRICHMENT_RULE.Code,
			Message:     errors2.ADD_ENRICHMENT_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()
	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for adding enrichment rule for property: %s",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_ENRICHMENT_RULE.Code,
			Message:     errors2.ADD_ENRICHMENT_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := `INSERT INTO profile_enrichment_rules 
		(rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err = tx.Exec(query,
		rule.RuleId, rule.PropertyName, rule.ValueType, rule.MergeStrategy, rule.Value, rule.ComputationMethod,
		rule.SourceField, rule.TimeRange, rule.Trigger.EventType, rule.Trigger.EventName, rule.CreatedAt,
		rule.UpdatedAt)

	if err != nil {
		err = tx.Rollback()
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to rollback transaction for adding enrichment rule for property: %s",
				rule.PropertyName)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_ENRICHMENT_RULE.Code,
				Message:     errors2.ADD_ENRICHMENT_RULE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		errorMsg := fmt.Sprintf("Failed on inserting enrichment rule for property: %s", rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_ENRICHMENT_RULE.Code,
			Message:     errors2.ADD_ENRICHMENT_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	for _, cond := range rule.Trigger.Conditions {
		_, err = tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				errorMsg := fmt.Sprintf("Failed to rollback transaction for trigger conditions for the"+
					" enrichment rule for property: %s",
					rule.PropertyName)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.ADD_ENRICHMENT_RULE.Code,
					Message:     errors2.ADD_ENRICHMENT_RULE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			errorMsg := fmt.Sprintf("Failed on inserting trigger conditions for the enrichment rule "+
				"for property: %s", rule.PropertyName)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_ENRICHMENT_RULE.Code,
				Message:     errors2.ADD_ENRICHMENT_RULE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}

	if err := tx.Commit(); err != nil {
		errorMsg := fmt.Sprintf("Failed on commiting transaction while adding enrichment rule "+
			"for property: %s", rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_ENRICHMENT_RULE.Code,
			Message:     errors2.ADD_ENRICHMENT_RULE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Enrichment rule: %s for property: %s added successfully.", rule.RuleId,
		rule.PropertyName))
	return nil
}

// UpdateEnrichmentRule updates an existing enrichment rule.
func UpdateEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating enrichment rule for property: %s.",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
			Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()
	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to beging transaction for updating enrichment rule for property: %s.",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
			Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	timestamp := time.Now().UTC().Unix()

	query := `UPDATE profile_enrichment_rules SET 
		property_name=$1, value_type=$2, merge_strategy=$3, source_field= $4, value=$5, computation_method=$6, time_range=$7, 
		event_type=$8, event_name=$9, updated_at=$10
		WHERE rule_id=$11`

	_, err = tx.Exec(query,
		rule.PropertyName, rule.ValueType, rule.MergeStrategy, rule.SourceField, rule.Value, rule.ComputationMethod, rule.TimeRange,
		rule.Trigger.EventType, rule.Trigger.EventName, timestamp, rule.RuleId)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to rollback while updating enrichment rule for property: %s.",
				rule.PropertyName)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
				Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		errorMsg := fmt.Sprintf("Failed to update enrichment rule for property: %s.", rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
			Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	_, err = tx.Exec(`DELETE FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to update trigger conditions of enrichment rule for property: %s.",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
			Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError

	}
	for _, cond := range rule.Trigger.Conditions {
		_, err := tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				errorMsg := fmt.Sprintf("Failed to rollback updating trigger conditions of enrichment rule for "+
					"property: %s.", rule.PropertyName)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
					Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			errorMsg := fmt.Sprintf("Failed to update trigger conditions of the enrichment rule for property: %s.",
				rule.PropertyName)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
				Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}

	if err := tx.Commit(); err != nil {
		errorMsg := fmt.Sprintf("Failed to commit transaction for updating enrichment rule for property: %s.",
			rule.PropertyName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ENRICHMENT_RULES.Code,
			Message:     errors2.UPDATE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Enrichment rule: %s for property: %s updated successfully.", rule.RuleId,
		rule.PropertyName))
	return nil
}

// GetProfileEnrichmentRule fetches a specific enrichment rule by its ID.
func GetProfileEnrichmentRule(ruleId string) (*model.ProfileEnrichmentRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching enrichment rule with "+
			"enrichment rule id: %s.", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
			Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := `SELECT rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, 
       time_range, event_type, event_name, created_at, updated_at FROM profile_enrichment_rules WHERE rule_id = $1`

	var rule model.ProfileEnrichmentRule
	var createdAt, updatedAt int64

	results, err := dbClient.ExecuteQuery(query, ruleId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch enrichment rule with rule id: %s.", ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
			Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No enrichment rule found for rule id: %s ", ruleId))
		return nil, nil
	}
	row := results[0]

	rule.RuleId = row["rule_id"].(string)
	rule.PropertyName = row["property_name"].(string)
	rule.ValueType = row["value_type"].(string)
	rule.MergeStrategy = row["merge_strategy"].(string)
	rule.Value = row["value"]
	rule.ComputationMethod = row["computation_method"].(string)
	rule.SourceField = row["source_field"].(string)
	rule.TimeRange = row["time_range"].(int64)
	rule.Trigger.EventType = row["event_type"].(string)
	rule.Trigger.EventName = row["event_name"].(string)
	createdAt = row["created_at"].(int64)
	updatedAt = row["updated_at"].(int64)

	rule.CreatedAt = createdAt
	rule.UpdatedAt = updatedAt

	// Fetch trigger conditions
	condResults, err := dbClient.ExecuteQuery(
		`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`,
		rule.RuleId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch trigger conditions for enrichment rule with rule id: %s.",
			ruleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
			Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return &rule, serverError
	}
	for _, row := range condResults {
		var cond model.RuleCondition
		cond.Field = row["field"].(string)
		cond.Operator = row["operator"].(string)
		cond.Value = row["value"].(string)

		rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
	}
	logger.Info(fmt.Sprintf("Enrichment rule: %s for property: %s fetched successfully.", rule.RuleId,
		rule.PropertyName))
	return &rule, nil
}

// GetProfileEnrichmentRules fetches all enrichment rules.
func GetProfileEnrichmentRules() ([]model.ProfileEnrichmentRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client for fetching enrichment rules."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
			Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := `SELECT rule_id, property_name, value_type, merge_strategy,
		value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules`

	rules := []model.ProfileEnrichmentRule{}

	results, err := dbClient.ExecuteQuery(query)
	if err != nil {
		errorMsg := "Failed to fetch enrichment rules."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
			Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	for _, row := range results {
		rule := model.ProfileEnrichmentRule{}
		var createdAt, updatedAt int64

		rule.RuleId = row["rule_id"].(string)
		rule.PropertyName = row["property_name"].(string)
		rule.ValueType = row["value_type"].(string)
		rule.MergeStrategy = row["merge_strategy"].(string)
		rule.Value = row["value"]
		rule.ComputationMethod = row["computation_method"].(string)
		rule.SourceField = row["source_field"].(string)
		rule.TimeRange = row["time_range"].(int64)
		rule.Trigger.EventType = row["event_type"].(string)
		rule.Trigger.EventName = row["event_name"].(string)

		rule.CreatedAt = createdAt
		rule.UpdatedAt = updatedAt

		condResults, err := dbClient.ExecuteQuery(
			`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`,
			rule.RuleId)
		if err != nil {
			errorMsg := "Failed to fetch trigger conditions for enrichment rule(s)."
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.FETCH_ENRICHMENT_RULES.Code,
				Message:     errors2.FETCH_ENRICHMENT_RULES.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		for _, condRow := range condResults {
			var cond model.RuleCondition

			cond.Field = condRow["field"].(string)
			cond.Operator = condRow["operator"].(string)
			cond.Value = condRow["value"].(string)
			rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
		}
		rules = append(rules, rule)
	}
	logger.Info("Fetching Enrichment rules successful.")
	return rules, nil
}

// DeleteProfileEnrichmentRule deletes an enrichment rule by its ID.
func DeleteProfileEnrichmentRule(rule *model.ProfileEnrichmentRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting enrichment rule with rule id: %s",
			rule.RuleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_ENRICHMENT_RULES.Code,
			Message:     errors2.DELETE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	_, err = dbClient.ExecuteQuery(`DELETE FROM profile_enrichment_rules WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to delete enrichment rule: %s", rule.RuleId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_ENRICHMENT_RULES.Code,
			Message:     errors2.DELETE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	// Delete unification rules if they exist on the same property name
	_, err = dbClient.ExecuteQuery(`DELETE FROM unification_rules WHERE property_name = $1`, rule.PropertyName)
	if err != nil {
		errorMsg := "Failed to delete unification rules."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_ENRICHMENT_RULES.Code,
			Message:     errors2.DELETE_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Enrichment rule: %s for property: %s deleted successfully.", rule.RuleId,
		rule.PropertyName))
	return nil
}

func GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client for filtering enrichment rules."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FILTER_ENRICHMENT_RULES.Code,
			Message:     errors2.FILTER_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	baseQuery := `SELECT rule_id, property_name, value_type, merge_strategy,
		 value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules`

	var whereClauses []string
	var args []interface{}
	argIndex := 1

	specialClauses := make(map[string][]string)

	for _, f := range filters {
		tokens := strings.SplitN(f, " ", 3)
		if len(tokens) != 3 {
			// todo: see if this is client or server error
			errorMsg := "Invalid filter format."
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.FILTER_ENRICHMENT_RULES.Code,
				Message:     errors2.FILTER_ENRICHMENT_RULES.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}

		field := tokens[0]
		operator := strings.ToLower(tokens[1])
		value := strings.TrimSpace(tokens[2])

		switch field {
		case "property_name", "value_type", "merge_strategy", "source_field", "trigger.event_type", "trigger.event_name":
			dbField := map[string]string{
				"trigger.event_type": "event_type",
				"trigger.event_name": "event_name",
			}[field]
			if dbField == "" {
				dbField = field
			}
			switch operator {
			case "eq":
				whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", dbField, argIndex))
				args = append(args, value)
			case "co":
				whereClauses = append(whereClauses, fmt.Sprintf("%s ILIKE $%d", dbField, argIndex))
				args = append(args, "%"+value+"%")
			case "sw":
				whereClauses = append(whereClauses, fmt.Sprintf("%s ILIKE $%d", dbField, argIndex))
				args = append(args, value+"%")
			default:
				return nil, fmt.Errorf("unsupported operator for field %s: %s", field, operator)
			}
			argIndex++
		case "trigger.conditions.field", "trigger.conditions.value":
			// Collect for post-query filtering
			specialClauses[field] = append(specialClauses[field], value)
		default:
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ENRICHMENT_RULE_FILTERING.Code,
				Message:     errors2.INVALID_ENRICHMENT_RULE_FILTERING.Message,
				Description: fmt.Sprintf("Unsupported field in filtering: %s", field),
			}, http.StatusBadRequest)
			return nil, clientError
		}
	}

	finalQuery := baseQuery
	if len(whereClauses) > 0 {
		finalQuery += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	rules := []model.ProfileEnrichmentRule{}
	ruleMap := make(map[string]*model.ProfileEnrichmentRule)

	results, err := dbClient.ExecuteQuery(finalQuery, args...)
	if err != nil {
		errorMsg := "Failed to execute query for filtering enrichment rules."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FILTER_ENRICHMENT_RULES.Code,
			Message:     errors2.FILTER_ENRICHMENT_RULES.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	for _, row := range results {
		rule := model.ProfileEnrichmentRule{}
		var createdAt, updatedAt int64
		rule.RuleId = row["rule_id"].(string)
		rule.PropertyName = row["property_name"].(string)
		rule.ValueType = row["value_type"].(string)
		rule.MergeStrategy = row["merge_strategy"].(string)
		rule.Value = row["value"]
		rule.ComputationMethod = row["computation_method"].(string)
		rule.SourceField = row["source_field"].(string)
		rule.TimeRange = row["time_range"].(int64)
		rule.Trigger.EventType = row["event_type"].(string)
		rule.Trigger.EventName = row["event_name"].(string)
		rule.CreatedAt = createdAt
		rule.UpdatedAt = updatedAt
		// Temporarily store in map to avoid duplicate fetches
		ruleMap[rule.RuleId] = &rule
		rules = append(rules, rule)
	}

	// Fetch and attach trigger conditions
	for _, rule := range rules {
		condResults, err := dbClient.ExecuteQuery(
			`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
		if err != nil {
			errorMsg := "Failed to execute query for fetching trigger conditions for filtering enrichment rules."
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.FILTER_ENRICHMENT_RULES.Code,
				Message:     errors2.FILTER_ENRICHMENT_RULES.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		for _, condRow := range condResults {
			var cond model.RuleCondition
			cond.Field = condRow["field"].(string)
			cond.Operator = condRow["operator"].(string)
			cond.Value = condRow["value"].(string)

			rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
		}
	}

	// Apply post-query filters for special fields
	filtered := []model.ProfileEnrichmentRule{}
Outer:
	for _, rule := range rules {
		for key, vals := range specialClauses {
			switch key {
			case "trigger.conditions.field":
				found := false
				for _, val := range vals {
					for _, cond := range rule.Trigger.Conditions {
						if cond.Field == val {
							found = true
							break
						}
					}
				}
				if !found {
					continue Outer
				}
			case "trigger.conditions.value":
				found := false
				for _, val := range vals {
					for _, cond := range rule.Trigger.Conditions {
						if cond.Value == val {
							found = true
							break
						}
					}
				}
				if !found {
					continue Outer
				}
			}
		}
		filtered = append(filtered, rule)
	}

	logger.Info("Enrichment rules filtered successfully.")
	return filtered, nil
}
