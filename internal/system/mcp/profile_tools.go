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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// The REST operations whose scopes unlock the tools and resources.
const (
	opProfileView         = "profile:view"
	opProfileUpdate       = "profile:update"
	opProfileSchemaView   = "profile_schema:view"
	opConsentCategoryView = "consent_category:view"
)

// operations lists every operation the server can offer, leaving out updates when read only.
func operations(readOnly bool) []string {

	ops := []string{opProfileView, opProfileSchemaView, opConsentCategoryView}
	if !readOnly {
		ops = append(ops, opProfileUpdate)
	}
	return ops
}

// The attributes a profile lookup returns. The list endpoint returns only IDs and metadata
// unless asked for more, and an agent looking someone up wants to see who it found.
const lookupAttributes = "identity_attributes,traits"

type searchProfilesInput struct {
	Filters      []string `json:"filters,omitempty" jsonschema:"Exact conditions that must all hold, each written 'field operator value', for example 'identity_attributes.email eq jane@example.com'. Fields are user_id, profile_id, identity_attributes.<name>, traits.<name> and application_data.<app id>.<name>. Operators are eq, ne, co, sw and ew."`
	FuzzyFilters []string `json:"fuzzy_filters,omitempty" jsonschema:"Tolerant conditions, each written 'field eq value', that also match values with small differences such as typos. Results carry a match score and are not paginated."`
	Threshold    float64  `json:"threshold,omitempty" jsonschema:"Lowest match score to return for fuzzy_filters, between 0 and 1."`
	Attributes   []string `json:"attributes,omitempty" jsonschema:"Data to include for each profile, for example 'identity_attributes', 'traits.segment' or 'application_data.<app id>'. Only IDs and metadata are returned when omitted."`
	Cursor       string   `json:"cursor,omitempty" jsonschema:"next_cursor or previous_cursor from an earlier page."`
	PageSize     int      `json:"page_size,omitempty" jsonschema:"Profiles per page. The server caps it."`
}

type profileIDInput struct {
	ProfileID string `json:"profile_id" jsonschema:"ID of the profile."`
}

type findProfileInput struct {
	UserID    string `json:"user_id,omitempty" jsonschema:"ID of the customer's user account in the identity server."`
	Attribute string `json:"attribute,omitempty" jsonschema:"Attribute to match value against, for example 'identity_attributes.email' or 'identity_attributes.mobile'. Use with value instead of user_id."`
	Value     string `json:"value,omitempty" jsonschema:"Value of attribute to look for."`
	Fuzzy     bool   `json:"fuzzy,omitempty" jsonschema:"Also match values with small differences such as typos or formatting. Results carry a match score."`
}

type patchProfileInput struct {
	ProfileID       string         `json:"profile_id" jsonschema:"ID of the profile to update."`
	Traits          map[string]any `json:"traits,omitempty" jsonschema:"Traits to set, merged into the existing traits."`
	ApplicationData map[string]any `json:"application_data,omitempty" jsonschema:"Application data to set, keyed by application ID and merged into the existing data."`
}

// profileSummary is a profile without its application data or metadata.
type profileSummary struct {
	ProfileID          string          `json:"profile_id"`
	UserID             string          `json:"user_id,omitempty"`
	IdentityAttributes map[string]any  `json:"identity_attributes,omitempty"`
	Traits             map[string]any  `json:"traits,omitempty"`
	Applications       []string        `json:"applications_with_data,omitempty"`
	Consents           json.RawMessage `json:"consents,omitempty"`
	MergedTo           json.RawMessage `json:"merged_to,omitempty"`
	MergedFromCount    int             `json:"merged_from_count,omitempty"`
	UpdatedAt          string          `json:"updated_at,omitempty"`
}

