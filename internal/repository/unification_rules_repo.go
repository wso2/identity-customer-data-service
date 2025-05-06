package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/logger"
	"github.com/wso2/identity-customer-data-service/internal/models"
)

type UnificationRuleRepository struct {
	DB *sql.DB
}

func NewUnificationRuleRepository(db *sql.DB) *UnificationRuleRepository {
	return &UnificationRuleRepository{
		DB: db,
	}
}

func (repo *UnificationRuleRepository) AddUnificationRule(rule models.UnificationRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO unification_rules (rule_id, rule_name, property, priority, is_active, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := repo.DB.ExecContext(ctx, query, rule.RuleId, rule.RuleName, rule.Property, rule.Priority, rule.IsActive, rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileCreatingUnificationRules, err)
	}

	logger.Info("Unification rule created successfully: " + rule.RuleName)
	return nil
}

func (repo *UnificationRuleRepository) GetUnificationRules() ([]models.UnificationRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT rule_id, rule_name, property, priority, is_active, created_at, updated_at FROM unification_rules`
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		logger.Info("Error occurred while fetching unification rules.")
		return nil, errors.NewServerError(errors.ErrWhileFetchingUnificationRules, err)
	}
	defer rows.Close()

	var rules []models.UnificationRule
	for rows.Next() {
		var rule models.UnificationRule
		if err := rows.Scan(&rule.RuleId, &rule.RuleName, &rule.Property, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			logger.Debug("Error occurred while decoding unification rules.", err)
			return nil, errors.NewServerError(errors.ErrWhileFetchingUnificationRules, err)
		}
		rules = append(rules, rule)
	}

	logger.Info("Successfully fetched unification rules")
	return rules, nil
}

func (repo *UnificationRuleRepository) GetUnificationRule(ruleId string) (models.UnificationRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT rule_id, rule_name, property, priority, is_active, created_at, updated_at FROM unification_rules WHERE rule_id = $1`
	var rule models.UnificationRule
	if err := repo.DB.QueryRowContext(ctx, query, ruleId).Scan(&rule.RuleId, &rule.RuleName, &rule.Property, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			logger.Info("No unification rule found for rule_id: " + ruleId)
			return models.UnificationRule{}, nil
		}
		logger.Debug("Error occurred while fetching unification rule with rule_id: "+ruleId, err)
		return models.UnificationRule{}, errors.NewServerError(errors.ErrWhileFetchingUnificationRule, err)
	}

	logger.Info("Successfully fetched unification rule for rule_id: " + ruleId)
	return rule, nil
}

func (repo *UnificationRuleRepository) PatchUnificationRule(ruleId string, updates map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1
	for key, value := range updates {
		setClauses = append(setClauses, key+" = $"+string(argIndex))
		args = append(args, value)
		argIndex++
	}
	args = append(args, time.Now().Unix(), ruleId)

	query := `UPDATE unification_rules SET ` + strings.Join(setClauses, ", ") + `, updated_at = $` + string(argIndex) + ` WHERE rule_id = $` + string(argIndex+1)
	_, err := repo.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileUpdatingUnificationRule, err)
	}

	logger.Info("Successfully updated unification rule for rule_id: " + ruleId)
	return nil
}

func (repo *UnificationRuleRepository) DeleteUnificationRule(ruleId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM unification_rules WHERE rule_id = $1`
	_, err := repo.DB.ExecContext(ctx, query, ruleId)
	if err != nil {
		logger.Error(err, "Error while deleting unification rule for rule_id: "+ruleId)
		return err
	}

	logger.Info("Successfully deleted unification rule with rule_id: " + ruleId)
	return nil
}
