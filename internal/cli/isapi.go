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

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// isAPI calls the Identity Server management APIs as the administrator.
//
// Provisioning runs against a server the operator already administers, so the
// credentials are theirs rather than something enablement mints. Everything
// here is idempotent: each step asks what exists before it creates anything,
// so enablement can be re-run after a failure without duplicating
// applications, claims or authorisations.
type isAPI struct {
	base     string
	user     string
	password string
	client   *http.Client
}

const mgmtAPI = "/api/server/v1"

// newISAPI builds a client for the Identity Server at base.
//
// The IS certificate is verified against trustPEM when one was exported during
// enablement, which is the normal case. insecure skips verification and is for
// a first enablement against a server whose certificate is not yet available
// locally; it is an explicit choice the operator makes and the caller reports.
func newISAPI(base, user, password, trustPEM string, insecure bool) (*isAPI, error) {

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case insecure:
		tlsConfig.InsecureSkipVerify = true // #nosec G402 -- explicit --insecure, reported to the operator
	case trustPEM != "":
		pem, err := os.ReadFile(trustPEM)
		if err != nil {
			return nil, fmt.Errorf("could not read the Identity Server certificate %s: %w", trustPEM, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s holds no usable certificate", trustPEM)
		}
		tlsConfig.RootCAs = pool
	}

	return &isAPI{
		base:     strings.TrimSuffix(base, "/"),
		user:     user,
		password: password,
		client: &http.Client{
			Timeout:   2 * time.Minute,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}, nil
}

// call performs one request and decodes a JSON response into out, which may be
// nil when the response body is not needed.
func (api *isAPI) call(method, endpoint string, body, out any) error {

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("could not encode the request for %s: %w", endpoint, err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, api.base+endpoint, payload)
	if err != nil {
		return fmt.Errorf("could not build the request for %s: %w", endpoint, err)
	}
	req.SetBasicAuth(api.user, api.password)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the Identity Server at %s: %w", api.base, err)
	}
	defer func() { _ = resp.Body.Close() }()

	content, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("the Identity Server rejected the administrator credentials (HTTP 401)")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s returned HTTP %d: %s", method, endpoint, resp.StatusCode,
			strings.TrimSpace(firstLine(string(content))))
	}
	if out == nil || len(bytes.TrimSpace(content)) == 0 {
		return nil
	}
	if err := json.Unmarshal(content, out); err != nil {
		return fmt.Errorf("could not read the response of %s %s: %w", method, endpoint, err)
	}
	return nil
}

func firstLine(s string) string {
	if index := strings.IndexByte(s, '\n'); index >= 0 {
		return s[:index]
	}
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

// ---------------------------------------------------------------------------
// API resources
// ---------------------------------------------------------------------------

// apiResourceID resolves an API resource by its identifier, returning "" when
// the Identity Server has not registered it.
//
// The CDS API resources are seeded from deployment.toml rather than created
// over REST, because IS reserves the internal_ scope prefix for system APIs
// and its REST API refuses those names. So an empty result here means the
// restart has not happened, or the toml section did not take effect.
func (api *isAPI) apiResourceID(identifier string) (string, error) {

	var page struct {
		APIResources []struct {
			ID string `json:"id"`
		} `json:"apiResources"`
	}
	endpoint := mgmtAPI + "/api-resources?filter=" + url.QueryEscape("identifier eq "+identifier)
	if err := api.call(http.MethodGet, endpoint, nil, &page); err != nil {
		return "", err
	}
	if len(page.APIResources) == 0 {
		return "", nil
	}
	return page.APIResources[0].ID, nil
}

func (api *isAPI) apiResourceScopes(resourceID string) ([]string, error) {

	var scopes []struct {
		Name string `json:"name"`
	}
	if err := api.call(http.MethodGet, mgmtAPI+"/api-resources/"+resourceID+"/scopes", nil, &scopes); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		names = append(names, scope.Name)
	}
	return names, nil
}

// ---------------------------------------------------------------------------
// Applications
// ---------------------------------------------------------------------------

func (api *isAPI) applicationID(name string) (string, error) {

	var page struct {
		Applications []struct {
			ID string `json:"id"`
		} `json:"applications"`
	}
	endpoint := mgmtAPI + "/applications?filter=" + url.QueryEscape("name eq "+name)
	if err := api.call(http.MethodGet, endpoint, nil, &page); err != nil {
		return "", err
	}
	if len(page.Applications) == 0 {
		return "", nil
	}
	return page.Applications[0].ID, nil
}

