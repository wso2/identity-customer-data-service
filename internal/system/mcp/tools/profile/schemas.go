// tools_schemas.go
package profile

import "github.com/google/jsonschema-go/jsonschema"

// NOTE: We intentionally avoid jsonschema.False()/True()/Num() helpers
// because they are not available in github.com/google/jsonschema-go/jsonschema.

var searchProfilesInputSchema = &jsonschema.Schema{
	Type: "object",
	Required: []string{
		"org_handle",
	},
	Properties: map[string]*jsonschema.Schema{
		"filter": {
			Type:        "string",
			Description: "Filter in either 'field op value' or 'field+op+value' format. Example: application_data.<appId>.preferences+eq+hi wage",
		},
		"cursor": {
			Type:        "string",
			Description: "Opaque cursor token for pagination.",
		},
		"page_size": {
			Type:        "integer",
			Description: "Max results per page (defaults to 50).",
		},
		"org_handle": {
			Type:        "string",
			Description: "Organization handle (tenant). Required.",
		},
		"attributes": {
			Type:        "array",
			Description: "Optional projection list. Can be ignored if not supported.",
			Items:       &jsonschema.Schema{Type: "string"},
		},
	},
}

var getProfileInputSchema = &jsonschema.Schema{
	Type: "object",
	Required: []string{
		"profile_id",
	},
	Properties: map[string]*jsonschema.Schema{
		"profile_id": {
			Type:        "string",
			Description: "Unique profile identifier.",
		},
	},
}

var patchProfileInputSchema = &jsonschema.Schema{
	Type: "object",
	Required: []string{
		"profile_id",
		"org_handle",
		"patch",
	},
	Properties: map[string]*jsonschema.Schema{
		"profile_id": {
			Type:        "string",
			Description: "Unique profile identifier.",
		},
		"org_handle": {
			Type:        "string",
			Description: "Organization handle (tenant). Required.",
		},
		"patch": {
			Type:        "object",
			Description: "Partial update payload. Example: {\"traits\": {\"segment\": \"vip\"}}",
		},
	},
}
