package database

const (
	AddUnificationRuleQuery = `
		INSERT INTO unification_rules 
		(rule_id, rule_name, property_name, priority, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	GetUnificationRulesQuery = `
		SELECT rule_id, rule_name, property_name, priority, is_active, created_at, updated_at
		FROM unification_rules
	`

	GetUnificationRuleByIdQuery = `
		SELECT rule_id, rule_name, property_name, priority, is_active, created_at, updated_at
		FROM unification_rules WHERE rule_id = $1
	`

	DeleteUnificationRuleQuery = `
		DELETE FROM unification_rules WHERE rule_id = $1
	`

	AddEnrichmentRuleQuery = `
		INSERT INTO profile_enrichment_rules 
		(rule_id, property_name, value_type, merge_strategy, value, computation_method, source_field, time_range, event_type, event_name, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`

	UpdateEnrichmentRuleQuery = `
		UPDATE profile_enrichment_rules SET 
		property_name=$1, value_type=$2, merge_strategy=$3, source_field=$4, value=$5, computation_method=$6, time_range=$7, 
		event_type=$8, event_name=$9, updated_at=$10
		WHERE rule_id=$11
	`

	DeleteEnrichmentTriggerConditionsQuery = `
		DELETE FROM profile_enrichment_trigger_conditions WHERE rule_id = $1
	`

	InsertEnrichmentTriggerConditionQuery = `
		INSERT INTO profile_enrichment_trigger_conditions 
		(rule_id, field, operator, value) VALUES ($1, $2, $3, $4)
	`
)
