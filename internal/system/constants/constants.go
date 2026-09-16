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

package constants

import "time"

import "regexp"

const ApiBasePath = "/cds/api"
const ProfileApiPath = "profiles"
const UnificationRulesApiPath = "unification-rules"
const ConsentApiPath = "consent"
const ProfileSchemaApiPath = "profile-schema"
const IdentityServerDialectsPath = "/api/server/v1/claim-dialects"
const Filter = "filter"
const FuzzyFilter = "fuzzyFilter"
const Attributes = "attributes"     // Query parameter to filter attributes in the request.
const ProfileCookie = "cds_profile" // Cookie name to store cookie that corresponds to profile ID.
const DefaultTenant = "carbon.super"
const SpaceSeparator = " "
const SystemAppHeader = "SystemApp"
const DefaultQueueSize = 1000
const DefaultLimit = 50
const CONSOLE_APP = "CONSOLE"
const AZPClaim = "azp"
const SUBClaim = "sub"
const ClientIdClaim = "client_id"
const ActiveClaim = "active"
const OrgHandleClaim = "org_handle"
const AudienceClaim = "aud"
const ExpiryClaim = "exp"
const FilterRegex = `^[a-zA-Z0-9._-]+$`
const MaxAttributeDisplayNameLength = 50
const GetProfilesPageSize = 500

var DisplayNameCamelCaseSplitter = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var DisplayNameRegex = regexp.MustCompile(`[^a-zA-Z0-9.\-_+ ]`)

type contextKey string

const TenantContextKey contextKey = "org_handle"
const PathPrefixContextKey contextKey = "path_prefix"

