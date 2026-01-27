/*
 * Copyright (c) 2025-2026, WSO2 LLC. (http://www.wso2.com).
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

	// Server (15xxx) sub-grouping
	//   150xx - Admin Config & Server Operations
	//   151xx - Profile Schema & Schema Sync / IS Integration
	//   152xx - Unification Rules Management
	//   153xx - Consent Management
	//   154xx - Profiles & Cookie Management
	//   159xx - Other Server Errors

	GET_ADMIN_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15001",
		Message: "Error while fetching tenant configurations.",
	}
	UPDATE_ADMIN_CONFIG = ErrorMessage{
		Code:    errorPrefix + "15002",
		Message: "Error while updating tenant configurations.",
	}

	ADD_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15101",
		Message: "Error while adding profile schema.",
	}

	GET_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15102",
		Message: "Error while fetching profile schema.",
	}

	UPDATE_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15103",
		Message: "Error while updating profile schema.",
	}

	DELETE_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15104",
		Message: "Error while deleting profile schema attribute(s).",
	}

	SYNC_PROFILE_SCHEMA = ErrorMessage{
		Code:    errorPrefix + "15105",
		Message: "Error while syncing identity attributes of profile schema.",
	}

	TOKEN_FETCH_FAILED = ErrorMessage{
		Code:    errorPrefix + "15106",
		Message: "Fetching token failed.",
	}

	GET_SCIM_DIALECTS = ErrorMessage{
		Code:    errorPrefix + "15107",
		Message: "Error while fetching SCIM dialects.",
	}

	GET_DIALECT_CLAIMS = ErrorMessage{
		Code:    errorPrefix + "15108",
		Message: "Error while fetching dialect claims.",
	}

	GET_LOCAL_CLAIMS_FAILED = ErrorMessage{
		Code:    errorPrefix + "15109",
		Message: "Error while fetching local claims.",
	}

	GET_SCIM_USER_FAILED = ErrorMessage{
		Code:    errorPrefix + "15110",
		Message: "Error while fetching SCIM user.",
	}

	ADD_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15201",
		Message: "Error while adding unification rules.",
	}

	GET_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15202",
		Message: "Error while fetching unification rules.",
	}

	UPDATE_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15203",
		Message: "Error while updating unification rules.",
	}

	DELETE_UNIFICATION_RULE = ErrorMessage{
		Code:    errorPrefix + "15204",
		Message: "Error while deleting unification rule(s).",
	}

	ADD_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15301",
		Message: "Adding consent category failed.",
	}

	ADD_CONSENT_CATEGORY_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "15302",
		Message: "Invalid request payload for adding consent category.",
	}

	FETCH_CONSENT_CATEGORIES = ErrorMessage{
		Code:    errorPrefix + "15303",
		Message: "Fetching consent category failed.",
	}

	UPDATE_CONSENT_CATEGORY = ErrorMessage{
		Code:    errorPrefix + "15304",
		Message: "Updating consent category failed.",
	}

	GET_COOKIE = ErrorMessage{
		Code:    errorPrefix + "15401",
		Message: "Fetching profile cookie failed.",
	}

	UPDATE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "15402",
		Message: "Updating profile cookie failed.",
	}

	DELETE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "15403",
		Message: "Updating profile cookie failed.",
	}

	PARSING_ERROR = ErrorMessage{
		Code:    errorPrefix + "15901",
		Message: "Parsing token failed.",
	}

	//   100xx - Auth / Access
	//   110xx - Profile & App Data & cookies
	//   120xx - Profile Schema
	//   130xx - Unification Rules
	//   140xx - Consent Management
	//   160xx - Admin Configurations
	//   190xx - Other Client Errors
	BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "10001",
		Message: "Invalid body.",
	}

	UN_AUTHORIZED = ErrorMessage{
		Code:        errorPrefix + "10002",
		Message:     "Unauthorized",
		Description: "Authorization failure. Authorization information was invalid or missing from your request.",
	}

	FORBIDDEN = ErrorMessage{
		Code:        errorPrefix + "10003",
		Message:     "Forbidden",
		Description: "You do not have enough permission to access this resource.",
	}

	PROFILE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "11001",
		Message:     "Profile not found.",
		Description: "No user profile record found for the given profile_id",
	}

	ADD_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11002",
		Message: "Profile addition failed.",
	}

	GET_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11003",
		Message: "Fetching profile(s) failed.",
	}

	GET_PROFILE_CONSENT = ErrorMessage{
		Code:    errorPrefix + "11004",
		Message: "Fetching profile consent failed.",
	}

	CREATE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11005",
		Message: "Creating profile cookie failed.",
	}
	GET_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11006",
		Message: "Fetching profile cookie failed.",
	}

	UPDATE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11007",
		Message: "Updating profile cookie failed.",
	}

	PROFILE_COOKIE_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "11008",
		Message: "Profile cookie not found.",
	}

	DELETE_PROFILE_COOKIE = ErrorMessage{
		Code:    errorPrefix + "11009",
		Message: "Deleting profile cookie failed.",
	}

	FILTER_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11010",
		Message: "Filtering profiles failed.",
	}

	UPDATE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11011",
		Message: "Profile update failed.",
	}

	DELETE_PROFILE = ErrorMessage{
		Code:    errorPrefix + "11012",
		Message: "Profile deletion failed.",
	}

	ADD_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11013",
		Message: "Add app data failed.",
	}

	GET_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11014",
		Message: "Fetching app data failed.",
	}

	UPDATE_APP_DATA = ErrorMessage{
		Code:    errorPrefix + "11015",
		Message: "Updating app data failed.",
	}

	MULTIPLE_PROFILE_FOUND = ErrorMessage{
		Code:        errorPrefix + "11016",
		Message:     "Multiple Profiles found.",
		Description: "Multiple user profiles record found for the given user_id",
	}

	UNIFICATION_RULE_NOT_FOUND = ErrorMessage{
		Code:        errorPrefix + "12001",
		Message:     "No unification rule found.",
		Description: "No unification rule defined for this organization for the provided rule_id..",
	}

	UNIFICATION_UPDATE_FAILED = ErrorMessage{
		Code:    errorPrefix + "12002",
		Message: "Validation failed for unification rule update.",
	}

	UNIFICATION_RULE_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "12003",
		Message: "Unification already exist.",
	}
	UNIFICATION_RULE_PRIORITY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "12004",
		Message: "Unification priority already taken.",
	}

	PROFILE_SCHEMA_ADD_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "13001",
		Message: "Invalid request payload.",
	}

	INVALID_ATTRIBUTE_NAME = ErrorMessage{
		Code:    errorPrefix + "13002",
		Message: "Invalid attribute name.",
	}

	SCHEMA_ATTRIBUTE_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "13003",
		Message: "Schema attribute already exists.",
	}

	ATTRIBUTE_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "13004",
		Message: "Attribute not found.",
	}

	CONSENT_CAT_VALIDATION = ErrorMessage{
		Code:    errorPrefix + "14001",
		Message: "Consent category validation failed",
	}

	CONSENT_CAT_ALREADY_EXISTS = ErrorMessage{
		Code:    errorPrefix + "14002",
		Message: "Consent category already exist.",
	}

	CONSENT_CAT_NOT_FOUND = ErrorMessage{
		Code:    errorPrefix + "14003",
		Message: "Consent category not found.",
	}

	CONSENT_CAT_ID = ErrorMessage{
		Code:    errorPrefix + "14004",
		Message: "Consent category Id is required.",
	}

	UPDATE_CONSENT_CATEGORY_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "14005",
		Message: "Invalid request payload for updating consent category.",
	}

	UPDATE_CONFIG_BAD_REQUEST = ErrorMessage{
		Code:    errorPrefix + "16002",
		Message: "Invalid request payload for updating admin configuration.",
	}

	CDS_NOT_ENABLED = ErrorMessage{
		Code:    errorPrefix + "16001",
		Message: "Customer data service is not enabled for the tenant.",
	}

	INVALID_FILTER_FORMAT = ErrorMessage{
		Code:    errorPrefix + "19001",
		Message: "Invalid filter format.",
	}
)
