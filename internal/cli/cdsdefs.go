/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

package cli

// The names and identifiers enablement creates in the Identity Server.
//
// These are a contract, not a convention: the API identifiers match the
// [[api_resources]] entries seeded from deployment.toml, the scopes match
// auth_server.required_scopes in the CDS configuration, and the audience
// matches the check the CDS authentication layer performs on every token.
// Changing one of them here without changing its counterpart breaks
// authorisation at runtime rather than at enablement.
const (
	// systemAppName is the application the CDS integration authenticates as:
	// CDS presents it when calling the Identity Server's management APIs, and
	// enablement presents it when calling CDS's own configuration API.
	//
	// An application for an operator's own API consumers is deliberately not
	// created. Which clients may call CDS, and with which scopes, is theirs
	// to decide; enablement establishes the relationship between CDS and one
	// Identity Server and nothing beyond it.
	systemAppName = "Customer Data Service"

	// cdsAudience is the audience CDS requires in the tokens it accepts.
	cdsAudience = "iam-cds"

	// tokenExpirySeconds is the lifetime of the tokens these applications
	// issue. One hour is the Identity Server's own default for a management
	// application and there is no reason for CDS to differ.
	tokenExpirySeconds = 3600

	defaultOrganization      = "carbon.super"
	defaultISManagementURL   = "https://localhost:9443"
	defaultISAdminUser       = "admin"
	isAdminPasswordEnvVar    = "CDS_IS_ADMIN_PASSWORD"
	isKeystorePasswordEnvVar = "CDS_IS_KEYSTORE_PASSWORD"
	defaultISStorePassword   = "wso2carbon"
)

// cdsAPIResources are the CDS APIs the Identity Server registers, from the
// [[api_resources]] entries that enablement writes into deployment.toml. Their
// presence is how a restarted server is told apart from one that has not
// picked the section up yet.
var cdsAPIResources = []string{
	"/cds/api/v1/profiles",
	"/cds/api/v1/profile-schema",
	"/cds/api/v1/unification-rules",
	"/cds/api/v1/config",
	"/cds/api/v1/consent-categories",
}

// managementAPIGrants are the Identity Server APIs CDS itself calls: it reads
// applications to resolve a client, claim dialects to synchronise the profile
// schema, and users to resolve a profile's owner.
var managementAPIGrants = []struct {
	Identifier string
	Scopes     []string
}{
	{"/api/server/v1/applications", []string{"internal_application_mgt_view"}},
	{"/api/server/v1/claim-dialects", []string{"internal_claim_meta_view"}},
	{"/scim2/Users", []string{"internal_user_mgt_list", "internal_user_mgt_view"}},
}

// consoleFeatureScopes gate the Customer Data section of the Console. They are
// registered as a CONSOLE_FEATURE API resource by the same toml section.
var consoleFeatureScopes = []string{
	"console:customerData",
	"console:customerData_view",
	"console:customerData_edit",
}
