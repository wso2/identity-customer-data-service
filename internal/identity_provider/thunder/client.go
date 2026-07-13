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

// Package thunder implements identity_provider.Client against WSO2 Thunder
// (github.com/thunder-id/thunderid). Thunder is a pre-1.0, single-tenant
// identity core: it supports client_credentials token issuance, RFC 7662
// introspection and JWKS-based JWT verification, but has no claim-dialect /
// SCIM model (see SchemaSyncCapable — deliberately not implemented here) and
// no OAuth2 client_id isolation between "Organization Units". See
// docs/guides/identity-providers.md for the full list of gaps and the
// operational caveats this implies.
package thunder

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/identity-customer-data-service/internal/identity_provider"
	idpModel "github.com/wso2/identity-customer-data-service/internal/identity_provider/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

// applicationsPageLimit and applicationsPageMax bound the worst-case latency
// of FetchApplicationIdentifier's client-side pagination scan (Thunder's
// /applications API has no filter-by-clientId support).
const (
	applicationsPageLimit = 50
	applicationsPageMax   = 20 // i.e. at most 1000 applications scanned
)

// Client is CDS's identity_provider.Client implementation for WSO2 Thunder.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	cfg        config.ThunderConfig
}

// NewClient creates a Client with a TLS/mTLS-ready HTTP client, reusing the
// same trust store / mTLS posture as the wso2is client (cfg.TLS).
func NewClient(cfg config.Config) *Client {
	thunderCfg := cfg.AuthServer.Thunder
	baseHostPort := thunderCfg.Host
	if thunderCfg.Port != "" {
		baseHostPort = thunderCfg.Host + ":" + thunderCfg.Port
	}
	log.GetLogger().Info("Creating thunder identity provider client with base URL: " + baseHostPort)

	httpClient, err := utils.NewOutboundHTTPClient(cfg.TLS, thunderCfg.Host)
	if err != nil {
		log.GetLogger().Error("Failed to create outbound HTTPS client for Thunder", log.Error(err))
		os.Exit(1)
	}

	return &Client{
		BaseURL:    baseHostPort,
		HTTPClient: httpClient,
		cfg:        thunderCfg,
	}
}

// Name identifies this provider for logging/diagnostics.
func (c *Client) Name() string {
	return constants.IdentityProviderThunder
}

// FetchToken obtains an access token via client_credentials. Thunder has no
// system_app_grant/org-token-exchange equivalent, so orgHandle is accepted
// for interface parity but not sent to Thunder — see design note in
// docs/guides/identity-providers.md on the lack of tenant isolation.
func (c *Client) FetchToken(_ string) (string, error) {
	endpoint := fmt.Sprintf("https://%s%s", c.BaseURL, c.cfg.TokenEndpoint)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: "Failed to create Thunder token request",
		}, err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	logger := log.GetLogger()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errMsg := "Failed to fetch token from Thunder"
		logger.Debug(errMsg, log.Error(err))
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errMsg,
		}, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Thunder token endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		logger.Debug(errMsg)
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errMsg,
		}, fmt.Errorf("token endpoint non-200: %d", resp.StatusCode))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" {
		errMsg := "Failed to parse Thunder token response"
		logger.Debug(errMsg, log.Error(err))
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errMsg,
		}, err)
	}

	return result.AccessToken, nil
}

// IntrospectToken introspects a token against Thunder's single, global
// /oauth2/introspect endpoint (RFC 7662). orgHandle is accepted for
// interface parity but unused — Thunder has no tenant-scoped introspection.
func (c *Client) IntrospectToken(token, _ string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s%s", c.BaseURL, c.cfg.IntrospectionEndpoint)

	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	logger := log.GetLogger()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errMsg := "Failed to introspect token against Thunder"
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_INTROSPECTION_FAILED",
			Message:     "Unable to introspect access token",
			Description: errMsg,
		}, http.StatusUnauthorized)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Thunder introspection endpoint returned status %d: %s", resp.StatusCode, string(body))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_INTROSPECTION_INVALID_RESPONSE",
			Message:     "Token introspection failed",
			Description: errMsg,
		}, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// thunderApplication mirrors the subset of Thunder's application resource
