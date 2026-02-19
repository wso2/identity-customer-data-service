package constants

const ApiBasePath = "/cds/api"
const ProfileApiPath = "profiles"
const UnificationRulesApiPath = "unification-rules"
const ConsentApiPath = "consent"
const ProfileSchemaApiPath = "profile-schema"
const IdentityServerDialectsPath = "/api/server/v1/claim-dialects"
const Filter = "filter"
const Attributes = "attributes"     // Query parameter to filter attributes in the request.
const ProfileCookie = "cds_profile" // Cookie name to store cookie that corresponds to profile ID.
const DefaultTenant = "carbon.super"
const SpaceSeparator = " "
const SystemAppHeader = "SystemApp"
const DefaultQueueSize = 1000
const DefaultLimit = 50
const CONSOLE_APP = "CONSOLE"
const AZPClaim = "azp"
const ClientIdClaim = "client_id"
const ActiveClaim = "active"
const OrgHandleClaim = "org_handle"
const AudienceClaim = "aud"
const ExpiryClaim = "exp"
const FilterRegex = `^[a-zA-Z0-9._-]+$`

type contextKey string

const TenantContextKey contextKey = "org_handle"

const (
	ProfileResource         = "profile"
	UnificationRuleResource = "unification rule"
	SchemaAttribute         = "schema attribute"
	AdminConfigResource     = "admin config"
)

const (
	StringDataType   = "string"
	IntegerDataType  = "integer"
	DecimalDataType  = "decimal"
	BooleanDataType  = "boolean"
	DateTimeDataType = "date_time"
	DateDataType     = "date"
	ComplexDataType  = "complex"
	EpochDataType    = "epoch"
)

const (
	AddScimAttributeEvent     = "POST_ADD_EXTERNAL_CLAIM"
	UpdateScimAttributeEvent  = "POST_UPDATE_EXTERNAL_CLAIM"
	DeleteScimAttributeEvent  = "POST_DELETE_EXTERNAL_CLAIM"
	UpdateLocalAttributeEvent = "POST_UPDATE_LOCAL_CLAIM"
	DeleteLocalClaimEvent     = "POST_DELETE_LOCAL_CLAIM"
	AddUserEvent              = "POST_ADD_USER"
	DeleteUserEvent           = "POST_DELETE_USER_WITH_ID"
	UpdateUserClaimEvent      = "POST_SET_USER_CLAIM_VALUE_WITH_ID"
	UpdateUserClaimsEvent     = "POST_SET_USER_CLAIM_VALUES_WITH_ID"
)

var AllowedValueTypes = map[string]bool{
	StringDataType:   true,
	IntegerDataType:  true,
	DecimalDataType:  true,
	BooleanDataType:  true,
	DateTimeDataType: true,
	ComplexDataType:  true,
	EpochDataType:    true,
	DateDataType:     true,
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
	"combine":   true, // Combine values from both profiles (the value type has to be arrayOfString or arrayOfInt)
	"overwrite": true, // todo: Remove later.
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

var AllowedFilterFieldsForSchema = map[string]bool{
	"attribute_name":         true,
	"application_identifier": true,
}

const (
	ConfigCDSEnabled            = "cds_enabled"
	ConfigInitialSchemaSyncDone = "initial_schema_sync_done"
	ConfigSystemApplications    = "system_applications"
)