func addProfileTools(srv *mcpsdk.Server, c *restClient, allowed func(operation string) bool, readOnly bool) {

	readOnlyTool := &mcpsdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)}

	if allowed(opProfileView) {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_search_profiles",
			Title:       "Search profiles",
			Description: "Search customer profiles with exact or tolerant conditions. Returns a page of profiles and the cursors for the pages either side.",
			Annotations: readOnlyTool,
		}, audited(c, "cds_search_profiles", noResourceID, c.searchProfiles))

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_find_profile",
			Title:       "Find a profile",
			Description: "Find a customer's profile by their user ID, or by an attribute such as email or mobile number, without writing a filter.",
			Annotations: readOnlyTool,
		}, audited(c, "cds_find_profile", func(in findProfileInput) string { return in.UserID }, c.findProfile))

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_get_profile",
			Title:       "Get a profile",
			Description: "Get a customer profile in full: identity attributes, traits, application data and merge state.",
			Annotations: readOnlyTool,
		}, audited(c, "cds_get_profile", profileIDOf, c.getProfile))

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_get_profile_summary",
			Title:       "Summarize a profile",
			Description: "Get a short view of a customer profile: identity attributes, traits, which applications hold data, consents and merge state. Prefer it to cds_get_profile unless application data is needed.",
			Annotations: readOnlyTool,
		}, audited(c, "cds_get_profile_summary", profileIDOf, c.getProfileSummary))

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_get_profile_consents",
			Title:       "Get a profile's consents",
			Description: "Get the consent categories a customer has granted or refused. Check them before using the customer's data for a purpose.",
			Annotations: readOnlyTool,
		}, audited(c, "cds_get_profile_consents", profileIDOf, c.getProfileConsents))
	}

	if allowed(opProfileUpdate) && !readOnly {
		// Identity attributes cannot be patched here. They come from the identity server,
		// and changing one can unify the profile with another, which cannot be undone.
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "cds_patch_profile",
			Title:       "Update a profile",
			Description: "Set traits or application data on a customer profile, merging them into what is there. Identity attributes cannot be changed.",
			Annotations: &mcpsdk.ToolAnnotations{IdempotentHint: true, OpenWorldHint: new(false)},
		}, audited(c, "cds_patch_profile", func(in patchProfileInput) string { return in.ProfileID }, c.patchProfile))
	}
}

func (c *restClient) searchProfiles(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in searchProfilesInput) (*mcpsdk.CallToolResult, any, error) {

	query := url.Values{}
	for _, filter := range in.Filters {
		query.Add("filter", filter)
	}
	for _, filter := range in.FuzzyFilters {
		query.Add("fuzzyFilter", filter)
	}
	if in.Threshold != 0 {
		query.Set("threshold", strconv.FormatFloat(in.Threshold, 'f', -1, 64))
	}
	if len(in.Attributes) > 0 {
		query.Set("attributes", strings.Join(in.Attributes, ","))
	}
	if in.Cursor != "" {
		query.Set("cursor", in.Cursor)
	}
	if in.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(in.PageSize))
	}
	return jsonResult(c.get(ctx, "/profiles", query))
}

func (c *restClient) findProfile(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in findProfileInput) (*mcpsdk.CallToolResult, any, error) {

	field, value := "user_id", in.UserID
	switch {
	case in.UserID != "" && in.Attribute != "":
		return nil, nil, errors.New("give either user_id, or attribute and value, not both")
	case in.UserID == "" && (in.Attribute == "" || in.Value == ""):
		return nil, nil, errors.New("give either user_id, or attribute and value")
	case in.Attribute != "":
		if strings.ContainsAny(in.Attribute, " \t") {
			return nil, nil, fmt.Errorf("attribute %q must be a single field name", in.Attribute)
		}
		field, value = in.Attribute, in.Value
	}

	query := url.Values{"attributes": {lookupAttributes}}
	if in.Fuzzy {
		query.Set("fuzzyFilter", field+" eq "+value)
	} else {
		query.Set("filter", field+" eq "+value)
	}
	return jsonResult(c.get(ctx, "/profiles", query))
}

