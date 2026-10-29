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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
)

const apiBasePath = constants.ApiBasePath + "/v1"

// restClient calls the REST API in process, for one org, with the agent's token.
type restClient struct {
	routes        http.Handler
	orgHandle     string
	pathPrefix    string
	authorization string
	clientID      string
}

func (c *restClient) get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

func (c *restClient) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPatch, path, nil, body)
}

// do serves the request with the REST handler for path, which is relative to /cds/api/v1.
// A response other than 2xx comes back as an error carrying the API's own description.
func (c *restClient) do(ctx context.Context, method, path string, query url.Values,
	body any) (json.RawMessage, error) {

	// Handlers are written for requests from the server, whose Body is never nil.
	var reqBody io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(encoded)
	}

	// The handlers read the org from the context, where the /t/ and /o/ dispatchers put it.
	ctx = context.WithValue(ctx, constants.TenantContextKey, c.orgHandle)
	ctx = context.WithValue(ctx, constants.PathPrefixContextKey, c.pathPrefix)

	target := apiBasePath + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authorization)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	c.routes.ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code > 299 {
		return nil, apiError(rec.Code, rec.Body.Bytes())
	}
	return rec.Body.Bytes(), nil
}

// apiError turns an error response into a message an agent can act on.
func apiError(status int, body []byte) error {

	var msg errors2.ErrorMessage
	if err := json.Unmarshal(body, &msg); err == nil && msg.Message != "" {
		if msg.Description != "" {
			return fmt.Errorf("%s: %s", msg.Message, msg.Description)
		}
		return fmt.Errorf("%s", msg.Message)
	}
	return fmt.Errorf("request failed with status %d %s", status, http.StatusText(status))
}
