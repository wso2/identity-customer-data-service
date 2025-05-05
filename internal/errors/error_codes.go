package errors

const errorPrefix = "CDS-"

var (
	// Server error codes

	ErrWhileCreatingUnificationRules = ErrorMessage{
		Code:        errorPrefix + "15001",
		Message:     "Error while adding unification rules.",
		Description: "Error while adding unification rules for the organization.",
	}

	ErrWhileRevokingToken = ErrorMessage{
		Code:        errorPrefix + "15002",
		Message:     "Token revocation failed.",
		Description: "Error while revoking the token",
	}

	ErrWhileIssuingNewToken = ErrorMessage{
		Code:        errorPrefix + "15003",
		Message:     "Token issuance failed.",
		Description: "Error while issuing new token",
	}

	ErrWhileIntrospectingNewToken = ErrorMessage{
		Code:        errorPrefix + "15004",
		Message:     "Token introspection failed.",
		Description: "Error while introspecting the token",
	}

	ErrWhileFetchingProfileEnrichmentRules = ErrorMessage{
		Code:        errorPrefix + "15005",
		Message:     "Error while fetching profile enrichment rules.",
		Description: "Error while fetching profile enrichment rules.",
	}

	ErrWhileFetchingProfile = ErrorMessage{
		Code:        errorPrefix + "15006",
		Message:     "Error while fetching profile.",
		Description: "Server error occurred while trying to fetch profile by profileId.",
	}

	ErrWhileDeletingProfile = ErrorMessage{
		Code:        errorPrefix + "15007",
		Message:     "Error while deleting profile.",
		Description: "Server error occurred while trying to fetch profile by profileId.",
	}

	ErrWhileFetchingEnrichmentRules = ErrorMessage{
		Code:        errorPrefix + "15008",
		Message:     "Error while fetching enrichment rules.",
		Description: "Error while fetching enrichment rules of the organization.",
	}

	ErrWhileAddingUnificationRules = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while adding unification rules.",
	}

	ErrWhileFetchingUnificationRule = ErrorMessage{
		Code:        errorPrefix + "15010",
		Message:     "Error while fetching unification rule.",
		Description: "Error while fetching unification rule of the organization.",
	}

	ErrWhileFetchingUnificationRules = ErrorMessage{
		Code:        errorPrefix + "15011",
		Message:     "Error while fetching unification rule.",
		Description: "Error while fetching unification rule of the organization.",
	}

	ErrWhileUpdatingUnificationRule = ErrorMessage{
		Code:        errorPrefix + "15012",
		Message:     "Error while updating unification rule.",
		Description: "Error while updating unification rule of the organization.",
	}

	ErrWhileAddingEvent = ErrorMessage{
		Code:        errorPrefix + "15013",
		Message:     "Error while adding event.",
		Description: "Error while adding event.",
	}

	ErrWhileBuildingPath = ErrorMessage{
		Code:    errorPrefix + "15014",
		Message: "Error while building URL.",
	}

	ErrWhileGeneratingWriteKey = ErrorMessage{
		Code:    errorPrefix + "15015",
		Message: "Error while generating the write key.",
	}

	// Client error codes

	ErrBadRequest = ErrorMessage{
		Code:    errorPrefix + "11001",
		Message: "Invalid body format.",
	}

	ErrUnAuthorizedRequest = ErrorMessage{
		Code:        errorPrefix + "11002",
		Message:     "Unauthorized",
		Description: "Authorization failure. Authorization information was invalid or missing from your request.",
	}

	ErrUnAuthorizedExpiryRequest = ErrorMessage{
		Code:        errorPrefix + "11003",
		Message:     "Unauthorized",
		Description: "Token has expired ",
	}

	ErrInvalidAudience = ErrorMessage{
		Code:        errorPrefix + "11004",
		Message:     "Unauthorized",
		Description: "Invalid audience",
	}

	ErrResolutionRuleNotFound = ErrorMessage{
		Code:        errorPrefix + "11004",
		Message:     "No resolution rule found.",
		Description: "No resolution rule defined for this organization for the provided rule_id..",
	}

	ErrOnlyStatusUpdatePossible = ErrorMessage{
		Code:    errorPrefix + "11005",
		Message: "Rule Name, Active Status or Property can only be updated.",
	}

	ErrNoEventProps = ErrorMessage{
		Code:        errorPrefix + "11006",
		Message:     "No event properties.",
		Description: "At least one event property should be added to the event schema.",
	}

	ErrNoEventPropValue = ErrorMessage{
		Code:        errorPrefix + "11007",
		Message:     "No event properties.",
		Description: "Property %s must have both name and type",
	}

	ErrImproperProperty = ErrorMessage{
		Code:        errorPrefix + "11008",
		Message:     "Improper property name or type.",
		Description: "Allowed types are: string, int, boolean, timestamp, array",
	}

	ErrValidationProfileTrait = ErrorMessage{
		Code:        errorPrefix + "11009",
		Message:     "Invalid value for the profile trait.",
		Description: "Allowed types are: string, int, boolean, timestamp, array",
	}

	ErrProfileNotFound = ErrorMessage{
		Code:        errorPrefix + "11010",
		Message:     "Profile not found.",
		Description: "No user profile record found for the given profile_id",
	}

	ErrResolutionRuleAlreadyExists = ErrorMessage{
		Code:        errorPrefix + "11011",
		Message:     "Rule already exist.",
		Description: "No user profile record found for the given profile_id",
	}

	ErrPropertyTypeValidation = ErrorMessage{
		Code:        errorPrefix + "10012",
		Message:     "Invalid rule type.",
		Description: "Trait type must be either 'static' or 'computed'",
	}

	ErrEnrichmentRuleValueValidation = ErrorMessage{
		Code:        errorPrefix + "10013",
		Message:     "Missing static value.",
		Description: "For static traits, 'value' must be provided.",
	}

	ErrComputationValidation = ErrorMessage{
		Code:        errorPrefix + "10014",
		Message:     "Missing computation logic.",
		Description: "For computed traits, 'computation' must be provided.",
	}

	ErrPropertyNameValidation = ErrorMessage{
		Code:        errorPrefix + "10015",
		Message:     "Missing computation logic.",
		Description: "For computed traits, 'computation' must be provided.",
	}

	ErrSourceFieldValidation = ErrorMessage{
		Code:        errorPrefix + "10015",
		Message:     "Missing source field",
		Description: "For copy computation, 'source field' must be provided.",
	}

	ErrTriggerValidation = ErrorMessage{
		Code:    errorPrefix + "10016",
		Message: "Invalid trigger definition.",
	}

	ErrConditionOpValidation = ErrorMessage{
		Code:    errorPrefix + "10017",
		Message: "Unsupported operator.",
	}

	ErrMergeStratValidation = ErrorMessage{
		Code:    errorPrefix + "10018",
		Message: "Invalid merge strategy.",
	}

	ErrMaskingStratValidation = ErrorMessage{
		Code:    errorPrefix + "10019",
		Message: "Missing masking strategy.",
	}

	ErrPropDoesntExists = ErrorMessage{
		Code:    errorPrefix + "11020",
		Message: "Property does not exist.",
	}
)
