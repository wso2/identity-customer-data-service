package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/models"
)

type ProfileSchemaRepository struct {
	DB *sql.DB
}

func NewProfileSchemaRepository(db *sql.DB) *ProfileSchemaRepository {
	return &ProfileSchemaRepository{
		DB: db,
	}
}

// AddEnrichmentRule adds a new enrichment rule
func (repo *ProfileSchemaRepository) AddEnrichmentRule(rule models.ProfileEnrichmentRule) error {

	tx, err := repo.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO profile_enrichment_rules 
		(rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err = tx.Exec(query,
		rule.RuleId, rule.PropertyName, rule.ValueType, rule.MergeStrategy, rule.Value, rule.ComputationMethod,
		rule.SourceField, rule.TimeRange, rule.Trigger.EventType, rule.Trigger.EventName, rule.CreatedAt,
		rule.UpdatedAt)

	if err != nil {
		return errors.NewServerError(errors.ErrWhileAddingEnrichmentRules, err)
	}

	for _, cond := range rule.Trigger.Conditions {
		_, err := tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileAddingEnrichmentRules, err)
		}
	}

	return tx.Commit()
}

// UpdateEnrichmentRule updates an existing enrichment rule.
func (repo *ProfileSchemaRepository) UpdateEnrichmentRule(rule models.ProfileEnrichmentRule) error {

	tx, err := repo.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timestamp := time.Now().UTC().Unix()

	query := `UPDATE profile_enrichment_rules SET 
		property_name=$1, value_type=$2, merge_strategy=$3, source_field= $4, value=$5, computation_method=$6, time_range=$7, 
		event_type=$8, event_name=$9, updated_at=$10
		WHERE rule_id=$11`

	_, err = tx.Exec(query,
		rule.PropertyName, rule.ValueType, rule.MergeStrategy, rule.SourceField, rule.Value, rule.ComputationMethod, rule.TimeRange,
		rule.Trigger.EventType, rule.Trigger.EventName, timestamp, rule.RuleId)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileUpdatingEnrichmentRules, err)
	}

	_, err = tx.Exec(`DELETE FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileUpdatingEnrichmentRules, err)

	}
	for _, cond := range rule.Trigger.Conditions {
		_, err := tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileUpdatingEnrichmentRules, err)
		}
	}

	return tx.Commit()
}

func (repo *ProfileSchemaRepository) GetProfileEnrichmentRule(ruleId string) (models.ProfileEnrichmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules WHERE rule_id = $1`

	var rule models.ProfileEnrichmentRule
	var createdAt, updatedAt int64

	row := repo.DB.QueryRowContext(ctx, query, ruleId)
	err := row.Scan(&rule.RuleId, &rule.PropertyName, &rule.ValueType, &rule.MergeStrategy, &rule.Value,
		&rule.ComputationMethod, &rule.SourceField, &rule.TimeRange, &rule.Trigger.EventType, &rule.Trigger.EventName,
		&createdAt, &updatedAt)

	if err != nil {
		return rule, err
	}

	rule.CreatedAt = createdAt
	rule.UpdatedAt = updatedAt

	// Fetch trigger conditions
	condRows, err := repo.DB.QueryContext(ctx,
		`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		return rule, err
	}
	for condRows.Next() {
		var cond models.RuleCondition
		if err := condRows.Scan(&cond.Field, &cond.Operator, &cond.Value); err != nil {
			return rule, err
		}
		rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
	}
	condRows.Close()
	return rule, nil
}

func (repo *ProfileSchemaRepository) GetProfileEnrichmentRules() ([]models.ProfileEnrichmentRule, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT rule_id, property_name, value_type, merge_strategy,
		value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules`

	rules := []models.ProfileEnrichmentRule{}

	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		rule := models.ProfileEnrichmentRule{}
		var createdAt, updatedAt int64
		err := rows.Scan(&rule.RuleId, &rule.PropertyName, &rule.ValueType, &rule.MergeStrategy,
			&rule.Value,
			&rule.ComputationMethod, &rule.SourceField, &rule.TimeRange, &rule.Trigger.EventType, &rule.Trigger.EventName,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		rule.CreatedAt = createdAt
		rule.UpdatedAt = updatedAt

		condRows, err := repo.DB.QueryContext(ctx,
			`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
		if err != nil {
			return nil, err
		}
		for condRows.Next() {
			var cond models.RuleCondition
			err := condRows.Scan(&cond.Field, &cond.Operator, &cond.Value)
			if err != nil {
				return nil, err
			}
			rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
		}
		condRows.Close()
		rules = append(rules, rule)
	}

	return rules, nil
}

func (repo *ProfileSchemaRepository) DeleteProfileEnrichmentRule(rule models.ProfileEnrichmentRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := repo.DB.ExecContext(ctx, `DELETE FROM profile_enrichment_rules WHERE rule_id = $1`, rule.RuleId)
	if err == nil {
		// Delete unification rules if they exist on the same property name
		_, err = repo.DB.ExecContext(ctx, `DELETE FROM unification_rules WHERE property_name = $1`, rule.PropertyName)
	}
	return err
}

func (repo *ProfileSchemaRepository) GetEnrichmentRulesByFilter(filters []string) ([]models.ProfileEnrichmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
			return nil, fmt.Errorf("invalid filter format: %s", f)
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
			clientError := errors.NewClientError(errors.ErrorMessage{
				Code:        errors.ErrInvalidFiltering.Code,
				Message:     errors.ErrInvalidFiltering.Message,
				Description: fmt.Sprintf("unsupported field in filtering: %s", field),
			}, http.StatusBadRequest)
			return nil, clientError
		}
	}

	finalQuery := baseQuery
	if len(whereClauses) > 0 {
		finalQuery += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	rules := []models.ProfileEnrichmentRule{}
	ruleMap := make(map[string]*models.ProfileEnrichmentRule)

	rows, err := repo.DB.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		rule := models.ProfileEnrichmentRule{}
		var createdAt, updatedAt int64
		err := rows.Scan(&rule.RuleId, &rule.PropertyName, &rule.ValueType, &rule.MergeStrategy,
			&rule.Value, &rule.ComputationMethod, &rule.SourceField, &rule.TimeRange, &rule.Trigger.EventType, &rule.Trigger.EventName,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		rule.CreatedAt = createdAt
		rule.UpdatedAt = updatedAt
		// Temporarily store in map to avoid duplicate fetches
		ruleMap[rule.RuleId] = &rule
		rules = append(rules, rule)
	}

	// Fetch and attach trigger conditions
	for _, rule := range rules {
		condRows, err := repo.DB.QueryContext(ctx,
			`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
		if err != nil {
			return nil, err
		}
		for condRows.Next() {
			var cond models.RuleCondition
			err := condRows.Scan(&cond.Field, &cond.Operator, &cond.Value)
			if err != nil {
				condRows.Close()
				return nil, err
			}
			rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
		}
		condRows.Close()
	}

	// Apply post-query filters for special fields
	filtered := []models.ProfileEnrichmentRule{}
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

	return filtered, nil
}
