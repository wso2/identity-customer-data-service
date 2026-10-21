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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// cdsClient calls the CDS APIs as the client application provisioning created.
type cdsClient struct {
	base         string
	organization string
	token        string
}

// cdsToken obtains a client-credentials token for the CDS APIs from the
// Identity Server.
//
// The scopes are asked for explicitly rather than taken from the application's
// full grant, so a token that is missing one of them fails here, where the
// cause is obvious, rather than at the call that needed it.
func cdsToken(isBase, clientID, clientSecret string, scopes []string, trustPEM string, insecure bool) (string, error) {

	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("no CDS client credentials recorded. Run `wso2cds enable` to provision them")
	}

	api, err := newISAPI(isBase, clientID, clientSecret, trustPEM, insecure)
	if err != nil {
		return "", err
	}

	form := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {strings.Join(scopes, " ")},
	}
	request, err := http.NewRequest(http.MethodPost, api.base+"/oauth2/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("could not build the token request: %w", err)
	}
	request.SetBasicAuth(clientID, clientSecret)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := api.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("could not reach the token endpoint at %s: %w", isBase, err)
	}
	defer func() { _ = response.Body.Close() }()

	var issued struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.NewDecoder(response.Body).Decode(&issued); err != nil {
		return "", fmt.Errorf("could not read the token response: %w", err)
	}
	if issued.AccessToken == "" {
		return "", fmt.Errorf("the Identity Server issued no token: %s %s", issued.Error, issued.Description)
	}
	return issued.AccessToken, nil
}

// call performs one request against the organization-scoped CDS API.
func (c *cdsClient) call(method, endpoint string, body, out any) (int, error) {

	var payload *strings.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("could not encode the request for %s: %w", endpoint, err)
		}
		payload = strings.NewReader(string(encoded))
	}

	target := fmt.Sprintf("%s/t/%s/cds/api/v1%s", c.base, c.organization, endpoint)
	var request *http.Request
	var err error
	if payload != nil {
		request, err = http.NewRequest(method, target, payload)
	} else {
		request, err = http.NewRequest(method, target, nil)
	}
	if err != nil {
		return 0, fmt.Errorf("could not build the request for %s: %w", endpoint, err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := localClient(2 * time.Minute).Do(request)
	if err != nil {
		return 0, fmt.Errorf("could not reach CDS at %s: %w", c.base, err)
	}
	defer func() { _ = response.Body.Close() }()

	if out != nil {
		_ = json.NewDecoder(response.Body).Decode(out)
	}
	return response.StatusCode, nil
}

// enableOrganization turns CDS on for one organization, which is also what
// runs the first profile-schema synchronisation and seeds the default consent
// categories. Every profile, schema and rule endpoint refuses requests until
// it has happened.
//
// It is idempotent: an organization that is already enabled is left alone.
func enableOrganization(client *cdsClient, systemApplications []string) (bool, error) {

	var current struct {
		Enabled bool `json:"cds_enabled"`
	}
	status, err := client.call(http.MethodGet, "/config", nil, &current)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("reading the CDS configuration of %q returned HTTP %d",
			client.organization, status)
	}
	if current.Enabled {
		return false, nil
	}

	// Admin surfaces see unfiltered profiles; every other application gets
	// consent-filtered data, so the Console and the provisioned client are
	// named here and nothing else is.
	body := map[string]any{
		"cds_enabled":         true,
		"system_applications": systemApplications,
	}
	status, err = client.call(http.MethodPatch, "/config", body, nil)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return false, fmt.Errorf("enabling CDS for %q returned HTTP %d.\n"+
			"This call runs the initial schema synchronisation, so it fails when CDS "+
			"cannot reach the Identity Server claim APIs — see %s",
			client.organization, status, runtimeLog)
	}
	return true, nil
}
