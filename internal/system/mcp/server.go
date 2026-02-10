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
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	// MCP tools you wrote
	profileTools "github.com/wso2/identity-customer-data-service/internal/system/mcp/tools/profile"

	// CDS service interface
	profileSvc "github.com/wso2/identity-customer-data-service/internal/profile/service"
)

// server holds dependencies for MCP tool registration.
type server struct {
	profilesService profileSvc.ProfilesServiceInterface

	// cached MCP server instance
	mcp *mcpsdk.Server
}

func newServer(profilesService profileSvc.ProfilesServiceInterface) *server {
	return &server{
		profilesService: profilesService,
	}
}

// getMCPServer builds (once) and returns the MCP server with CDS tools registered.
func (s *server) getMCPServer() *mcpsdk.Server {
	if s.mcp != nil {
		return s.mcp
	}

	// NOTE:
	// The go-sdk has had API changes. In many versions this works:
	//   mcpsdk.NewServer(mcpsdk.WithName(...), mcpsdk.WithVersion(...))
	//
	// If your IDE shows a different signature, adjust the constructor accordingly
	// (but the rest of the file stays the same).
	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "thunder-mcp",
		Version: "1.0.0",
	}, nil)

	// Register CDS Profile tools (cds_search_profiles, cds_get_profile, cds_patch_profile)
	pt := profileTools.NewTools(s.profilesService)
	pt.RegisterTools(mcpServer)

	s.mcp = mcpServer
	return s.mcp
}
