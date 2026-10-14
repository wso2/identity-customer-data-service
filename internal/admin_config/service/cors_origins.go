/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// corsOriginsCacheTTL bounds how long an organisation's origin list is served from memory.
// The cache is per process and is invalidated in-process on update, so this is also the
// window in which another replica may still answer with the previous list.
const corsOriginsCacheTTL = 30 * time.Second

// corsOriginsCache holds []string origin lists keyed by organisation handle. The CORS
// middleware runs on every request, preflight included, so the list is cached rather
// than read from the database each time.
var corsOriginsCache = cache.NewCache(corsOriginsCacheTTL)

// normalizeOrigin lower-cases the scheme and host of an origin so that comparison against
// a browser-supplied Origin header, which is already normalized that way, is reliable.
// The port is left as given.
func normalizeOrigin(origin string) string {

	trimmed := strings.TrimSpace(origin)
	scheme, rest, found := strings.Cut(trimmed, "://")
	if !found {
		return trimmed
	}
	host, remainder, hasRemainder := strings.Cut(rest, "/")
	normalized := strings.ToLower(scheme) + "://" + strings.ToLower(host)
	if hasRemainder {
		return normalized + "/" + remainder
	}
	return normalized
}

// validateCORSOrigins checks that every entry is an absolute scheme://host[:port] origin
// carrying no path, no query, no fragment and no credentials, and returns the normalized
// list with duplicates removed. A wildcard is refused: the middleware answers with
// Access-Control-Allow-Credentials, which a browser rejects alongside "*", so accepting
// one here would only produce an origin that never matches.
func validateCORSOrigins(origins []string) ([]string, error) {

	normalized := make([]string, 0, len(origins))
	seen := make(map[string]bool, len(origins))

	for _, origin := range origins {
		candidate := strings.TrimSpace(origin)

		if candidate == "" {
			return nil, newCORSOriginError("An allowed origin may not be empty.")
		}
		// A browser matches Origin exactly, so neither "*" nor a wildcard host such as
		// "https://*.acme.com" can ever match. Refuse both rather than store an origin
		// that silently never applies.
		if strings.Contains(candidate, "*") {
			return nil, newCORSOriginError(fmt.Sprintf(
				"Origin '%s' may not contain a wildcard. List each origin explicitly.", candidate))
		}
		if strings.HasSuffix(candidate, "/") {
			return nil, newCORSOriginError(fmt.Sprintf(
				"Origin '%s' must not end with a trailing slash.", candidate))
		}

		parsed, err := url.Parse(candidate)
		if err != nil {
			return nil, newCORSOriginError(fmt.Sprintf("Origin '%s' is not a valid URL.", candidate))
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, newCORSOriginError(fmt.Sprintf(
				"Origin '%s' must use the http or https scheme.", candidate))
		}
		if parsed.Host == "" {
			return nil, newCORSOriginError(fmt.Sprintf(
				"Origin '%s' must be an absolute origin of the form scheme://host[:port].", candidate))
		}
		if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return nil, newCORSOriginError(fmt.Sprintf(
				"Origin '%s' must not carry a path, query, fragment or credentials.", candidate))
		}

		candidate = normalizeOrigin(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		normalized = append(normalized, candidate)
	}

	return normalized, nil
}

func newCORSOriginError(description string) error {

	return errors.NewClientError(errors.ErrorMessage{
		Code:        errors.UPDATE_CONFIG_BAD_REQUEST.Code,
		Message:     errors.UPDATE_CONFIG_BAD_REQUEST.Message,
		Description: description,
	}, http.StatusBadRequest)
}
