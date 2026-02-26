// tools_handlers.go
package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileSvc "github.com/wso2/identity-customer-data-service/internal/profile/service"
)

type Tools struct {
	profiles profileSvc.ProfilesServiceInterface
}

func NewTools(profiles profileSvc.ProfilesServiceInterface) *Tools {
	return &Tools{profiles: profiles}
}

func (t *Tools) RegisterTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cds_search_profiles",
		Description: "Search/filter CDS profiles with cursor pagination.",
		InputSchema: searchProfilesInputSchema,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Search Profiles",
			ReadOnlyHint: true,
		},
	}, t.searchProfiles)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cds_get_profile",
		Description: "Retrieve a single CDS profile by profile_id.",
		InputSchema: getProfileInputSchema,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Profile",
			ReadOnlyHint: true,
		},
	}, t.getProfile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cds_patch_profile",
		Description: "Apply partial updates to a CDS profile safely.",
		InputSchema: patchProfileInputSchema,
		Annotations: &mcp.ToolAnnotations{
			Title:          "Patch Profile",
			IdempotentHint: true,
		},
	}, t.patchProfile)
}

func (t *Tools) getProfile(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetProfileInput,
) (*mcp.CallToolResult, GetProfileOutput, error) {

	if strings.TrimSpace(input.ProfileID) == "" {
		return nil, GetProfileOutput{}, fmt.Errorf("profile_id is required")
	}

	p, err := t.profiles.GetProfile(input.ProfileID)
	if err != nil {
		return nil, GetProfileOutput{}, fmt.Errorf("failed to fetch profile: %w", err)
	}

	return nil, GetProfileOutput{Profile: p}, nil
}

func (t *Tools) patchProfile(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input PatchProfileInput,
) (*mcp.CallToolResult, *profileModel.ProfileResponse, error) {

	if strings.TrimSpace(input.ProfileID) == "" {
		return nil, nil, fmt.Errorf("profile_id is required")
	}
	if strings.TrimSpace(input.OrgHandle) == "" {
		return nil, nil, fmt.Errorf("org_handle is required")
	}
	if input.Patch == nil {
		return nil, nil, fmt.Errorf("patch is required")
	}

	updated, err := t.profiles.PatchProfile(input.ProfileID, input.OrgHandle, input.Patch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to patch profile: %w", err)
	}

	return nil, updated, nil
}

func (t *Tools) searchProfiles(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input SearchProfilesInput,
) (*mcp.CallToolResult, SearchProfilesOutput, error) {

	if strings.TrimSpace(input.OrgHandle) == "" {
		return nil, SearchProfilesOutput{}, fmt.Errorf("org_handle is required")
	}

	limit := input.PageSize
	if limit <= 0 {
		limit = 50
	}

	// TODO: Decode cursor string -> *profileModel.ProfileCursor using your existing REST cursor logic.
	var cursorObj *profileModel.ProfileCursor = nil

	filterStr := strings.TrimSpace(input.Filter)
	if filterStr != "" {
		filterStr = normalizeFilter(filterStr)
	}

	var (
		profiles []profileModel.ProfileResponse
		hasMore  bool
		err      error
	)

	if filterStr == "" {
		profiles, hasMore, err = t.profiles.GetAllProfilesCursor(input.OrgHandle, limit, cursorObj)
	} else {
		profiles, hasMore, err = t.profiles.GetAllProfilesWithFilterCursor(input.OrgHandle, []string{filterStr}, limit, cursorObj)
	}

	if err != nil {
		return nil, SearchProfilesOutput{}, fmt.Errorf("failed to search profiles: %w", err)
	}

	// TODO: Encode next cursor using the same logic as your REST list endpoint.
	nextCursor := ""

	return nil, SearchProfilesOutput{
		Profiles:   profiles,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// normalizeFilter converts "field+op+value" -> "field op value".
// Leaves already space-formatted filters unchanged.
func normalizeFilter(in string) string {
	in = strings.TrimSpace(in)
	parts := strings.SplitN(in, "+", 3)
	if len(parts) == 3 {
		field := strings.TrimSpace(parts[0])
		op := strings.TrimSpace(parts[1])
		val := strings.TrimSpace(parts[2])
		if field != "" && op != "" {
			return fmt.Sprintf("%s %s %s", field, op, val)
		}
	}
	return in
}
