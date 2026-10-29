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
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

// metadataPathPrefix is where OAuth protected resource metadata (RFC 9728) is served. The
// metadata for /t/{org}/cds/mcp is at /.well-known/oauth-protected-resource/t/{org}/cds/mcp.
const metadataPathPrefix = "/.well-known/oauth-protected-resource"

const clientIDKey = "client_id"

// requireToken rejects a request without a valid bearer token for its org. The 401 names
// metadataURL so that an MCP client can discover where to obtain a token.
func (s *server) requireToken(metadataURL string) func(http.Handler) http.Handler {

	return auth.RequireBearerToken(s.verifyToken, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
		// The token has been checked against IS by the time it gets here, so an
		// expiry the claims leave out is not a reason to refuse it.
		AllowMissingExpiration: true,
	})
}

func (s *server) verifyToken(_ context.Context, token string, r *http.Request) (*auth.TokenInfo, error) {

	claims, err := s.validateToken(token, utils.ExtractOrgHandleFromPath(r))
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	scope, _ := claims["scope"].(string)
	info := &auth.TokenInfo{
		Scopes: strings.Fields(scope),
		Extra:  map[string]any{clientIDKey: clientIDFromClaims(claims)},
	}
	if exp, ok := claims["exp"].(float64); ok {
		info.Expiration = time.Unix(int64(exp), 0)
	}
	return info, nil
}

// tokenInfo returns what verifyToken found for the request.
func tokenInfo(r *http.Request) *auth.TokenInfo {

	if info := auth.TokenInfoFromContext(r.Context()); info != nil {
		return info
	}
	return &auth.TokenInfo{}
}

func clientIDOf(info *auth.TokenInfo) string {

	clientID, _ := info.Extra[clientIDKey].(string)
	return clientID
}

func clientIDFromClaims(claims map[string]interface{}) string {

	if azp, ok := claims[constants.AZPClaim].(string); ok && azp != "" {
		return azp
	}
	clientID, _ := claims[constants.ClientIdClaim].(string)
	return clientID
}

// resourceMetadataURL returns the metadata URL for the endpoint r was sent to.
func resourceMetadataURL(r *http.Request) string {

	resourcePath := pathPrefix(r) + utils.ExtractOrgHandleFromPath(r) + Path
	return publicBaseURL(r) + metadataPathPrefix + resourcePath
}

// serveMetadata answers /.well-known/oauth-protected-resource/{t|o}/{org}/cds/mcp.
func (s *server) serveMetadata(w http.ResponseWriter, r *http.Request) {

	resourcePath := strings.TrimPrefix(r.URL.Path, metadataPathPrefix)
	orgHandle, ok := orgOfResourcePath(resourcePath)
	if !ok {
		http.NotFound(w, r)
		return
	}

	auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               publicBaseURL(r) + resourcePath,
		AuthorizationServers:   []string{authorizationServer(orgHandle)},
		ScopesSupported:        s.scopesSupported(),
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "WSO2 Customer Data Service",
	}).ServeHTTP(w, r)
}

// orgOfResourcePath returns the org of "/t/{org}/cds/mcp" or "/o/{org}/cds/mcp".
func orgOfResourcePath(resourcePath string) (string, bool) {

	for _, prefix := range []string{"/t/", "/o/"} {
		rest, found := strings.CutPrefix(resourcePath, prefix)
		if !found {
			continue
		}
		orgHandle, found := strings.CutSuffix(rest, Path)
		if found && orgHandle != "" && !strings.Contains(orgHandle, "/") {
			return orgHandle, true
		}
	}
	return "", false
}

// authorizationServer returns the issuer of tokens for the org, built the way the identity
// client builds the token endpoint: sub-organizations, named by UUID, sit below carbon.super.
func authorizationServer(orgHandle string) string {

	authServer := config.GetCDSRuntime().Config.AuthServer
	host := authServer.Host
	if authServer.Port != "" {
		host += ":" + authServer.Port
	}
	if _, err := uuid.Parse(orgHandle); err == nil {
		return fmt.Sprintf("https://%s/t/carbon.super/o/%s/oauth2/token", host, orgHandle)
	}
	return fmt.Sprintf("https://%s/t/%s/oauth2/token", host, orgHandle)
}

// scopesSupported lists the scopes that unlock the tools and resources this server offers.
func (s *server) scopesSupported() []string {

	required := config.GetCDSRuntime().Config.AuthServer.RequiredScopes
	var scopes []string
	for _, operation := range operations(s.readOnly) {
		for _, scope := range required[operation] {
			if !slices.Contains(scopes, scope) {
				scopes = append(scopes, scope)
			}
		}
	}
	return scopes
}

// publicBaseURL is the configured server_url, or failing that the host the request was sent to.
func publicBaseURL(r *http.Request) string {

	if serverURL := strings.TrimSuffix(config.GetCDSRuntime().Config.ServerURL, "/"); serverURL != "" {
		return serverURL
	}
	return "https://" + r.Host
}
