package store

import (
	"database/sql"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"strconv"
	"strings"
	"time"
)

func AddUnificationRule(rule model.UnificationRule) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `INSERT INTO unification_rules (rule_id, rule_name, property, priority, is_active, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = dbClient.ExecuteQuery(query, query, rule.RuleId, rule.RuleName, rule.Property, rule.Priority, rule.IsActive,
		rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileCreatingUnificationRules, err)
	}

	logger.Info("Unification rule created successfully: " + rule.RuleName)
	return nil
}

func GetUnificationRules() ([]model.UnificationRule, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT rule_id, rule_name, property, priority, is_active, created_at, updated_at FROM unification_rules`
	results, err := dbClient.ExecuteQuery(query)
	if err != nil {
		logger.Info("Error occurred while fetching unification rules.")
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingUnificationRules, err)
	}

	var rules []model.UnificationRule
	for _, row := range results {
		var rule model.UnificationRule
		rule.RuleId = row["rule_id"].(string)
		rule.RuleName = row["rule_name"].(string)
		rule.Property = row["property"].(string)
		rule.Priority = int(row["priority"].(int64))
		rule.IsActive = row["is_active"].(bool)
		rule.CreatedAt = row["created_at"].(int64)
		rule.UpdatedAt = row["updated_at"].(int64)

		rules = append(rules, rule)
	}

	logger.Info("Successfully fetched unification rules")
	return rules, nil
}

func GetUnificationRule(ruleId string) (model.UnificationRule, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return model.UnificationRule{}, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT rule_id, rule_name, property, priority, is_active, created_at, updated_at FROM unification_rules WHERE rule_id = $1`
	results, err := dbClient.ExecuteQuery(query, ruleId)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("No unification rule found for rule_id: " + ruleId)
			return model.UnificationRule{}, nil
		}
		logger.Debug("Error occurred while fetching unification rule with rule_id: "+ruleId, err)
		return model.UnificationRule{}, errors2.NewServerError(errors2.ErrWhileFetchingUnificationRule, err)
	}

	row := results[0]
	var rule model.UnificationRule
	rule.RuleId = row["rule_id"].(string)
	rule.RuleName = row["rule_name"].(string)
	rule.Property = row["property"].(string)
	rule.Priority = int(row["priority"].(int64))
	rule.IsActive = row["is_active"].(bool)
	rule.CreatedAt = row["created_at"].(int64)
	rule.UpdatedAt = row["updated_at"].(int64)

	logger.Info("Successfully fetched unification rule for rule_id: " + ruleId)
	return rule, nil
}

func PatchUnificationRule(ruleId string, updates map[string]interface{}) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
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
		return errors2.NewServerError(errors2.ErrWhileUpdatingUnificationRule, err)
	}

	logger.Info("Successfully updated unification rule for rule_id: " + ruleId)
	return nil
}

func DeleteUnificationRule(ruleId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `DELETE FROM unification_rules WHERE rule_id = $1`
	_, err = dbClient.ExecuteQuery(query, ruleId)
	if err != nil {
		logger.Error(err, "Error while deleting unification rule for rule_id: "+ruleId)
		return err
	}

	logger.Info("Successfully deleted unification rule with rule_id: " + ruleId)
	return nil
}
