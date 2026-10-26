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
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/idp/credential"
)

// The scope string the WSO2 sources send has to be exactly what was hardcoded in
// FetchToken before the permissions existed. An application provisioned against the old
// list keeps working, and a token keeps carrying what every management call here needs.
func TestManagementPermissionsProduceTheHistoricScopeString(t *testing.T) {

	const historic = "internal_application_mgt_view internal_claim_meta_view " +
		"internal_user_mgt_list internal_user_mgt_view"

	got := strings.Join(wso2Scopes(credential.ManagementPermissions()), constants.SpaceSeparator)
	if got != historic {
		t.Fatalf("scopes = %q, want %q", got, historic)
	}
}

func TestWSO2ScopesAreDeduplicatedAndOrdered(t *testing.T) {

	scopes := wso2Scopes([]credential.Permission{
		credential.PermissionReadUsers,
		credential.PermissionReadApplications,
		credential.PermissionReadUsers,
	})

	want := []string{"internal_application_mgt_view", "internal_user_mgt_list", "internal_user_mgt_view"}
	if len(scopes) != len(want) {
		t.Fatalf("scopes = %v, want %v", scopes, want)
	}
	for i := range want {
		if scopes[i] != want[i] {
			t.Fatalf("scopes = %v, want %v", scopes, want)
		}
	}
}

// An unknown permission maps to no scopes rather than to a wrong one, so a token is
// never quietly issued carrying something else.
func TestWSO2ScopesIgnoresAnUnmappedPermission(t *testing.T) {

	if scopes := wso2Scopes([]credential.Permission{"read:nothing"}); len(scopes) != 0 {
		t.Fatalf("scopes = %v, want none", scopes)
	}
}

// Each source presents its credential under its own scheme. This is the whole reason
// Token carries one: the call sites set a header and no longer branch on the provider.
func TestSourcesPresentTheirOwnScheme(t *testing.T) {

	cases := map[string]struct {
		newSource  func(*IdentityClient) credential.Source
		wantScheme string
		wantGrant  string
	}{
		"wso2is uses client credentials under Bearer": {
			newSource:  func(c *IdentityClient) credential.Source { return &wso2ISSource{client: c} },
			wantScheme: credential.SchemeBearer,
			wantGrant:  "client_credentials",
		},
		"asgardeo uses the system application grant under SystemApp": {
			newSource:  func(c *IdentityClient) credential.Source { return &asgardeoSource{client: c} },
			wantScheme: credential.SchemeSystemApp,
			wantGrant:  "system_app_grant",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			identityClient, stub := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"access_token":"issued-token"}`))
			})

			token, err := tc.newSource(identityClient).Token("acme", credential.ManagementPermissions())
			if err != nil {
				t.Fatalf("Token: %v", err)
			}
			if token.Value != "issued-token" {
				t.Fatalf("token value = %q", token.Value)
			}
			if token.Scheme != tc.wantScheme {
				t.Fatalf("scheme = %q, want %q", token.Scheme, tc.wantScheme)
			}
			if body := stub.bodies[0]; !strings.Contains(body, "grant_type="+tc.wantGrant) {
				t.Fatalf("request does not use %s: %s", tc.wantGrant, body)
			}
		})
	}
}

// Asgardeo registers CDS from its control plane, so provisioning must do nothing rather
// than attempt management calls that would be refused.
func TestAsgardeoProvisionDoesNothing(t *testing.T) {

	identityClient, stub := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("Asgardeo provisioning called the management API")
	})

	source := &asgardeoSource{client: identityClient}
	if err := source.Provision(context.Background(), "acme"); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if len(stub.requests) != 0 {
		t.Fatalf("made %d requests, want none", len(stub.requests))
	}
}

// Both providers of the WSO2 lineage have to be registered by importing this package,
// since that is what makes idp.type resolvable at start.
func TestBothWSO2ProvidersAreRegistered(t *testing.T) {

	for _, idpType := range []string{credential.TypeWSO2IS, credential.TypeAsgardeo} {
		if _, err := credential.NewSource(idpType); err != nil {
			t.Fatalf("NewSource(%q): %v", idpType, err)
		}
	}
}
