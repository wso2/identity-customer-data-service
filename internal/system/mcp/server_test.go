/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

const (
	testOrg         = "acme"
	viewToken       = "view-token"
	viewUpdateToken = "view-update-token"
)

// tokens maps each test token to the org it was issued for and the scopes it carries.
var tokens = map[string]struct{ org, scope string }{
	viewToken:       {testOrg, "internal_cds_profile_view"},
	viewUpdateToken: {testOrg, "internal_cds_profile_view internal_cds_profile_update internal_cds_profile_schema_view"},
}

func TestMain(m *testing.M) {

	_ = log.Init("ERROR")
	os.Exit(m.Run())
}

func fakeValidateToken(token, orgHandle string) (map[string]interface{}, error) {

	t, ok := tokens[token]
	if !ok || t.org != orgHandle {
		return nil, errors.New("invalid token")
	}
	return map[string]interface{}{"scope": t.scope, "azp": "agent-app"}, nil
}

// restCall is what a stubbed REST handler saw.
type restCall struct {
	method, path, query, org, authorization, body string
}

// fakeREST stands in for the REST handlers, recording each call and answering with canned JSON.
type fakeREST struct {
	mu    sync.Mutex
	calls []restCall
}

func (f *fakeREST) register(routes *http.ServeMux) {

	answer := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.record(r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		}
	}
	routes.HandleFunc("GET /cds/api/v1/profiles", answer(`{"pagination":{"count":0},"items":[]}`))
	routes.HandleFunc("GET /cds/api/v1/profiles/{profileId}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if r.PathValue("profileId") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error_code":"CDS-15002","error_message":"Profile not found",`+
				`"error_description":"Profile missing not found"}`)
			return
		}
		_, _ = io.WriteString(w, `{"profile_id":"p1","user_id":"u1","meta":{"updated_at":"2026-10-01T00:00:00Z"},`+
			`"identity_attributes":{"email":"jane@example.com"},"traits":{"segment":"vip"},`+
			`"application_data":{"app-a":{"cart":3},"app-b":{"theme":"dark"}},`+
			`"merged_from":[{"profile_id":"p2"},{"profile_id":"p3"}]}`)
	})
	routes.HandleFunc("GET /cds/api/v1/profiles/{profileId}/consents",
		answer(`[{"category_identifier":"marketing","is_consented":true}]`))
	routes.HandleFunc("PATCH /cds/api/v1/profiles/{profileId}", answer(`{"profile_id":"p1"}`))
	routes.HandleFunc("GET /cds/api/v1/profile-schema", answer(`{"traits":[]}`))
}

func (f *fakeREST) record(r *http.Request) {

	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, restCall{
		method:        r.Method,
		path:          r.URL.Path,
		query:         r.URL.RawQuery,
		org:           utils.ExtractOrgHandleFromPath(r),
		authorization: r.Header.Get("Authorization"),
		body:          string(body),
	})
}

func (f *fakeREST) lastCall(t *testing.T) restCall {

	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("no REST call was made")
	}
	return f.calls[len(f.calls)-1]
}

// newTestServer serves the endpoint the way RegisterServices does: below the /t/ dispatcher,
// on the same mux as the REST routes.
func newTestServer(t *testing.T, readOnly bool) (*httptest.Server, *fakeREST) {

	t.Helper()
	conf := config.Config{ServerURL: "https://cds.example.com"}
	conf.AuthServer.Host = "is.example.com"
	conf.AuthServer.Port = "9443"
	conf.AuthServer.RequiredScopes = map[string][]string{
		opProfileView:         {"internal_cds_profile_view"},
		opProfileUpdate:       {"internal_cds_profile_update"},
		opProfileSchemaView:   {"internal_cds_profile_schema_view"},
		opConsentCategoryView: {"internal_cds_consent_category_view"},
	}
	config.OverrideCDSRuntime(conf)

	rest := &fakeREST{}
	root, routes := http.NewServeMux(), http.NewServeMux()
	rest.register(routes)

	s := &server{routes: routes, validateToken: fakeValidateToken, readOnly: readOnly}
	routes.Handle(Path, s.endpoint())
	root.HandleFunc(metadataPathPrefix+"/", s.serveMetadata)
	utils.MountTenantDispatcher(root, routes.ServeHTTP)

	ts := httptest.NewServer(root)
	t.Cleanup(ts.Close)
	return ts, rest
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {

	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func connect(t *testing.T, ts *httptest.Server, org, token string) *mcpsdk.ClientSession {

	t.Helper()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-agent"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:             ts.URL + "/t/" + org + Path,
		HTTPClient:           &http.Client{Transport: bearer{token}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func toolNames(t *testing.T, session *mcpsdk.ClientSession) []string {

	t.Helper()
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func callTool(t *testing.T, session *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {

	t.Helper()
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("calling %s: %v", name, err)
	}
	return res
}

func textOf(res *mcpsdk.CallToolResult) string {

	var parts []string
	for _, c := range res.Content {
		if text, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// TestRequestWithoutTokenIsRefused checks that the 401 names the metadata URL, which is
// how an MCP client finds out where to get a token.
func TestRequestWithoutTokenIsRefused(t *testing.T) {

	ts, _ := newTestServer(t, false)

	resp, err := http.Post(ts.URL+"/t/"+testOrg+Path, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	want := `resource_metadata="https://cds.example.com/.well-known/oauth-protected-resource/t/acme/cds/mcp"`
	if got := resp.Header.Get("WWW-Authenticate"); !strings.Contains(got, want) {
		t.Errorf("WWW-Authenticate = %q, want it to contain %s", got, want)
	}
}

// TestTokenForAnotherOrgIsRefused checks that the org comes from the path the token is
// validated against, so a token for one org cannot reach another org's profiles.
func TestTokenForAnotherOrgIsRefused(t *testing.T) {

	ts, _ := newTestServer(t, false)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-agent"}, nil)
	_, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   ts.URL + "/t/other-org" + Path,
		HTTPClient: &http.Client{Transport: bearer{viewToken}},
		MaxRetries: -1,
	}, nil)
	if err == nil {
		t.Fatal("connecting to another org with an acme token succeeded")
	}
}

func TestToolsFollowScopes(t *testing.T) {

	ts, _ := newTestServer(t, false)

	viewOnly := toolNames(t, connect(t, ts, testOrg, viewToken))
	wantView := []string{"cds_find_profile", "cds_get_profile", "cds_get_profile_consents",
		"cds_get_profile_summary", "cds_search_profiles"}
	if !slices.Equal(viewOnly, wantView) {
		t.Errorf("view scope lists %v, want %v", viewOnly, wantView)
	}

	withUpdate := toolNames(t, connect(t, ts, testOrg, viewUpdateToken))
	if !slices.Contains(withUpdate, "cds_patch_profile") {
		t.Errorf("update scope lists %v, want cds_patch_profile among them", withUpdate)
	}
}

func TestReadOnlyLeavesOutPatch(t *testing.T) {

	ts, _ := newTestServer(t, true)

	if names := toolNames(t, connect(t, ts, testOrg, viewUpdateToken)); slices.Contains(names, "cds_patch_profile") {
		t.Errorf("read_only lists %v, want no cds_patch_profile", names)
	}
}

func TestResourcesFollowScopes(t *testing.T) {

	ts, _ := newTestServer(t, false)

	res, err := connect(t, ts, testOrg, viewUpdateToken).ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var uris []string
	for _, r := range res.Resources {
		uris = append(uris, r.URI)
	}
	// The token carries the schema scope but not the consent category one.
	if !slices.Equal(uris, []string{"cds://profile-schema"}) {
		t.Errorf("resources = %v, want only cds://profile-schema", uris)
	}
}

// TestToolCallsGoThroughREST checks that a tool reaches the REST handler with the agent's
// own token and the org from the path, so the handler applies its usual checks and filters.
func TestToolCallsGoThroughREST(t *testing.T) {

	ts, rest := newTestServer(t, false)
	session := connect(t, ts, testOrg, viewToken)

	res := callTool(t, session, "cds_find_profile", map[string]any{
		"attribute": "identity_attributes.email",
		"value":     "jane@example.com",
	})
	if res.IsError {
		t.Fatalf("cds_find_profile failed: %s", textOf(res))
	}

	call := rest.lastCall(t)
	if call.method != http.MethodGet || call.path != "/cds/api/v1/profiles" {
		t.Errorf("REST call = %s %s, want GET /cds/api/v1/profiles", call.method, call.path)
	}
	if call.org != testOrg {
		t.Errorf("org = %q, want %q", call.org, testOrg)
	}
	if call.authorization != "Bearer "+viewToken {
		t.Errorf("Authorization = %q, want the agent's token", call.authorization)
	}
	if want := "attributes=identity_attributes%2Ctraits&filter=identity_attributes.email+eq+jane%40example.com"; call.query != want {
		t.Errorf("query = %q, want %q", call.query, want)
	}
}

func TestFindProfileNeedsOneWayToLookUp(t *testing.T) {

	ts, _ := newTestServer(t, false)
	session := connect(t, ts, testOrg, viewToken)

	for name, args := range map[string]map[string]any{
		"nothing":       {},
		"no value":      {"attribute": "identity_attributes.email"},
		"both":          {"user_id": "u1", "attribute": "identity_attributes.email", "value": "x"},
		"spaced fields": {"attribute": "identity_attributes.email eq", "value": "x"},
	} {
		if res := callTool(t, session, "cds_find_profile", args); !res.IsError {
			t.Errorf("%s: cds_find_profile succeeded, want a tool error", name)
		}
	}
}

func TestRESTErrorBecomesToolError(t *testing.T) {

	ts, _ := newTestServer(t, false)

	res := callTool(t, connect(t, ts, testOrg, viewToken), "cds_get_profile", map[string]any{"profile_id": "missing"})
	if !res.IsError {
		t.Fatal("getting a missing profile succeeded")
	}
	if got := textOf(res); !strings.Contains(got, "Profile missing not found") {
		t.Errorf("error = %q, want the API's description", got)
	}
}

// TestPatchCannotTouchIdentityAttributes pins the reason the tool has no identity_attributes
// field: changing one can unify the profile with another, which cannot be undone.
func TestPatchCannotTouchIdentityAttributes(t *testing.T) {

	ts, rest := newTestServer(t, false)
	session := connect(t, ts, testOrg, viewUpdateToken)

	res := callTool(t, session, "cds_patch_profile", map[string]any{
		"profile_id":          "p1",
		"identity_attributes": map[string]any{"email": "someone-else@example.com"},
	})
	if !res.IsError {
		t.Fatal("a patch carrying identity_attributes was accepted")
	}
	for _, call := range rest.calls {
		if call.method == http.MethodPatch {
			t.Fatalf("a refused patch still reached REST: %s", call.body)
		}
	}

	res = callTool(t, session, "cds_patch_profile", map[string]any{
		"profile_id": "p1",
		"traits":     map[string]any{"segment": "vip"},
	})
	if res.IsError {
		t.Fatalf("patching traits failed: %s", textOf(res))
	}
	call := rest.lastCall(t)
	if call.method != http.MethodPatch || call.path != "/cds/api/v1/profiles/p1" {
		t.Errorf("REST call = %s %s, want PATCH /cds/api/v1/profiles/p1", call.method, call.path)
	}
	if call.body != `{"traits":{"segment":"vip"}}` {
		t.Errorf("body = %s, want only the traits", call.body)
	}
}

func TestProfileSummary(t *testing.T) {

	ts, _ := newTestServer(t, false)

	res := callTool(t, connect(t, ts, testOrg, viewToken), "cds_get_profile_summary", map[string]any{"profile_id": "p1"})
	if res.IsError {
		t.Fatalf("cds_get_profile_summary failed: %s", textOf(res))
	}
	var summary profileSummary
	if err := json.Unmarshal([]byte(textOf(res)), &summary); err != nil {
		t.Fatal(err)
	}
	slices.Sort(summary.Applications)
	if !slices.Equal(summary.Applications, []string{"app-a", "app-b"}) {
		t.Errorf("applications = %v, want the app IDs without their data", summary.Applications)
	}
	if summary.MergedFromCount != 2 {
		t.Errorf("merged_from_count = %d, want 2", summary.MergedFromCount)
	}
	if !strings.Contains(string(summary.Consents), "marketing") {
		t.Errorf("consents = %s, want the profile's consents", summary.Consents)
	}
	if strings.Contains(textOf(res), "cart") {
		t.Error("the summary carries application data")
	}
}

func TestProtectedResourceMetadata(t *testing.T) {

	ts, _ := newTestServer(t, false)

	for path, wantIssuer := range map[string]string{
		"/t/acme/cds/mcp": "https://is.example.com:9443/t/acme/oauth2/token",
		"/o/6f1d3c2a-8b7e-4f5a-9c0d-1e2f3a4b5c6d/cds/mcp": "https://is.example.com:9443/t/carbon.super/o/" +
			"6f1d3c2a-8b7e-4f5a-9c0d-1e2f3a4b5c6d/oauth2/token",
	} {
		resp, err := http.Get(ts.URL + metadataPathPrefix + path)
		if err != nil {
			t.Fatal(err)
		}
		var meta struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
			ScopesSupported      []string `json:"scopes_supported"`
		}
		err = json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if meta.Resource != "https://cds.example.com"+path {
			t.Errorf("%s: resource = %q", path, meta.Resource)
		}
		if !slices.Equal(meta.AuthorizationServers, []string{wantIssuer}) {
			t.Errorf("%s: authorization_servers = %v, want %s", path, meta.AuthorizationServers, wantIssuer)
		}
		if !slices.Contains(meta.ScopesSupported, "internal_cds_profile_update") {
			t.Errorf("%s: scopes_supported = %v, want the update scope", path, meta.ScopesSupported)
		}
	}

	resp, err := http.Get(ts.URL + metadataPathPrefix + "/t/acme/cds/api/v1/profiles")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("metadata for a non-MCP path: status = %d, want 404", resp.StatusCode)
	}
}

// The org reaches the handlers through the context the /t/ dispatcher sets; make sure the
// key the client writes is the one the handlers read.
func TestRESTClientSetsOrgContext(t *testing.T) {

	var got any
	routes := http.NewServeMux()
	routes.HandleFunc("GET /cds/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		got = r.Context().Value(constants.TenantContextKey)
	})
	c := &restClient{routes: routes, orgHandle: testOrg, pathPrefix: "/t/"}

	if _, err := c.get(context.Background(), "/ping", nil); err != nil {
		t.Fatal(err)
	}
	if got != testOrg {
		t.Errorf("org in context = %v, want %q", got, testOrg)
	}
}