// (api/application.yaml) that CDS needs.
type thunderApplication struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ClientId string `json:"clientId"`
	OuId     string `json:"ouId"`
}

type thunderApplicationsPage struct {
	TotalResults int                  `json:"totalResults"`
	Applications []thunderApplication `json:"applications"`
}

// FetchApplicationIdentifier looks up an application by clientId. Thunder's
// /applications API has no server-side filter, only limit/offset pagination,
// so this pages through results and matches clientId client-side (capped at
// applicationsPageMax pages). Unlike wso2is, Thunder exposes no "issuer"
// field, so issuer-based matches are not supported here — see
// docs/guides/identity-providers.md.
func (c *Client) FetchApplicationIdentifier(applicationIdentifier, _ string) (idpModel.ApplicationsListResponse, error) {
	var result idpModel.ApplicationsListResponse
	logger := log.GetLogger()

	token, err := c.FetchToken("")
	if err != nil {
		logger.Debug("Failed to get token for Thunder application lookup", log.Error(err))
		return result, err
	}

	for page := 0; page < applicationsPageMax; page++ {
		offset := page * applicationsPageLimit
		endpoint := fmt.Sprintf("https://%s%s?limit=%d&offset=%d", c.BaseURL, c.cfg.ApplicationsEndpoint, applicationsPageLimit, offset)

		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return result, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			errMsg := "Failed to fetch applications from Thunder"
			return result, errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_APPLICATIONS_FAILED.Code,
				Message:     errors2.GET_APPLICATIONS_FAILED.Message,
				Description: errMsg,
			}, err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errMsg := fmt.Sprintf("Thunder applications endpoint returned status %d: %s", resp.StatusCode, string(body))
			return result, errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_APPLICATIONS_FAILED.Code,
				Message:     errors2.GET_APPLICATIONS_FAILED.Message,
				Description: errMsg,
			}, fmt.Errorf("applications endpoint returned status: %d", resp.StatusCode))
		}
		if readErr != nil {
			return result, readErr
		}

		var pageResult thunderApplicationsPage
		if err := json.Unmarshal(body, &pageResult); err != nil {
			errMsg := "Failed to parse Thunder applications response"
			logger.Debug(errMsg, log.Error(err))
			return result, errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_APPLICATIONS_FAILED.Code,
				Message:     errors2.GET_APPLICATIONS_FAILED.Message,
				Description: errMsg,
			}, err)
		}

		for _, app := range pageResult.Applications {
			if app.ClientId == applicationIdentifier {
				result.Applications = append(result.Applications, idpModel.ApplicationItem{
					ID:       app.ID,
					Name:     app.Name,
					ClientId: app.ClientId,
				})
			}
		}
		result.TotalResults = len(result.Applications)
		result.Count = len(result.Applications)

		if len(pageResult.Applications) < applicationsPageLimit || len(result.Applications) > 0 {
			// Either we've exhausted the list, or we already found our match(es).
			break
		}
	}

	return result, nil
}

// VerifyJWT locally verifies a JWT's signature against Thunder's JWKS
// endpoint. Not yet wired into internal/system/authn — see
// docs/guides/identity-providers.md, Phase 2.
func (c *Client) VerifyJWT(tokenString string) (map[string]interface{}, error) {
	unverified, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token header: %w", err)
	}
	kid, _ := unverified.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("token header does not contain a kid")
	}

	jwksEndpoint := fmt.Sprintf("https://%s%s", c.BaseURL, c.cfg.JWKSEndpoint)
	req, err := http.NewRequest(http.MethodGet, jwksEndpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksEndpoint, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	pubKey, err := identity_provider.RSAPublicKeyFromJWKS(body, kid)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return pubKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, fmt.Errorf("JWT signature verification failed: %w", err)
	}

	return claims, nil
}