func (c *restClient) getProfile(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in profileIDInput) (*mcpsdk.CallToolResult, any, error) {

	return jsonResult(c.get(ctx, profilePath(in.ProfileID), nil))
}

func (c *restClient) getProfileConsents(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in profileIDInput) (*mcpsdk.CallToolResult, any, error) {

	return jsonResult(c.get(ctx, profilePath(in.ProfileID)+"/consents", nil))
}

func (c *restClient) getProfileSummary(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in profileIDInput) (*mcpsdk.CallToolResult, any, error) {

	body, err := c.get(ctx, profilePath(in.ProfileID), nil)
	if err != nil {
		return nil, nil, err
	}
	var profile struct {
		ProfileID          string                    `json:"profile_id"`
		UserID             string                    `json:"user_id"`
		IdentityAttributes map[string]any            `json:"identity_attributes"`
		Traits             map[string]any            `json:"traits"`
		ApplicationData    map[string]map[string]any `json:"application_data"`
		MergedTo           json.RawMessage           `json:"merged_to"`
		MergedFrom         []json.RawMessage         `json:"merged_from"`
		Meta               struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, nil, fmt.Errorf("reading profile %s: %w", in.ProfileID, err)
	}

	consents, err := c.get(ctx, profilePath(in.ProfileID)+"/consents", nil)
	if err != nil {
		return nil, nil, err
	}

	summary := profileSummary{
		ProfileID:          profile.ProfileID,
		UserID:             profile.UserID,
		IdentityAttributes: profile.IdentityAttributes,
		Traits:             profile.Traits,
		Consents:           consents,
		MergedTo:           profile.MergedTo,
		MergedFromCount:    len(profile.MergedFrom),
		UpdatedAt:          profile.Meta.UpdatedAt,
	}
	for appID := range profile.ApplicationData {
		summary.Applications = append(summary.Applications, appID)
	}
	encoded, err := json.Marshal(summary)
	return jsonResult(encoded, err)
}

func (c *restClient) patchProfile(ctx context.Context, _ *mcpsdk.CallToolRequest,
	in patchProfileInput) (*mcpsdk.CallToolResult, any, error) {

	if len(in.Traits) == 0 && len(in.ApplicationData) == 0 {
		return nil, nil, errors.New("give traits, application_data or both")
	}
	patch := map[string]any{}
	if len(in.Traits) > 0 {
		patch["traits"] = in.Traits
	}
	if len(in.ApplicationData) > 0 {
		patch["application_data"] = in.ApplicationData
	}
	return jsonResult(c.patch(ctx, profilePath(in.ProfileID), patch))
}

func profilePath(profileID string) string {
	return "/profiles/" + url.PathEscape(profileID)
}

// jsonResult returns an API response to the agent as it came. A list or an object both go
// back as text content, since structured content has to be an object.
func jsonResult(body json.RawMessage, err error) (*mcpsdk.CallToolResult, any, error) {

	if err != nil {
		return nil, nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(body)}},
	}, nil, nil
}

func profileIDOf(in profileIDInput) string { return in.ProfileID }

func noResourceID[In any](In) string { return "" }

// audited logs every call to a tool: which client called it, in which org, on what, and
// whether it succeeded. Profile data is never logged.
func audited[In any](c *restClient, tool string, resourceID func(In) string,
	h mcpsdk.ToolHandlerFor[In, any]) mcpsdk.ToolHandlerFor[In, any] {

	return func(ctx context.Context, req *mcpsdk.CallToolRequest, in In) (*mcpsdk.CallToolResult, any, error) {
		res, out, err := h(ctx, req, in)
		outcome := "success"
		if err != nil {
			outcome = "failure"
		}
		log.GetLogger().Info("MCP tool call",
			log.String("tool", tool),
			log.String("org_handle", c.orgHandle),
			log.String("client_id", c.clientID),
			log.String("resource_id", resourceID(in)),
			log.String("outcome", outcome))
		return res, out, err
	}
}
