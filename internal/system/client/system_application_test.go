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
	goerrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {

	if err := log.Init("ERROR"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// stubIdentityServer stands in for the Identity Server management APIs. It
// records every request so a test can assert on the calls the provisioning made,
// not only on what it returned.
type stubIdentityServer struct {
	server   *httptest.Server
	requests []*http.Request
	bodies   []string
}

func newStubIdentityServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*IdentityClient, *stubIdentityServer) {

	t.Helper()

	stub := &stubIdentityServer{}
	stub.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stub.requests = append(stub.requests, r)
		stub.bodies = append(stub.bodies, string(body))
		handler(w, r)
	}))
	t.Cleanup(stub.server.Close)

	config.OverrideCDSRuntime(config.Config{
		AuthServer: config.AuthServerConfig{
			AdminUsername: "admin",
			AdminPassword: "admin",
			SystemApplication: config.SystemApplicationConfig{
				AutoProvision: true,
				NamePrefix:    "CDS System App",
				EncryptionKey: "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
			},
		},
	})

	return &IdentityClient{
		BaseURL:    strings.TrimPrefix(stub.server.URL, "https://"),
		HTTPClient: stub.server.Client(),
	}, stub
}

// The Identity Server filter is a search, so a response can carry applications
// whose names merely resemble the one asked for. Taking the first hit would
// subscribe some unrelated application to the management APIs and then hand CDS
// its credentials.
func TestFindApplicationByNameConfirmsTheName(t *testing.T) {

	identityClient, _ := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applications":[
			{"id":"other-app","name":"CDS System App - acme-staging"},
			{"id":"wanted-app","name":"CDS System App - acme"}
		]}`))
	})

	appID, err := identityClient.findApplicationByName("CDS System App - acme", "acme")
	if err != nil {
		t.Fatalf("findApplicationByName: %v", err)
	}
	if appID != "wanted-app" {
		t.Fatalf("resolved %q, want wanted-app", appID)
	}
}

func TestFindApplicationByNameReturnsEmptyWhenAbsent(t *testing.T) {

	identityClient, _ := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applications":[]}`))
	})

	appID, err := identityClient.findApplicationByName("CDS System App - acme", "acme")
	if err != nil {
		t.Fatalf("findApplicationByName: %v", err)
	}
	if appID != "" {
		t.Fatalf("resolved %q for an organisation with no application", appID)
	}
}

// Re-running provisioning against an application that already carries the
// resource has to add scopes rather than re-subscribe: a second POST is refused,
// which would strand an adopted application half-subscribed.
func TestAuthorizeSystemAPIPatchesAnAlreadySubscribedResource(t *testing.T) {

	identityClient, stub := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api-resources"):
			_, _ = w.Write([]byte(`{"apiResources":[{"id":"res-1","identifier":"/scim2/Users"}]}`))
		case strings.HasSuffix(r.URL.Path, "/authorized-apis") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":"res-1"}]`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	err := identityClient.authorizeSystemAPI("app-1", "acme", "/scim2/Users",
		[]string{"internal_user_mgt_list"})
	if err != nil {
		t.Fatalf("authorizeSystemAPI: %v", err)
	}

	last := stub.requests[len(stub.requests)-1]
	if last.Method != http.MethodPatch {
		t.Fatalf("subscribed with %s, want PATCH for an already subscribed resource", last.Method)
	}
	if !strings.HasSuffix(last.URL.Path, "/authorized-apis/res-1") {
		t.Fatalf("patched %s, want the resource path", last.URL.Path)
	}
}

func TestAuthorizeSystemAPIPostsAnUnsubscribedResource(t *testing.T) {

	identityClient, stub := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api-resources"):
			_, _ = w.Write([]byte(`{"apiResources":[{"id":"res-1","identifier":"/scim2/Users"}]}`))
		case strings.HasSuffix(r.URL.Path, "/authorized-apis") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	err := identityClient.authorizeSystemAPI("app-1", "acme", "/scim2/Users",
		[]string{"internal_user_mgt_list"})
	if err != nil {
		t.Fatalf("authorizeSystemAPI: %v", err)
	}

	last := stub.requests[len(stub.requests)-1]
	if last.Method != http.MethodPost {
		t.Fatalf("subscribed with %s, want POST for an unsubscribed resource", last.Method)
	}

	body := stub.bodies[len(stub.bodies)-1]
	if !strings.Contains(body, "internal_user_mgt_list") {
		t.Fatalf("subscription does not carry the scope: %s", body)
	}
	if !strings.Contains(body, `"policyIdentifier":"RBAC"`) {
		t.Fatalf("subscription does not carry an authorization policy: %s", body)
	}
}

// An Identity Server that does not carry the CDS API resources cannot serve a
// provisioned application, and saying so names the missing resource rather than
// leaving an application subscribed to nothing.
func TestAuthorizeSystemAPINamesAMissingResource(t *testing.T) {

	identityClient, _ := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"apiResources":[]}`))
	})

	err := identityClient.authorizeSystemAPI("app-1", "acme", "/scim2/Users", []string{"internal_user_mgt_list"})
	if err == nil {
		t.Fatal("expected a missing API resource to fail provisioning")
	}

	// A ServerError prints only its code and message, so the detail an operator acts
	// on lives in the description.
	var serverErr *errors2.ServerError
	if !goerrors.As(err, &serverErr) {
		t.Fatalf("got %T, want a ServerError", err)
	}
	if !strings.Contains(serverErr.Description, "/scim2/Users") {
		t.Fatalf("description does not name the missing resource: %s", serverErr.Description)
	}
}

func TestReadOIDCCredentialsRefusesAnApplicationWithoutCredentials(t *testing.T) {

	identityClient, _ := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"clientId":"abc"}`))
	})

	_, _, err := identityClient.readOIDCCredentials("app-1", "acme")
	if err == nil {
		t.Fatal("expected an application with no client secret to be refused")
	}
}

// A failure has to carry the status and the Identity Server's own message, since
// that is all an operator has to work from, but not the filters - those carry
// organisation and application names into logs that do not need them.
func TestManagementRequestReportsFailuresWithoutTheQueryString(t *testing.T) {

	identityClient, _ := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"description":"insufficient permissions"}`))
	})

	_, err := identityClient.managementRequest(http.MethodGet,
		"/api/server/v1/applications?filter=name+eq+secret-org", "acme", nil)
	if err == nil {
		t.Fatal("expected a 403 to be reported as an error")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "insufficient permissions") {
		t.Fatalf("error loses the status or the server message: %v", err)
	}
	if strings.Contains(err.Error(), "secret-org") {
		t.Fatalf("error carries the query string: %v", err)
	}
}

func TestManagementRequestAuthenticatesAsTheAdministrator(t *testing.T) {

	identityClient, stub := newStubIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	if _, err := identityClient.managementRequest(http.MethodGet,
		"/api/server/v1/applications", "acme", nil); err != nil {
		t.Fatalf("managementRequest: %v", err)
	}

	username, password, ok := stub.requests[0].BasicAuth()
	if !ok || username != "admin" || password != "admin" {
		t.Fatalf("request did not carry the configured administrator credentials")
	}
	if stub.requests[0].URL.Path != "/t/acme/api/server/v1/applications" {
		t.Fatalf("called %s, want the organisation-scoped path", stub.requests[0].URL.Path)
	}
}