const (
	ProfileResource            = "profile"
	UnificationRuleResource    = "unification rule"
	SchemaAttribute            = "schema attribute"
	AdminConfigResource        = "admin config"
	UnificationOptionsResource = "unification options"
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
	AuthenticationSuccess     = "AUTHENTICATION_SUCCESS"
	SessionTermination        = "SESSION_TERMINATE"
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

const (
	DefaultIdentityDataCategoryName    = "Identity Data"
	DefaultIdentityDataCategoryPurpose = "profiling"
)

const (
	ScopeIdentityAttributes = "identityAttributes"
	ScopeTraits             = "traits"
	ScopeApplicationData    = "applicationData"
)

var AllowedConsentAttributeScopes = map[string]bool{
	ScopeIdentityAttributes: true,
	ScopeTraits:             true,
	ScopeApplicationData:    true,
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

	// Identity resolution config keys
	ConfigAutoMergeEnabled      = "auto_merge_enabled"
	ConfigAutoMergeThreshold    = "auto_merge_threshold"
	ConfigManualReviewThreshold = "manual_review_threshold"
)

const (
	AttributeTypePrimitiveExact = "PRIMITIVE_EXACT"
	AttributeTypeFuzzyString    = "FUZZY_STRING"
	AttributeTypeName           = "NAME"
	AttributeTypeEmail          = "EMAIL"
	AttributeTypePhone          = "PHONE"
	AttributeTypeLocation       = "LOCATION"
	AttributeTypeDate           = "DATE"
	AttributeTypeUniqueID       = "UNIQUE_ID"
)

const (
	AttributeTypeLabelPrimitiveExact = "Exact Match (Strictly identical)"
	AttributeTypeLabelFuzzyString    = "General Text (Tolerates typos)"
	AttributeTypeLabelName           = "Person / Company Name"
	AttributeTypeLabelEmail          = "Email Address"
	AttributeTypeLabelPhone          = "Phone Number"
	AttributeTypeLabelLocation       = "Location / Address"
	AttributeTypeLabelDate           = "Date (e.g. Date of Birth)"
	AttributeTypeLabelUniqueID       = "Unique Identifier"
)

var AllowedAttributeTypes = map[string]bool{
	AttributeTypePrimitiveExact: true,
	AttributeTypeFuzzyString:    true,
	AttributeTypeName:           true,
	AttributeTypeEmail:          true,
	AttributeTypePhone:          true,
	AttributeTypeLocation:       true,
	AttributeTypeDate:           true,
	AttributeTypeUniqueID:       true,
}

const (
	UnificationMethodFuzzy         = "fuzzy"
	UnificationMethodDeterministic = "deterministic"
)

const (
	UnificationMethodLabelDeterministic = "Deterministic (Exact)"
	UnificationMethodLabelFuzzyGeneral  = "Fuzzy (Typo tolerant)"
	UnificationMethodLabelFuzzyPhonetic = "Fuzzy (Phonetic / Typo tolerant)"
	UnificationMethodLabelFuzzyFormat   = "Fuzzy (Format tolerant)"
)

var AllowedUnificationMethods = map[string]bool{
	UnificationMethodFuzzy:         true,
	UnificationMethodDeterministic: true,
}

// FuzzyCapableAttributeTypes lists types where unification_method "fuzzy" has
// an effect. Types NOT in this set always perform exact matching regardless of
// the method field, so we reject "fuzzy" for them at the API level.
var FuzzyCapableAttributeTypes = map[string]bool{
	AttributeTypeFuzzyString: true,
	AttributeTypeName:        true,
	AttributeTypeEmail:       true,
	AttributeTypePhone:       true,
	AttributeTypeLocation:    true,
}

const (
	ReviewStatusPending   = "PENDING"
	ReviewStatusApproved  = "APPROVED"
	ReviewStatusRejected  = "REJECTED"
	ReviewStatusCancelled = "CANCELLED"
)

const (
	DecisionAutoMerge    = "AUTO_MERGE"
	DecisionManualReview = "MANUAL_REVIEW"
	DecisionUnique       = "UNIQUE"
)

const (
	ProfileTypePermanent = "PERMANENT"
	ProfileTypeTemp      = "TEMP"
)

const (
	UnificationModeStrict = "STRICT"
	UnificationModeSmart  = "SMART"
)

const (
	MergeReasonAutoMerge   = "auto_merge"
	MergeReasonReviewMerge = "review_merge"
	MergeReasonManualMerge = "manual_merge"
)

const (
	CanceledBySystem = "SYSTEM"
)

const (
	SystemUserIdMatchReason = "system:user_id_match"
)

const (
	// Maximum number of candidates per rule. If we allow any number of
	// candidates per rule, the issue is that rule attribute value is very common
	// and we can not merge profile with very comman data.
	MaxCandidatesPerRule = 100
)

const (
	DefaultAutoMergeThreshold    = 0.95
	DefaultManualReviewThreshold = 0.75
)

const (
	// LSHSignatureSize is the total number of MinHash signatures generated.
	LSHSignatureSize = 8

	// LSHBands and LSHRows split those 8 hashes into 4 chunks (bands) of 2 hashes (rows).
	// The Rule: Two strings only need to perfectly match ONE of these 4 chunks to be
	// flagged as a potential typo/duplicate.
	//
	// Why 4 and 2? This specific ratio creates a mathematical "trap door" at 50% similarity.
	// Strings that are >= 50% similar will almost certainly match at least one chunk.
	// Strings < 50% similar will fail, effectively filtering out junk matches.
	LSHBands = 4
	LSHRows  = 2

	// LSHMinLength is the minimum string length required to generate a hash.
	LSHMinLength = 4

	// MaxMetaphoneLen truncates phonetic codes to 4 characters (approx. 2 syllables).
	// Industry standard: Lawrence Philips' Double Metaphone algorithm shows
	// exceeding 4 characters increases false negatives due to trailing vowel differences.
	MaxMetaphoneLen = 4
)

// Scoring engine

const (
	// ScoreContradictionThreshold is the score at or below which two present values are
	// treated as actively disagreeing rather than merely scoring poorly. Above it and
	// below the org's manual-review threshold a rule is inconclusive: it neither supports
	// nor opposes the match, and is excluded from both the agreement count and the
	// contradiction count.
	// NOTE: there is no universal value for this. Tune per tenant against real data.
	ScoreContradictionThreshold = 0.3

	// MinAgreeingRulesForAutoMerge is how many rules must independently agree before a
	// match may auto-merge. The exception is the org's highest-priority rule: when the
	// operator's strongest configured signal is the one agreeing, it may carry an
	// auto-merge alone. Any other single agreement is capped to manual review, so a match
	// on one weak attribute (a shared city, a common given name) can never merge unattended.
	MinAgreeingRulesForAutoMerge = 2

	// ScorePenaltyOffset is subtracted from the auto-merge threshold when capping a
	// score just below it. The small gap keeps the score detectable as sub-threshold
	// while remaining high enough to route to manual review.
	ScorePenaltyOffset = 0.01
)

// Evidence strength

const (
	// EvidenceStrengthHigh on a match means the attribute identifies a person well enough
	// to merge on its own (a national ID, an email address). On a mismatch it means two
	// present, differing values are close to proof of difference.
	EvidenceStrengthHigh = "HIGH"
	// EvidenceStrengthMedium means the attribute is real evidence but needs corroboration.
	EvidenceStrengthMedium = "MEDIUM"
	// EvidenceStrengthLow means the attribute barely moves the decision on its own.
	EvidenceStrengthLow = "LOW"
)

// Labels and descriptions for the evidence-strength choices, worded per direction because
// the same value carries a different meaning on a match than on a mismatch.
const (
	MatchStrengthLabelHigh         = "Strong — can merge on its own"
	MatchStrengthLabelMedium       = "Moderate — needs a second match"
	MatchStrengthLabelLow          = "Weak — supporting evidence only"
	MatchStrengthDescriptionHigh   = "Two profiles sharing this value are almost certainly the same person, so a match here can merge them unattended."
	MatchStrengthDescriptionMedium = "A match is real evidence but not conclusive; another attribute must agree before profiles merge automatically."
	MatchStrengthDescriptionLow    = "Many unrelated people share this value, so a match on its own is close to coincidence."

	MismatchStrengthLabelHigh         = "Decisive — different values mean different people"
	MismatchStrengthLabelMedium       = "Moderate — counts against a match"
	MismatchStrengthLabelLow          = "Weak — people legitimately have several"
	MismatchStrengthDescriptionHigh   = "If both profiles hold this attribute and the values differ, they are treated as different people and never merged automatically."
	MismatchStrengthDescriptionMedium = "A difference counts against the match but does not block it on its own."
	MismatchStrengthDescriptionLow    = "One person routinely has several of these, so differing values say little."
)

var AllowedEvidenceStrengths = map[string]bool{
	EvidenceStrengthHigh:   true,
	EvidenceStrengthMedium: true,
	EvidenceStrengthLow:    true,
}

// DefaultMatchStrength and DefaultMismatchStrength seed a rule's evidence weights from its
// attribute type when the operator does not set them explicitly.
//
// The two are deliberately independent, because agreement and disagreement on the same
// attribute rarely carry equal weight. Two people sharing an email address is strong
// evidence they are the same person, but holding different email addresses says almost
// nothing — most people have several. A shared date of birth is weak evidence of sameness,
// since birthdays collide constantly, yet two different dates of birth are close to proof
// of difference. Folding both directions into one number is what makes a mismatching
// attribute dilute a match instead of contradicting it.
var DefaultMatchStrength = map[string]string{
	AttributeTypeUniqueID: EvidenceStrengthHigh,
	AttributeTypeEmail:    EvidenceStrengthHigh,
	AttributeTypePhone:    EvidenceStrengthHigh,
	// PRIMITIVE_EXACT is what every rule written before typed matching resolves to, and
	// back then any rule matching exactly merged the two profiles outright. Treating it as
	// strong keeps that true on upgrade. An operator who wants a weaker reading gives the
	// attribute its real type instead.
	AttributeTypePrimitiveExact: EvidenceStrengthHigh,
	AttributeTypeFuzzyString:    EvidenceStrengthMedium,
	AttributeTypeName:           EvidenceStrengthLow,
	AttributeTypeLocation:       EvidenceStrengthLow,
	AttributeTypeDate:           EvidenceStrengthLow,
}

var DefaultMismatchStrength = map[string]string{
	AttributeTypeUniqueID:       EvidenceStrengthHigh,
	AttributeTypeDate:           EvidenceStrengthHigh,
	AttributeTypeName:           EvidenceStrengthMedium,
	AttributeTypePrimitiveExact: EvidenceStrengthMedium,
	AttributeTypeEmail:          EvidenceStrengthLow,
	AttributeTypePhone:          EvidenceStrengthLow,
	AttributeTypeLocation:       EvidenceStrengthLow,
	AttributeTypeFuzzyString:    EvidenceStrengthLow,
}

// Value rarity

const (
	// RarityCommonMinProfiles is how many profiles in the org must already share a value
	// before a match on it stops counting as identifying. A shared corporate address,
	// a support mailbox or a placeholder that slipped past the uninformative-value check
	// will each land on many profiles; agreement on such a value is coincidence, not
	// evidence, so it is not allowed to carry an auto-merge on its own.
	//
	// Rarity may only ever weaken evidence, never strengthen it: an uncommon value being
	// wrong is just as wrong as a common one, so rarity lowers the chance of coincidence
	// without adding corroboration.
	// NOTE: an absolute count rather than a share of the tenant, so it costs one indexed
	// lookup and no second query. Tune per tenant.
	RarityCommonMinProfiles = 50

	// RarityLookupCacheTTL bounds how long a value's frequency is reused. Frequencies
	// move slowly, and the count is only used to pick a band.
	RarityLookupCacheTTL = 10 * time.Minute
)

// Name matching

const (
	// NamePhoneticExactJWMin acts as a score floor (safety net) when two names have identical
	// primary phonetic codes. If the phonetic match is perfect (1.0), the final score is
	// guaranteed to be at least this value. If the raw JW score is lower, it is bumped up
	// to this minimum. If the raw JW score is higher, the JW score is kept.
	// NOTE: There is no mathematically perfect universal constant for this value. Must be tuned per-tenant.
	NamePhoneticExactJWMin = 0.9

	// PhoneticAlternateScore is returned by PhoneticSimilarity when two names share a
	// Double Metaphone alternate code but not the primary code. Lower than 1.0 to reflect
	// the reduced certainty of an alternate encoding.
	// NOTE: The Double Metaphone algorithm calculates both a primary and an alternate
	// pronunciation. This baseline dictates the score awarded when names only match
	// on their alternate encoding.
	PhoneticAlternateScore = 0.9
)

// Phone matching

const (
	// PhoneSuffixBlockingLength dictates how many trailing digits must perfectly align
	// to be considered a valid "Suffix Match."
	// NOTE: This is based on the standard Sri Lankan phone number format.
	// This must be tuned based on a tenant's regional data.
	PhoneSuffixBlockingLength = 7

	// NOTE: There is no mathematically perfect constant for a partial phone match.
	// This baseline dictates the score awarded when only the suffix matches,
	// but the area/country codes actively differ or are completely missing.
	// PhoneSuffixMatchScore is returned when two phone numbers share the same last
	// PhoneSuffixBlockingLength digits but differ elsewhere (e.g., different country
	// codes). A suffix-only match is strong evidence but not conclusive.
	PhoneSuffixMatchScore = 0.9
)

// Jaro-Winkler algorithm

const (
	// JaroWinklerMaxPrefix is set to 4 based on William E. Winkler's original
	// US Census Bureau paper (1990). Empirical studies showed prefix matches
	// beyond 4 characters yield diminishing returns for duplicate detection.
	JaroWinklerMaxPrefix = 4

	// JaroWinklerPFactor is the standard scaling constant (0.1) defined by Winkler.
	JaroWinklerPFactor = 0.1
)

const (
	DefaultCookieCleanupTime = 24 * 60 * 60 // 24 hours in seconds
)
