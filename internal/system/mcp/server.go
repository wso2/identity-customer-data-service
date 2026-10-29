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

// Package mcp serves the Customer Data Service to AI agents over the Model Context Protocol.
//
// Every tool and resource is answered by the REST handler for the same operation, called
// in process with the agent's own token. An agent therefore gets exactly what a REST
// caller with that token gets: the same scope checks, the same check that CDS is enabled
// for the org, and the same filtering of application data and consented fields.
package mcp

import (
	"net/http"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/identity-customer-data-service/internal/system/authn"
	"github.com/wso2/identity-customer-data-service/internal/system/authz"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

// Path is where the endpoint sits below /t/{org} and /o/{org}.
const Path = "/cds/mcp"

const serverName = "wso2-customer-data-service"

// server builds a fresh MCP server for every request, from that request's org and token.
type server struct {
	// routes serves the REST API paths, below the /t/{org} or /o/{org} prefix.
	routes http.Handler
	// validateToken checks a bearer token for an org and returns its claims.
	validateToken func(token, orgHandle string) (map[string]interface{}, error)
	readOnly      bool
}

// Register mounts the endpoint on routes, the per-org mux the REST API is served from,
// and its OAuth protected resource metadata on root.
func Register(root, routes *http.ServeMux, readOnly bool) {

	s := &server{
		routes:        routes,
		validateToken: authn.ValidateAuthenticationAndReturnClaims,
		readOnly:      readOnly,
	}
	routes.Handle(Path, s.endpoint())
	root.HandleFunc(metadataPathPrefix+"/", s.serveMetadata)
}

func (s *server) endpoint() http.Handler {

	streamable := mcpsdk.NewStreamableHTTPHandler(s.serverFor, &mcpsdk.StreamableHTTPOptions{
		// No sessions: each request is served by a server built from its own org and
		// token, so nothing granted to one request can carry over into another.
		Stateless:    true,
		JSONResponse: true,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireToken := s.requireToken(resourceMetadataURL(r))
		requireToken(streamable).ServeHTTP(w, r)
	})
}

// serverFor returns the server for one request, holding only the tools and resources the
// request's token is allowed to use.
func (s *server) serverFor(r *http.Request) *mcpsdk.Server {

	token := tokenInfo(r)
	granted := strings.Join(token.Scopes, " ")
	allowed := func(operation string) bool {
		return authz.ValidatePermission(granted, operation)
	}

	c := &restClient{
		routes:        s.routes,
		orgHandle:     utils.ExtractOrgHandleFromPath(r),
		pathPrefix:    pathPrefix(r),
		authorization: r.Header.Get("Authorization"),
		clientID:      clientIDOf(token),
	}

	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:  serverName,
		Title: "WSO2 Customer Data Service",
	}, nil)
	addProfileTools(srv, c, allowed, s.readOnly)
	addResources(srv, c, allowed)
	return srv
}

// pathPrefix returns "/t/" or "/o/", whichever the request came in under.
func pathPrefix(r *http.Request) string {

	if prefix, ok := r.Context().Value(constants.PathPrefixContextKey).(string); ok && prefix != "" {
		return prefix
	}
	return "/t/"
}
