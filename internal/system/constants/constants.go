package constants

import "time"

// Collection names
const (
	EventCollection   = "events"
	ProfileCollection = "profiles"
)
const MaxRetryAttempts = 10
const RetryDelay = 100 * time.Millisecond
const ApiBasePath = "/api/v1"
const Filter = "filter"
const DEFAULT_LOG_LEVEL = "INFO"

var AllowedFieldsForUnificationRulePatch = map[string]bool{
	"is_active": true,
	"priority":  true,
	"rule_name": true,
}

var AllowedPropertyTypes = map[string]bool{
	"string":        true,
	"int":           true,
	"boolean":       true,
	"date":          true,
	"arrayOfString": true,
	"arrayOfInt":    true,
}

var GoTypeMapping = map[string]string{
	"string":        "string",
	"int":           "int",
	"boolean":       "bool",
	"arrayofstring": "[]string",
	"arrayofint":    "[]int",
}

var AllowedComputationMethods = map[string]bool{
	"static":  true,
	"extract": true,
	"count":   true,
}

var AllowedMergeStrategies = map[string]bool{
	"overwrite": true,
	"combine":   true,
	"ignore":    true,
}

var AllowedConditionOperators = map[string]bool{
	"equals":              true,
	"not_equals":          true,
	"exists":              true,
	"not_exists":          true,
	"contains":            true,
	"not_contains":        true,
	"greater_than":        true,
	"greater_than_equals": true,
	"less_than":           true,
	"less_than_equals":    true,
}

var ApiKeyStates = map[string]bool{
	"active":  true,
	"revoked": true,
	"expired": true,
}
