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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/idp/credential"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/systemapp"
)

// ProvisionSystemApplication makes sure orgHandle has an Identity Server
// application CDS can call the management APIs with, and records its credentials.
//
// It is idempotent in both directions: an organisation whose credentials are
// already recorded is left alone, and an application that exists in the Identity
// Server but is not recorded - a restored database, an earlier run that stopped
// half way - is adopted rather than duplicated.
//
// A deployment that has its applications provisioned for it does nothing here and
// keeps using the configured pair.
//
// permissions is what the application has to be able to do, in CDS's terms. They are
// translated to the API resources and scopes the Identity Server grants them with, so
// the application ends up able to do what CDS asks for and nothing besides - an
// application that can do more than CDS needs is a larger credential to lose.
func (c *IdentityClient) ProvisionSystemApplication(ctx context.Context, orgHandle string,
	permissions []credential.Permission) error {

	logger := log.GetLogger()
	authCfg := config.GetCDSRuntime().Config.AuthServer
	if !authCfg.SystemApplication.AutoProvision {
		return nil
	}

	_, found, err := systemapp.Get(ctx, orgHandle)
	if err != nil {
		return err
	}
	if found {
		logger.Debug(fmt.Sprintf("System application already recorded for organization: %s", orgHandle))
		return nil
	}

	name := authCfg.SystemApplication.SystemApplicationName(orgHandle)

	appID, err := c.findApplicationByName(name, orgHandle)
	if err != nil {
		return err
	}
	if appID == "" {
		appID, err = c.createSystemApplication(name, orgHandle)
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("Created system application '%s' for organization: %s", name, orgHandle))
	} else {
		logger.Info(fmt.Sprintf("Adopting existing system application '%s' of organization: %s", name, orgHandle))
	}

	for _, permission := range permissions {
		granted, known := wso2Permissions[permission]
		if !known {
			return provisioningFailed(orgHandle, fmt.Sprintf(
				"the Identity Server has no API resource that grants '%s'", permission), nil)
		}
		if err := c.authorizeSystemAPI(appID, orgHandle, granted.APIResource, granted.Scopes); err != nil {
			return err
		}
	}

	clientID, clientSecret, err := c.readOIDCCredentials(appID, orgHandle)
	if err != nil {
		return err
	}

	return systemapp.Save(ctx, systemapp.Credentials{
		OrgHandle:    orgHandle,
		AppID:        appID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
}

// findApplicationByName returns the application resource ID, or an empty string
// when the organisation has no application by that name.
func (c *IdentityClient) findApplicationByName(name, orgHandle string) (string, error) {

	query := url.Values{"filter": {"name eq " + name}}
	body, err := c.managementRequest(http.MethodGet,
		"/api/server/v1/applications?"+query.Encode(), orgHandle, nil)
	if err != nil {
		return "", provisioningFailed(orgHandle, fmt.Sprintf("failed to look up application '%s'", name), err)
	}

	var listed struct {
		Applications []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"applications"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return "", provisioningFailed(orgHandle, "failed to parse the applications response", err)
	}

	// The filter is a search, so confirm the name rather than taking the first hit.
	for _, app := range listed.Applications {
		if app.Name == name {
			return app.ID, nil
		}
	}
	return "", nil
}

// createSystemApplication registers the machine-to-machine application. The
// Identity Server answers a create with a Location header rather than the
// resource, so the ID is read back by name.
func (c *IdentityClient) createSystemApplication(name, orgHandle string) (string, error) {

	payload := map[string]interface{}{
		"name": name,
		"description": "Created by the Customer Data Service to call the Identity Server " +
			"management APIs on behalf of this organization.",
		"isManagementApp": true,
		"inboundProtocolConfiguration": map[string]interface{}{
			"oidc": map[string]interface{}{
				"grantTypes":  []string{"client_credentials"},
				"accessToken": map[string]interface{}{"type": "JWT"},
			},
		},
	}

	if _, err := c.managementRequest(http.MethodPost,
		"/api/server/v1/applications", orgHandle, payload); err != nil {
		return "", provisioningFailed(orgHandle, fmt.Sprintf("failed to create application '%s'", name), err)
	}

	appID, err := c.findApplicationByName(name, orgHandle)
	if err != nil {
		return "", err
	}
	if appID == "" {
		return "", provisioningFailed(orgHandle,
			fmt.Sprintf("application '%s' was created but could not be read back", name), nil)
	}
	return appID, nil
}

// authorizeSystemAPI subscribes the application to one API resource. An
// application that already carries the resource is patched rather than
// re-subscribed, so that re-running provisioning adds scopes instead of failing.
func (c *IdentityClient) authorizeSystemAPI(appID, orgHandle, identifier string, scopes []string) error {

	resourceID, err := c.apiResourceID(identifier, orgHandle)
	if err != nil {
		return err
	}
	if resourceID == "" {
		return provisioningFailed(orgHandle, fmt.Sprintf(
			"the Identity Server has no API resource '%s', so the system application cannot be "+
				"subscribed to it", identifier), nil)
	}

	authorized, err := c.managementRequest(http.MethodGet,
		"/api/server/v1/applications/"+appID+"/authorized-apis", orgHandle, nil)
	if err != nil {
		return provisioningFailed(orgHandle, "failed to read the authorized APIs of the system application", err)
	}

	var existing []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(authorized, &existing); err != nil {
		return provisioningFailed(orgHandle, "failed to parse the authorized APIs response", err)
	}

	subscribed := false
	for _, api := range existing {
		if api.ID == resourceID {
			subscribed = true
			break
		}
	}

	if subscribed {
		_, err = c.managementRequest(http.MethodPatch,
			"/api/server/v1/applications/"+appID+"/authorized-apis/"+resourceID, orgHandle,
			map[string]interface{}{"addedScopes": scopes, "removedScopes": []string{}})
	} else {
		_, err = c.managementRequest(http.MethodPost,
			"/api/server/v1/applications/"+appID+"/authorized-apis", orgHandle,
			map[string]interface{}{"id": resourceID, "policyIdentifier": "RBAC", "scopes": scopes})
	}
	if err != nil {
		return provisioningFailed(orgHandle,
			fmt.Sprintf("failed to subscribe the system application to '%s'", identifier), err)
	}
	return nil
}

// apiResourceID resolves an API resource identifier to its resource ID, or an
// empty string when the Identity Server does not carry it.
func (c *IdentityClient) apiResourceID(identifier, orgHandle string) (string, error) {

	query := url.Values{"filter": {"identifier eq " + identifier}}
	body, err := c.managementRequest(http.MethodGet,
		"/api/server/v1/api-resources?"+query.Encode(), orgHandle, nil)
	if err != nil {
		return "", provisioningFailed(orgHandle,
			fmt.Sprintf("failed to resolve the API resource '%s'", identifier), err)
	}

	var listed struct {
		APIResources []struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
		} `json:"apiResources"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return "", provisioningFailed(orgHandle, "failed to parse the API resources response", err)
	}

	for _, resource := range listed.APIResources {
		if resource.Identifier == identifier {
			return resource.ID, nil
		}
	}
	return "", nil
}

// readOIDCCredentials reads the credentials the Identity Server issued the
// application. They are read back rather than chosen by CDS, so that an adopted
// application yields the credentials it already has.
func (c *IdentityClient) readOIDCCredentials(appID, orgHandle string) (string, string, error) {

	body, err := c.managementRequest(http.MethodGet,
		"/api/server/v1/applications/"+appID+"/inbound-protocols/oidc", orgHandle, nil)
	if err != nil {
		return "", "", provisioningFailed(orgHandle,
			"failed to read the OIDC configuration of the system application", err)
	}

	var oidc struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.Unmarshal(body, &oidc); err != nil {
		return "", "", provisioningFailed(orgHandle, "failed to parse the OIDC configuration", err)
	}
	if oidc.ClientID == "" || oidc.ClientSecret == "" {
		return "", "", provisioningFailed(orgHandle,
			"the system application carries no client credentials", nil)
	}
	return oidc.ClientID, oidc.ClientSecret, nil
}

// managementRequest calls an Identity Server management API as the configured
// administrator, and returns the response body of a 2xx.
//
// Every other call in this package authenticates with a token the organisation's
// own application issues. Provisioning cannot: until it has run, the organisation
// has no such application. It uses the administrator credentials instead - the one
// credential the deployment already hands CDS - which is why provisioning stays
// off unless a deployment turns it on.
func (c *IdentityClient) managementRequest(method, path, orgHandle string, payload interface{}) ([]byte, error) {

	authCfg := config.GetCDSRuntime().Config.AuthServer

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode the request body: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	endpoint := fmt.Sprintf("https://%s/t/%s%s", c.BaseURL, orgHandle, path)
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create the request: %w", err)
	}
	req.SetBasicAuth(authCfg.AdminUsername, authCfg.AdminPassword)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w", method, redactQuery(endpoint), err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read the response of %s %s: %w", method, redactQuery(endpoint), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s %s returned %d: %s",
			method, redactQuery(endpoint), resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

// redactQuery drops the query string from an endpoint before it reaches an error
// or a log line. The filters carry organisation and application names, and the
// path alone is what identifies the call that failed.
func redactQuery(endpoint string) string {

	if index := strings.IndexByte(endpoint, '?'); index >= 0 {
		return endpoint[:index]
	}
	return endpoint
}

func provisioningFailed(orgHandle, what string, err error) error {

	errorMsg := fmt.Sprintf("%s for the organization: %s", what, orgHandle)
	log.GetLogger().Debug(errorMsg, log.Error(err))
	return errors2.NewServerError(errors2.ErrorMessage{
		Code:        errors2.PROVISION_SYSTEM_APPLICATION_FAILED.Code,
		Message:     errors2.PROVISION_SYSTEM_APPLICATION_FAILED.Message,
		Description: errorMsg,
	}, err)
}
