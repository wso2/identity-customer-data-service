// tools_models.go
package profile

import profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"

// cds_search_profiles
type SearchProfilesInput struct {
	// Supports either:
	// 1) Space format (service expects):
	//    "application_data.<appId>.preferences eq hi wage"
	// 2) Plus format (agent friendly):
	//    "application_data.<appId>.preferences+eq+hi wage"
	//
	// We normalize "+" -> " " before passing to service.
	Filter string `json:"filter,omitempty"`

	// Opaque cursor token (same token your REST API accepts/returns)
	Cursor string `json:"cursor,omitempty"`

	// Defaults to 50 if omitted.
	PageSize int `json:"page_size,omitempty"`

	// Required (service list/filter requires orgHandle)
	OrgHandle string `json:"org_handle"`

	// Reserved for future projection (ignore for now if not supported)
	Attributes []string `json:"attributes,omitempty"`
}

type SearchProfilesOutput struct {
	Profiles   []profileModel.ProfileResponse `json:"profiles"`
	HasMore    bool                           `json:"has_more"`
	NextCursor string                         `json:"next_cursor,omitempty"`
}

// cds_get_profile
type GetProfileInput struct {
	ProfileID string `json:"profile_id"`
}

type GetProfileOutput struct {
	Profile *profileModel.ProfileResponse `json:"profile"`
}

// cds_patch_profile
type PatchProfileInput struct {
	ProfileID string                 `json:"profile_id"`
	OrgHandle string                 `json:"org_handle"`
	Patch     map[string]interface{} `json:"patch"`
}
