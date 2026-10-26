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

package client

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/idp/credential"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/systemapp"
)

// Importing this package registers the identity providers it implements.
func init() {

	credential.Register(credential.TypeWSO2IS, func() (credential.Source, error) {
		identityClient, err := NewIdentityClientWithError(config.GetCDSRuntime().Config)
		if err != nil {
			return nil, err
		}
		return &wso2ISSource{client: identityClient}, nil
	})

	credential.Register(credential.TypeAsgardeo, func() (credential.Source, error) {
		identityClient, err := NewIdentityClientWithError(config.GetCDSRuntime().Config)
		if err != nil {
			return nil, err
		}
		return &asgardeoSource{client: identityClient}, nil
	})
}

// wso2Permission is how one identity provider of the WSO2 lineage expresses a
// permission: the API resource that owns it, and the scopes that grant it.
type wso2Permission struct {
	APIResource string
	Scopes      []string
}

// wso2Permissions maps what CDS asks for onto that model. Asgardeo shares it, since it
// shares the authorization model; the two differ in how a credential is obtained, not
// in what the permissions are called.
//
// It is the one place the Identity Server scope vocabulary appears. Nothing above the
// Source names an internal_ scope.
var wso2Permissions = map[credential.Permission]wso2Permission{
	credential.PermissionReadApplications: {
		APIResource: "/api/server/v1/applications",
		Scopes:      []string{"internal_application_mgt_view"},
	},
	credential.PermissionReadClaims: {
		APIResource: constants.IdentityServerDialectsPath,
		Scopes:      []string{"internal_claim_meta_view"},
	},
	credential.PermissionReadUsers: {
		APIResource: "/scim2/Users",
		Scopes:      []string{"internal_user_mgt_list", "internal_user_mgt_view"},
	},
}

// wso2Scopes returns the scopes that grant the permissions, sorted so that one set of
// permissions always produces the same scope string.
func wso2Scopes(permissions []credential.Permission) []string {

	seen := map[string]struct{}{}
	for _, permission := range permissions {
		for _, scope := range wso2Permissions[permission].Scopes {
			seen[scope] = struct{}{}
		}
	}

	scopes := make([]string, 0, len(seen))
	for scope := range seen {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes
}

// wso2ISSource authenticates to a WSO2 Identity Server the deployment runs itself,
// with the client credentials grant.
type wso2ISSource struct {
	client *IdentityClient
}

// Token obtains an organisation-scoped token with the client credentials grant.
//
// The credentials are the organisation's own wherever one has been provisioned for it,
// because the endpoint is that organisation's token endpoint and an Identity Server
// client can only authenticate at the token endpoint of the tenant it belongs to.
func (s *wso2ISSource) Token(orgHandle string, permissions []credential.Permission) (credential.Token, error) {

	authCfg := config.GetCDSRuntime().Config.AuthServer

	clientID, clientSecret := authCfg.ClientID, authCfg.ClientSecret
	if authCfg.SystemApplication.AutoProvision {
		// This call chain carries no context; see the note on Source.Token.
		creds, found, err := systemapp.Get(context.Background(), orgHandle)
		if err != nil {
			return credential.Token{}, err
		}
		if found {
			clientID, clientSecret = creds.ClientID, creds.ClientSecret
		}
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", strings.Join(wso2Scopes(permissions), constants.SpaceSeparator))

	endpoint := s.client.buildTokenEndpoint(orgHandle, authCfg.TokenEndpoint)
	token, err := s.client.requestToken(endpoint, clientID, clientSecret, form, orgHandle)
	if err != nil {
		return credential.Token{}, err
	}
	return credential.Token{Value: token, Scheme: credential.SchemeBearer}, nil
}

// Provision creates the organisation's application at the Identity Server. A
// self-managed Identity Server has nothing that registers CDS for it.
func (s *wso2ISSource) Provision(ctx context.Context, orgHandle string) error {

	return s.client.ProvisionSystemApplication(ctx, orgHandle, credential.ManagementPermissions())
}

// asgardeoSource authenticates to Asgardeo with the system application grant, which
// exchanges a super-tenant credential for an organisation-scoped one.
type asgardeoSource struct {
	client *IdentityClient
}

// Token obtains an organisation-scoped token with the system application grant. The
// result is presented under the SystemApp scheme rather than Bearer.
func (s *asgardeoSource) Token(orgHandle string, permissions []credential.Permission) (credential.Token, error) {

	authCfg := config.GetCDSRuntime().Config.AuthServer

	form := url.Values{}
	form.Set("grant_type", "system_app_grant")
	form.Set("scope", strings.Join(wso2Scopes(permissions), constants.SpaceSeparator))
	form.Set("organizationHandle", orgHandle)

	endpoint := s.client.buildTokenEndpoint(constants.SuperTenantDomain, authCfg.TokenEndpoint)
	token, err := s.client.requestToken(endpoint, authCfg.ClientID, authCfg.ClientSecret, form, orgHandle)
	if err != nil {
		return credential.Token{}, err
	}
	return credential.Token{Value: token, Scheme: credential.SchemeSystemApp}, nil
}

// Provision does nothing. Asgardeo registers CDS from its control plane, and the
// credentials reach CDS as configuration.
func (s *asgardeoSource) Provision(_ context.Context, orgHandle string) error {

	log.GetLogger().Debug(
		"Asgardeo provisions the CDS application from its control plane; nothing to do for organization: " + orgHandle)
	return nil
}
