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

// Package identity_provider defines the seam between CDS and whichever
// OAuth2/OIDC authorization server it is deployed against. Concrete
// implementations live in sibling packages (wso2is, thunder).
package identity_provider

import (
	idpmodel "github.com/wso2/identity-customer-data-service/internal/identity_provider/model"
	psmodel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
)

// Client is the minimal, provider-agnostic surface CDS needs from an
// identity provider: M2M token acquisition, inbound token introspection,
// application lookup, and local JWT verification.
type Client interface {
	// Name identifies the provider for logging/diagnostics, e.g. "wso2is" or "thunder".
	Name() string

	// FetchToken obtains an access token that CDS can use to call the
	// identity provider's own management APIs on behalf of orgHandle.
	FetchToken(orgHandle string) (string, error)

	// IntrospectToken checks an inbound token's validity (RFC 7662).
	IntrospectToken(token, orgHandle string) (map[string]interface{}, error)

	// FetchApplicationIdentifier looks up an application by clientId (and,
	// where supported, issuer) within orgHandle.
	FetchApplicationIdentifier(applicationIdentifier, orgHandle string) (idpmodel.ApplicationsListResponse, error)

	// VerifyJWT locally verifies a JWT's signature against the provider's
	// JWKS endpoint. Exposed for provider-level testing; not yet wired into
	// the request-authentication hot path (internal/system/authn) — see
	// docs/guides/identity-providers.md, Phase 2.
	VerifyJWT(tokenString string) (map[string]interface{}, error)
}

// SchemaSyncCapable is implemented only by providers that expose a
// claim/attribute-dialect model CDS can auto-sync into its own
// profile-schema store. Callers must type-assert a Client to this interface
// and treat a failed assertion as "this provider requires manual
// profile-schema management".
type SchemaSyncCapable interface {
	GetProfileSchema(orgHandle string) ([]psmodel.ProfileSchemaAttribute, error)
}
