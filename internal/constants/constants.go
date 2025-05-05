package constants

import "time"

// Collection names
const (
	UnificationRulesCollection = "resolution_rules"
	EventCollection            = "events"
	ProfileCollection          = "profiles"
	ProfileSchemaCollection    = "profile_schema"
)
const MaxRetryAttempts = 10
const RetryDelay = 100 * time.Millisecond
const ApiBasePath = "/api/v1"
const Filter = "filter"

const (
	TokenEndpoint      = "/oauth2/token"
	RevocationEndpoint = "/oauth2/revoke"
)

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

var AllowedTraitTypes = map[string]bool{
	"static":   true,
	"computed": true,
}

var AllowedMergeStrategies = map[string]bool{
	"overwrite": true,
	"combine":   true,
	"ignore":    true,
}

var AllowedMaskingStrategies = map[string]bool{
	"partial": true,
	"hash":    true,
	"redact":  true,
}

var AllowedEventTypes = map[string]bool{
	"track":    true,
	"identify": true,
	"page":     true,
}

var AllowedProfileDataScopes = map[string]bool{
	"identity":    true,
	"personality": true,
	"app_context": true,
	"session":     true,
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
