package constants

const ApiBasePath = "/api/v1"
const ProfileApiPath = "profiles"
const UnificationRulesApiPath = "unification-rules"
const ConsentApiPath = "consent"
const ProfileSchemaApiPath = "profile-schema"
const IdentityServerDialectsPath = "/api/server/v1/claim-dialects"
const Filter = "filter"

type contextKey string

const TenantContextKey contextKey = "tenant"

var AllowedFieldsForUnificationRulePatch = map[string]bool{
	"is_active": true,
	"priority":  true,
}

const (
	StringDataType   = "string"
	IntegerDataType  = "integer"
	DecimalDataType  = "decimal"
	BooleanDataType  = "boolean"
	DateTimeDataType = "date_time"
	ComplexDataType  = "complex"
)

const (
	SchemaInitEvent = "schema-initialization"
)

const (
	update = "update" // Update an existing profile with new data.
	create = "create" // Create a new profile with the provided data.
	sync   = "sync"
)

var AllowedValueTypes = map[string]bool{
	StringDataType:   true,
	IntegerDataType:  true,
	DecimalDataType:  true,
	BooleanDataType:  true,
	DateTimeDataType: true,
	ComplexDataType:  true,
}

// Mutability values define how an attribute can be created, read, or updated.
const (
	MutabilityReadWrite = "readWrite" // Can be both read and updated freely.
	MutabilityReadOnly  = "readOnly"  // Can be read but not updated (system-set or computed).
	MutabilityWriteOnly = "writeOnly" // Can be written but not read back (e.g., passwords).
	MutabilityImmutable = "immutable" // Must be set at creation and cannot be changed later.
	MutabilityWriteOnce = "writeOnce" // Can be empty initially, but once set, cannot be updated.
	MutabilityComputed  = "computed"  // Value is derived or calculated, not directly stored.
)

const (
	MergeStrategyOverwrite = "overwrite" // Overwrite the existing value with the new one.
	MergeStrategyLatest    = "latest"
	MergeStrategyCombine   = "combine" // Combine values from both profiles (e.g., arrays).
	MergeStrategyOldest    = "oldest"  // Use the oldest value from the profiles being merged.
)

// AllowedMutabilityValues defines the valid set of mutability types.
var AllowedMutabilityValues = map[string]bool{
	MutabilityReadWrite: true, // Can be both read and updated freely.
	MutabilityReadOnly:  true, // Can be read but not updated (system-set or computed).
	MutabilityWriteOnly: true, // Can be written but not read back (e.g., passwords).
	MutabilityImmutable: true, // Must be set at creation and cannot be changed later (created time)
	MutabilityWriteOnce: true, // Can be empty initially, but once set, cannot be updated. (userId)
}

var AllowedAttributesScope = map[string]bool{
	IdentityAttributes: true,
	Traits:             true,
	ApplicationData:    true,
}

const (
	IdentityAttributes = "identity_attributes"
	Traits             = "traits"
	ApplicationData    = "application_data"
)

const (
	ValueType  = "value_type"
	Mutability = "mutability"
)

var AllowedMergeStrategies = map[string]bool{
	"latest":  true, // Use the latest value from the profiles being merged - rely on the updated_at field
	"combine": true, // Combine values from both profiles (the value type has to be arrayOfString or arrayOfInt)
	"oldest":  true, // Use the oldest value from the profiles being merged - rely on the updated_at field
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

var AllowedEventTypes = map[string]bool{
	"page":     true,
	"track":    true,
	"identify": true,
}

var AllowedConsentPurposes = map[string]bool{
	"profiling":       true,
	"personalization": true,
	"destination":     true,
}

// Merge usecases
const (
	TempProfile_TempProfile_Merge = "TEMP_TEMP"
	TempProfile_PermProfile_Merge = "TEMP_PERM"
	PermProfile_PermProfile_Merge = "PERM_PERM"
)

// Sync Profile usecases
const (
	SyncProfileOnSchedule = "SYNC_ON_SCHEDULE"
	SyncProfileOnUpdate   = "SYNC_ON_UPDATE"
)

// Merge Strategies
const (
	MergeByAdmin   = "MERGE_BY_ADMIN"
	MergeByUser    = "MERGE_BY_USER"
	MergeOnTrigger = "MERGE_ON_TRIGGER"
)

// Profile States
const (
	ReferenceProfile = "REFERENCE_PROFILE"
	WaitOnAdmin      = "WAIT_ON_ADMIN"
	WaitOnUser       = "WAIT_ON_USER"
	MergedTo         = "MERGED_TO"
)
