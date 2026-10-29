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

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// A resource is context an agent reads before acting, served by a REST GET.
type resource struct {
	operation   string
	uri         string
	name        string
	title       string
	description string
	path        string
}

var resources = []resource{
	{
		operation:   opProfileSchemaView,
		uri:         "cds://profile-schema",
		name:        "profile-schema",
		title:       "Profile schema",
		description: "The attributes a profile can hold, by scope: identity attributes, traits and application data. Read it to know which fields to filter on or update.",
		path:        "/profile-schema",
	},
	{
		operation:   opConsentCategoryView,
		uri:         "cds://consent-categories",
		name:        "consent-categories",
		title:       "Consent categories",
		description: "The consent categories of the organization: what each one covers and which attributes it governs. Read it to interpret a profile's consents.",
		path:        "/consent-categories",
	},
}

func addResources(srv *mcpsdk.Server, c *restClient, allowed func(operation string) bool) {

	for _, res := range resources {
		if !allowed(res.operation) {
			continue
		}
		srv.AddResource(&mcpsdk.Resource{
			URI:         res.uri,
			Name:        res.name,
			Title:       res.title,
			Description: res.description,
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			body, err := c.get(ctx, res.path, nil)
			if err != nil {
				return nil, err
			}
			return &mcpsdk.ReadResourceResult{
				Contents: []*mcpsdk.ResourceContents{{URI: res.uri, MIMEType: "application/json", Text: string(body)}},
			}, nil
		})
	}
}
