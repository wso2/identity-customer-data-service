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

	DELETE_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15011",
		Message: "Error while deleting unification rule(s).",
	}

	INVALID_TYPE = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Invalid type.",
	}

	ADD_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Adding consent category failed.",
	}

	ADD_CONSENT_CATEGORY_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Invalid request payload for adding consent category.",
	}

	UPDATE_CONSENT_CATEGORY_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "15019",
		Message: "Invalid request payload for updating consent category.",
	}

	FETCH_CONSENT_CATEGORIES = ErrorMessage{
		Code:    errorPrefix + "15020",
		Message: "Fetching consent category failed.",
	}

	UPDATE_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15021",
		Message: "Updating consent category failed.",
	}

	INTROSPECTION_FAILED = ErrorMessage{
		Code:    errorPrefix + "15022",
		Message: "Introspection failed.",
	}

	PARSING_ERROR = ErrorMessage{
		Code:    errorPrefix + "15023",
		Message: "Parsing token failed.",
	}

	ADD_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15024",
		Message: "Error while adding profile schema.",
	}

	GET_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15025",
		Message: "Error while fetching profile schema.",
	}

	UPDATE_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15025",
		Message: "Error while updating profile schema.",
	}

	DELETE_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15025",
		Message: "Error while deleting profile schema attribute(s).",
	}

	SYNC_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15025",
		Message: "Error while syncing identity attributes of profile schema.",
	}

	GET_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15027",
		Message: "Error while fetching config.",
	}

	UPDATE_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15028",
		Message: "Error while updating config.",
	}

	TOKEN_FETCH_FAILED = ErrorMessage{
		Code:    errorPrefix + "15029",
		Message: "Fetching token failed.",
	}

	GET_SCIM_DIALECTS = ErrorMessage{
		Code:    errorPrefix + "15030",
		Message: "Error while fetching SCIM dialects.",
	}

	GET_DIALECT_CLAIMS = ErrorMessage{
		Code:    errorPrefix + "15031",
		Message: "Error while fetching dialect claims.",
	}

	GET_LOCAL_CLAIMS_FAILED = ErrorMessage{
		Code:    errorPrefix + "15032",
		Message: "Error while fetching local claims.",
	}

	GET_SCIM_USER_FAILED = ErrorMessage{
		Code:    errorPrefix + "15033",
		Message: "Error while fetching SCIM user.",
	}

	GET_ADMIN_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15034",
		Message: "Error while fetching tenant configurations.",
	}
	UPDATE_ADMIN_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15035",
		Message: "Error while updating tenant configurations.",
	}

	GET_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching profile cookie failed.",
	}

	UPDATE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Updating profile cookie failed.",
	}

	DELETE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Updating profile cookie failed.",
	}

	// Client error codes
	BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "11001",
		Message: "Invalid body.",
	}

	UN_AUTHORIZED = ErrorMessage{
		Code:        errorPrefix + "11002",
		Message:     "Unauthorized",
		Description: "Authorization failure. Authorization information was invalid or missing from your request.",
	}

	PROFILE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "11005",
		Message:     "Profile not found.",
		Description: "No user profile record found for the given profile_id",
	}

	MULTIPLE_PROFILES_FOUND = ErrorMessage{
		Code:    errorPrefix + "11005",
		Message: "Multiple profiles found.",
	}

	ADD_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11013",
		Message: "Profile addition failed.",
	}

	GET_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching profile(s) failed.",
	}

	GET_PROFILE_CONSENT = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching profile consent failed.",
	}

	CREATE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Creating profile cookie failed.",
	}
	GET_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching profile cookie failed.",
	}

	UPDATE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Updating profile cookie failed.",
	}

	PROFILE_COOKIE_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Profile cookie not found.",
	}

	DELETE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Deleting profile cookie failed.",
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

	CONSENT_CAT_ID = ErrorMessage{
		Code:    errorPrefix + "10015",
		Message: "Consent category Id is required.",
	}

	FORBIDDEN = ErrorMessage{
		Code:        errorPrefix + "11025",
		Message:     "ForBidden",
		Description: "You do not have permission to access this resource.",
	}

	INVALID_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11030",
		Message: "Invalid profile body.",
	}

	GET_CONFIGURATION = ErrorMessage{
		Code:    errorPrefix + "11031",
		Message: "Error while fetching config.",
	}

	UPDATE_CONFIGURATION = ErrorMessage{
		Code:    errorPrefix + "11032",
		Message: "Error while updating config.",
	}

	INVALID_OPERATION = ErrorMessage{
		Code:    errorPrefix + "1133",
		Message: "Invalid Operation.",
	}

	PROFILE_SCHEMA_ADD_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "12001",
		Message: "Invalid request payload.",
	}

	INVALID_ATTRIBUTE_NAME = ErrorMessage{
		Code:    errorPrefix + "12002",
		Message: "Invalid attribute name.",
	}

	SCHEMA_ATTRIBUTE_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "12003",
		Message: "Schema attribute already exists.",
	}

	PROFILE_SCHEMA_UPDATE_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "12004",
		Message: "Invalid schema update.",
	}

	ATTRIBUTE_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "12005",
		Message: "Attribute not found.",
	}

	UNIFICATION_RULE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "12201",
		Message:     "No unification rule found.",
		Description: "No unification rule defined for this organization for the provided rule_id..",
	}

	UNIFICATION_UPDATE_FAILED = ErrorMessage{
		Code:    errorPrefix + "12202",
		Message: "Validation failed for unification rule update.",
	}

	UNIFICATION_RULE_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "12203",
		Message: "Unification already exist.",
	}
	UNIFICATION_RULE_PRIORITY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "12204",
		Message: "Unification priority already taken.",
	}

	INVALID_FILTER_FORMAT = ErrorMessage{
		Code:    errorPrefix + "13001",
		Message: "Invalid filter format.",
	}

	FETCH_TOKEN_FAILED = ErrorMessage{
		Code:    errorPrefix + "15029",
		Message: "Fetching token failed.",
	}

	UPDATE_CONFIG_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "13001",
		Message: "Invalid request payload for updating admin configuration.",
	}
)
