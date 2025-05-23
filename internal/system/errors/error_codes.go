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

package errors

const errorPrefix = "CDS-"

var (
	// Server error codes

	ErrWhileCreatingUnificationRules = ErrorMessage{
		Code:        errorPrefix + "15001",
		Message:     "Error while adding unification rules.",
		Description: "Error while adding unification rules for the organization.",
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

	ADD_ENRICHMENT_RULE = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while adding enrichment rules.",
	}

	ADD_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while adding event stream id.",
	}

	GET_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while fetching event stream id.",
	}

	UPDATE_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while fetching event stream id.",
	}

	UPDATE_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15008",
		Message: "Error while updating enrichment rules.",
	}

	FETCH_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15008",
		Message: "Error while fetching enrichment rule(s).",
	}

	DELETE_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15008",
		Message: "Error while deleting enrichment rule.",
	}

	FILTER_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15008",
		Message: "Error while filtering enrichment rules.",
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

	ADD_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while adding unification rules.",
	}

	GET_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while fetching unification rules.",
	}

	UPDATE_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while updating unification rules.",
	}

	LOCK_ACQUIRE = ErrorMessage{
		Code:    errorPrefix + "15016",
		Message: "Advisory lock acquisition failed",
	}

	DB_CLIENT_INIT = ErrorMessage{
		Code:    errorPrefix + "15017",
		Message: "Unable to initialize database client.",
	}

	DB_TRANSACTION_INIT = ErrorMessage{
		Code:    errorPrefix + "15017",
		Message: "Failed to begin transaction",
	}

	LOCK_RELEASE = ErrorMessage{
		Code:    errorPrefix + "15018",
		Message: "Error while releasing the lock.",
	}

	LOCK_KEY_GEN = ErrorMessage{
		Code:    errorPrefix + "15018",
		Message: "Error generating advisory lock key",
	}

	LOCK_RESULT_INVALID = ErrorMessage{
		Code:    errorPrefix + "15018",
		Message: "Invalid response from advisory lock query.",
	}

	MARSHAL_JSON = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Error while marshalling JSON.",
	}
	UNMARSHAL_JSON = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Error while un-marshalling JSON.",
	}

	INVALID_TYPE = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Invalid type.",
	}

	ADD_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Adding consent category failed.",
	}

	FETCH_CONSENT_CATEGORIES = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Fetching consent category failed.",
	}

	UPDATE_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Updating consent category failed.",
	}

	// Client error codes

	BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "11001",
		Message: "Invalid body.",
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

	ErrPropertyDoesnotExists = ErrorMessage{
		Code:    errorPrefix + "11011",
		Message: "Property does not exist.",
	}

	ErrPropertyAlreadyExists = ErrorMessage{
		Code:    errorPrefix + "11011",
		Message: "Property already exist.",
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
		Code:    errorPrefix + "10014",
		Message: "Invalid computation method.",
	}

	ErrPropertyNameValidation = ErrorMessage{
		Code:        errorPrefix + "10015",
		Message:     "Missing computation logic.",
		Description: "For computed traits, 'computation' must be provided.",
	}

	ENRICHMENT_RULE_VALIDATION = ErrorMessage{
		Code:    errorPrefix + "10015",
		Message: "Enrichment rule validation failed",
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

	ErrPropDoesntExists = ErrorMessage{
		Code:    errorPrefix + "11020",
		Message: "Property does not exist.",
	}

	ErrInvalidTime = ErrorMessage{
		Code:    errorPrefix + "11021",
		Message: "Validation failed.",
	}

	INVALID_ENRICHMENT_RULE_FILTERING = ErrorMessage{
		Code:    errorPrefix + "11022",
		Message: "Enrichment rules filtering failed.",
	}

	ADD_EVENT = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Error while adding event.",
	}

	GET_EVENT = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Error while fetching event.",
	}

	DELETE_EVENT = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Error while deleting event.",
	}

	EVENT_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Event not found.",
	}

	ADD_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Profile addition failed.",
	}

	GET_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Fetching profile(s) failed.",
	}

	FILTER_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Filtering profiles failed.",
	}

	UPDATE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Profile update failed.",
	}

	DELETE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Profile deletion failed.",
	}

	ADD_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Add app data failed.",
	}

	GET_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Fetching app data failed.",
	}

	UPDATE_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Updating app data failed.",
	}

	UPDATE_IDENTITY_ATT = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Updating identity attribute failed.",
	}

	UPDATE_TRAIT = ErrorMessage{
		Code:    errorPrefix + "11023",
		Message: "Updating trait failed.",
	}

	ErrMultipleProfileFound = ErrorMessage{
		Code:        errorPrefix + "11023",
		Message:     "Multiple Profiles found.",
		Description: "Multiple user profiles record found for the given user_id",
	}

	CONSENT_CAT_VALIDATION = ErrorMessage{
		Code:    errorPrefix + "10015",
		Message: "Consent category validation failed",
	}

	CONSENT_CAT_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "10015",
		Message: "Consent category already exist.",
	}

	CONSENT_CAT_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "10015",
		Message: "Consent category not found.",
	}
)
