package store

import (
	"database/sql"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"net/http"
	"strings"
	"time"
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
func (repo *ProfileSchemaRepository) AddEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()
	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `INSERT INTO profile_enrichment_rules 
		(rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err = tx.Exec(query,
		rule.RuleId, rule.PropertyName, rule.ValueType, rule.MergeStrategy, rule.Value, rule.ComputationMethod,
		rule.SourceField, rule.TimeRange, rule.Trigger.EventType, rule.Trigger.EventName, rule.CreatedAt,
		rule.UpdatedAt)

	if err != nil {
		tx.Rollback()
		return errors2.NewServerError(errors2.ErrWhileAddingEnrichmentRules, err)
	}

	for _, cond := range rule.Trigger.Conditions {
		_, err := tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			tx.Rollback()
			return errors2.NewServerError(errors2.ErrWhileAddingEnrichmentRules, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// UpdateEnrichmentRule updates an existing enrichment rule.
func (repo *ProfileSchemaRepository) UpdateEnrichmentRule(rule model.ProfileEnrichmentRule) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()
	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
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
		tx.Rollback()
		return errors2.NewServerError(errors2.ErrWhileUpdatingEnrichmentRules, err)
	}

	_, err = tx.Exec(`DELETE FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileUpdatingEnrichmentRules, err)

	}
	for _, cond := range rule.Trigger.Conditions {
		_, err := tx.Exec(`INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)`,
			rule.RuleId, cond.Field, cond.Operator, cond.Value)
		if err != nil {
			tx.Rollback()
			return errors2.NewServerError(errors2.ErrWhileUpdatingEnrichmentRules, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (repo *ProfileSchemaRepository) GetProfileEnrichmentRule(ruleId string) (model.ProfileEnrichmentRule, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return model.ProfileEnrichmentRule{}, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules WHERE rule_id = $1`

	var rule model.ProfileEnrichmentRule
	var createdAt, updatedAt int64

	results, err := dbClient.ExecuteQuery(query, ruleId)
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
		`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
	if err != nil {
		return rule, err
	}
	for _, row := range condResults {
		var cond model.RuleCondition
		cond.Field = row["field"].(string)
		cond.Operator = row["operator"].(string)
		cond.Value = row["value"].(string)

		rule.Trigger.Conditions = append(rule.Trigger.Conditions, cond)
	}
	return rule, nil
}

func (repo *ProfileSchemaRepository) GetProfileEnrichmentRules() ([]model.ProfileEnrichmentRule, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `SELECT rule_id, property_name, value_type, merge_strategy,
		value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at 
		FROM profile_enrichment_rules`

	rules := []model.ProfileEnrichmentRule{}

	results, err := dbClient.ExecuteQuery(query)
	if err != nil {
		return nil, err
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
			`SELECT field, operator, value FROM profile_enrichment_trigger_conditions WHERE rule_id = $1`, rule.RuleId)
		if err != nil {
			return nil, err
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

	return rules, nil
}

func (repo *ProfileSchemaRepository) DeleteProfileEnrichmentRule(rule model.ProfileEnrichmentRule) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	_, err = dbClient.ExecuteQuery(`DELETE FROM profile_enrichment_rules WHERE rule_id = $1`, rule.RuleId)
	if err == nil {
		// Delete unification rules if they exist on the same property name
		_, err = dbClient.ExecuteQuery(`DELETE FROM unification_rules WHERE property_name = $1`, rule.PropertyName)
	}
	return err
}

func (repo *ProfileSchemaRepository) GetEnrichmentRulesByFilter(filters []string) ([]model.ProfileEnrichmentRule, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
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
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.ErrInvalidFiltering.Code,
				Message:     errors2.ErrInvalidFiltering.Message,
				Description: fmt.Sprintf("unsupported field in filtering: %s", field),
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
		return nil, err
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
			return nil, err
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

	return filtered, nil
}