// createM2MApplication registers a machine-to-machine application. CDS and its
// clients authenticate as themselves rather than on behalf of a user, so
// client credentials is the only grant they need.
func (api *isAPI) createM2MApplication(name, description string, managementApp bool, tokenExpiry int) (string, error) {

	body := map[string]any{
		"name":            name,
		"description":     description,
		"isManagementApp": managementApp,
		"inboundProtocolConfiguration": map[string]any{
			"oidc": map[string]any{
				"grantTypes": []string{"client_credentials"},
				"accessToken": map[string]any{
					"type":                                  "JWT",
					"userAccessTokenExpiryInSeconds":        tokenExpiry,
					"applicationAccessTokenExpiryInSeconds": tokenExpiry,
				},
			},
		},
	}
	if err := api.call(http.MethodPost, mgmtAPI+"/applications", body, nil); err != nil {
		return "", err
	}
	return api.applicationID(name)
}

// oidcConfig is the inbound OIDC configuration of an application. Only the
// fields enablement reads or rewrites are named; the rest ride along in Extra
// so that a PUT returns the configuration unchanged apart from the one field
// that was meant to change.
type oidcConfig struct {
	ClientID     string         `json:"clientId"`
	ClientSecret string         `json:"clientSecret"`
	AccessToken  map[string]any `json:"accessToken,omitempty"`
	IDToken      map[string]any `json:"idToken,omitempty"`
	Extra        map[string]any `json:"-"`
}

func (api *isAPI) oidcConfig(appID string) (*oidcConfig, map[string]any, error) {

	raw := map[string]any{}
	if err := api.call(http.MethodGet, mgmtAPI+"/applications/"+appID+"/inbound-protocols/oidc", nil, &raw); err != nil {
		return nil, nil, err
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the OIDC configuration of %s: %w", appID, err)
	}
	var config oidcConfig
	if err := json.Unmarshal(encoded, &config); err != nil {
		return nil, nil, fmt.Errorf("could not read the OIDC configuration of %s: %w", appID, err)
	}
	return &config, raw, nil
}

// addTokenAudience adds the CDS audience to an application's issued tokens, so
// that the audience check in the CDS authentication layer accepts them. It
// reports whether anything changed.
func (api *isAPI) addTokenAudience(appID, audience string) (bool, error) {

	_, raw, err := api.oidcConfig(appID)
	if err != nil {
		return false, err
	}

	idToken, _ := raw["idToken"].(map[string]any)
	if idToken == nil {
		idToken = map[string]any{}
	}
	current, _ := idToken["audience"].([]any)
	for _, existing := range current {
		if fmt.Sprintf("%v", existing) == audience {
			return false, nil
		}
	}
	idToken["audience"] = append(current, audience)
	raw["idToken"] = idToken

	// The PUT replaces the whole inbound configuration, which is why the
	// untouched fields are sent back exactly as they were read.
	if err := api.call(http.MethodPut, mgmtAPI+"/applications/"+appID+"/inbound-protocols/oidc", raw, nil); err != nil {
		return false, err
	}
	return true, nil
}

// authorizeAPI grants an application the named scopes on an API resource,
// adding the authorisation when it is absent and extending it when it is not.
func (api *isAPI) authorizeAPI(appID, identifier string, scopes []string) error {

	resourceID, err := api.apiResourceID(identifier)
	if err != nil {
		return err
	}
	if resourceID == "" {
		return fmt.Errorf("the Identity Server has no API resource %s", identifier)
	}

	var authorized []struct {
		ID string `json:"id"`
	}
	if err := api.call(http.MethodGet, mgmtAPI+"/applications/"+appID+"/authorized-apis", nil, &authorized); err != nil {
		return err
	}
	for _, existing := range authorized {
		if existing.ID != resourceID {
			continue
		}
		return api.call(http.MethodPatch,
			mgmtAPI+"/applications/"+appID+"/authorized-apis/"+resourceID,
			map[string]any{"addedScopes": scopes, "removedScopes": []string{}}, nil)
	}

	return api.call(http.MethodPost, mgmtAPI+"/applications/"+appID+"/authorized-apis",
		map[string]any{"id": resourceID, "policyIdentifier": "RBAC", "scopes": scopes}, nil)
}

// ---------------------------------------------------------------------------
// Claims
// ---------------------------------------------------------------------------

type claim struct {
	ID       string `json:"id"`
	ClaimURI string `json:"claimURI"`
}

