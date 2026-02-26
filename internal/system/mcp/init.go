package mcp

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	// CDS services
	profileSvc "github.com/wso2/identity-customer-data-service/internal/profile/service"
)

// Initialize initializes the CDS MCP server and registers its HTTP routes with the provided mux.
//
// If you want auth like Thunder:
// wrap the handler here (auth.RequireBearerToken(...)(httpHandler)).
func Initialize(mux *http.ServeMux, profilesService profileSvc.ProfilesServiceInterface) {
	mcpServer := newServer(profilesService)

	// Streamable MCP over HTTP
	httpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return mcpServer.getMCPServer()
	}, nil)

	// Register MCP routes
	mux.Handle(MCPEndpointPath, httpHandler)
	mux.Handle(MCPEndpointPath+"/", httpHandler)
}
