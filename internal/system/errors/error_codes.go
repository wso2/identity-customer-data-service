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

	ADD_ENRICHMENT_RULE = ErrorMessage{
		Code:    errorPrefix + "15001",
		Message: "Error while adding enrichment rules.",
	}

	ADD_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15002",
		Message: "Error while adding event stream id.",
	}

	GET_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15003",
		Message: "Error while fetching event stream id.",
	}

	UPDATE_EVENT_STREAM_ID = ErrorMessage{
		Code:    errorPrefix + "15004",
		Message: "Error while fetching event stream id.",
	}

	UPDATE_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15005",
		Message: "Error while updating enrichment rules.",
	}

	FETCH_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15006",
		Message: "Error while fetching enrichment rule(s).",
	}

	DELETE_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15007",
		Message: "Error while deleting enrichment rule.",
	}

	FILTER_ENRICHMENT_RULES = ErrorMessage{
		Code:    errorPrefix + "15008",
		Message: "Error while filtering enrichment rules.",
	}

	ADD_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15009",
		Message: "Error while adding unification rules.",
	}

	GET_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15010",
		Message: "Error while fetching unification rules.",
	}

	UPDATE_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15011",
		Message: "Error while updating unification rules.",
	}

	LOCK_ACQUIRE = ErrorMessage{
		Code:    errorPrefix + "15012",
		Message: "Advisory lock acquisition failed",
	}

	DB_CLIENT_INIT = ErrorMessage{
		Code:    errorPrefix + "15013",
		Message: "Unable to initialize database client.",
	}

	LOCK_RELEASE = ErrorMessage{
		Code:    errorPrefix + "15014",
		Message: "Error while releasing the lock.",
	}

	LOCK_KEY_GEN = ErrorMessage{
		Code:    errorPrefix + "15015",
		Message: "Error generating advisory lock key",
	}

	LOCK_RESULT_INVALID = ErrorMessage{
		Code:    errorPrefix + "15016",
		Message: "Invalid response from advisory lock query.",
	}

	MARSHAL_JSON = ErrorMessage{
		Code:    errorPrefix + "15017",
		Message: "Error while marshalling JSON.",
	}
	UNMARSHAL_JSON = ErrorMessage{
		Code:    errorPrefix + "15018",
		Message: "Error while un-marshalling JSON.",
	}

	INVALID_TYPE = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Invalid type.",
	}

	INTROSPECTION_FAILED = ErrorMessage{
		Code:    errorPrefix + "15020",
		Message: "Introspection failed.",
	}

	PARSING_ERROR = ErrorMessage{
		Code:    errorPrefix + "15021",
		Message: "Parsing token failed.",
	}

	// Client error codes
	BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "11001",
		Message: "Invalid body format.",
	}

	UN_AUTHORIZED = ErrorMessage{
		Code:        errorPrefix + "11002",
		Message:     "Unauthorized",
		Description: "Authorization failure. Authorization information was invalid or missing from your request.",
	}

	UNIFICATION_RULE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "11003",
		Message:     "No resolution rule found.",
		Description: "No resolution rule defined for this organization for the provided rule_id..",
	}

	UNIFICATION_UPDATE_FAILED = ErrorMessage{
		Code:    errorPrefix + "11004",
		Message: "Validation failed for unification rule update.",
	}

	PROFILE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "11005",
		Message:     "Profile not found.",
		Description: "No user profile record found for the given profile_id",
	}

	PROP_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "11006",
		Message: "Property already exist.",
	}

	ENRICHMENT_RULE_VALIDATION = ErrorMessage{
		Code:    errorPrefix + "10007",
		Message: "Enrichment rule validation failed",
	}

	INVALID_ENRICHMENT_RULE_FILTERING = ErrorMessage{
		Code:    errorPrefix + "11008",
		Message: "Enrichment rules filtering failed.",
	}

	ADD_EVENT = ErrorMessage{
		Code:    errorPrefix + "11009",
		Message: "Error while adding event.",
	}

	GET_EVENT = ErrorMessage{
		Code:    errorPrefix + "11010",
		Message: "Error while fetching event.",
	}

	DELETE_EVENT = ErrorMessage{
		Code:    errorPrefix + "11011",
		Message: "Error while deleting event.",
	}

	EVENT_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "11012",
		Message: "Event not found.",
	}

	ADD_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11013",
		Message: "Profile addition failed.",
	}

	GET_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching profile(s) failed.",
	}

	FILTER_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11015",
		Message: "Filtering profiles failed.",
	}

	UPDATE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11016",
		Message: "Profile update failed.",
	}

	DELETE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11017",
		Message: "Profile deletion failed.",
	}

	ADD_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11018",
		Message: "Add app data failed.",
	}

	GET_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11019",
		Message: "Fetching app data failed.",
	}

	UPDATE_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11020",
		Message: "Updating app data failed.",
	}

	UPDATE_IDENTITY_ATT = ErrorMessage{
		Code:    errorPrefix + "11021",
		Message: "Updating identity attribute failed.",
	}

	UPDATE_TRAIT = ErrorMessage{
		Code:    errorPrefix + "11022",
		Message: "Updating trait failed.",
	}

	MULTIPLE_PROFILE_FOUND = ErrorMessage{
		Code:        errorPrefix + "11023",
		Message:     "Multiple Profiles found.",
		Description: "Multiple user profiles record found for the given user_id",
	}

	INVALID_EVENT = ErrorMessage{
		Code:    errorPrefix + "11024",
		Message: "Invalid event.",
	}

	FORBIDDEN = ErrorMessage{
		Code:        errorPrefix + "11025",
		Message:     "ForBidden",
		Description: "You do not have permission to access this resource.",
	}
)