// ensureLocalClaim creates the CDS profile claim when it is absent, reporting
// whether it had to.
func (api *isAPI) ensureLocalClaim(claimURI, displayName, description, attribute string) (bool, error) {

	var claims []claim
	if err := api.call(http.MethodGet, mgmtAPI+"/claim-dialects/local/claims", nil, &claims); err != nil {
		return false, err
	}
	for _, existing := range claims {
		if existing.ClaimURI == claimURI {
			return false, nil
		}
	}

	body := map[string]any{
		"claimURI":           claimURI,
		"description":        description,
		"displayName":        displayName,
		"displayOrder":       0,
		"readOnly":           false,
		"required":           false,
		"supportedByDefault": false,
		"attributeMapping": []map[string]string{
			{"mappedAttribute": attribute, "userstore": "PRIMARY"},
		},
		"properties": []any{},
	}
	if err := api.call(http.MethodPost, mgmtAPI+"/claim-dialects/local/claims", body, nil); err != nil {
		return false, err
	}
	return true, nil
}

// ensureDialectClaim maps an external claim onto a local one, so the attribute
// is reachable through that dialect's API — SCIM2 /Users, in the CDS case.
func (api *isAPI) ensureDialectClaim(dialectURI, claimURI, localClaimURI string) (bool, error) {

	dialectID, err := api.dialectID(dialectURI)
	if err != nil {
		return false, err
	}
	if dialectID == "" {
		return false, fmt.Errorf("the Identity Server has no %s claim dialect", dialectURI)
	}

	var claims []claim
	if err := api.call(http.MethodGet, mgmtAPI+"/claim-dialects/"+dialectID+"/claims", nil, &claims); err != nil {
		return false, err
	}
	for _, existing := range claims {
		if existing.ClaimURI == claimURI {
			return false, nil
		}
	}

	body := map[string]string{"claimURI": claimURI, "mappedLocalClaimURI": localClaimURI}
	if err := api.call(http.MethodPost, mgmtAPI+"/claim-dialects/"+dialectID+"/claims", body, nil); err != nil {
		return false, err
	}
	return true, nil
}

func (api *isAPI) dialectID(dialectURI string) (string, error) {

	// The endpoint answers with a bare array on some releases and with a
	// paged object on others, so both shapes are accepted.
	raw := json.RawMessage{}
	if err := api.call(http.MethodGet, mgmtAPI+"/claim-dialects", nil, &raw); err != nil {
		return "", err
	}

	type dialect struct {
		ID         string `json:"id"`
		DialectURI string `json:"dialectURI"`
	}
	var dialects []dialect
	if err := json.Unmarshal(raw, &dialects); err != nil {
		var paged struct {
			Dialects []dialect `json:"dialects"`
		}
		if err := json.Unmarshal(raw, &paged); err != nil {
			return "", fmt.Errorf("could not read the claim dialects: %w", err)
		}
		dialects = paged.Dialects
	}

	for _, candidate := range dialects {
		if candidate.DialectURI == dialectURI {
			return candidate.ID, nil
		}
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Roles
// ---------------------------------------------------------------------------

// grantRolePermissions adds the permissions a role does not already hold,
// returning how many were added. The administrator role needs the CDS scopes
// for the Console pages to show data rather than an empty list.
func (api *isAPI) grantRolePermissions(roleName string, permissions []string) (int, error) {

	var page struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"Resources"`
	}
	endpoint := "/scim2/v2/Roles?filter=" + url.QueryEscape("displayName eq "+roleName)
	if err := api.call(http.MethodGet, endpoint, nil, &page); err != nil {
		return 0, err
	}
	if len(page.Resources) == 0 {
		return 0, fmt.Errorf("the Identity Server has no %q role", roleName)
	}
	roleID := page.Resources[0].ID

	var role struct {
		Permissions []struct {
			Value string `json:"value"`
		} `json:"permissions"`
	}
	if err := api.call(http.MethodGet, "/scim2/v2/Roles/"+roleID, nil, &role); err != nil {
		return 0, err
	}
	held := make(map[string]bool, len(role.Permissions))
	for _, permission := range role.Permissions {
		held[permission.Value] = true
	}

	missing := make([]map[string]string, 0, len(permissions))
	for _, permission := range permissions {
		if !held[permission] {
			missing = append(missing, map[string]string{"value": permission})
		}
	}
	if len(missing) == 0 {
		return 0, nil
	}

	body := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{
			{"op": "add", "path": "permissions", "value": missing},
		},
	}
	if err := api.call(http.MethodPatch, "/scim2/v2/Roles/"+roleID, body, nil); err != nil {
		return 0, err
	}
	return len(missing), nil
}
