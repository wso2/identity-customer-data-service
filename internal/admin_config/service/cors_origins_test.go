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
	"net/http"
	"reflect"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/errors"
)

func TestValidateCORSOriginsAcceptsWellFormedOrigins(t *testing.T) {

	origins := []string{
		"https://app.acme.com",
		"http://localhost:3000",
		"https://app.acme.com:8443",
	}

	got, err := validateCORSOrigins(origins)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, origins) {
		t.Errorf("got %v, want %v", got, origins)
	}
}

func TestValidateCORSOriginsNormalizesAndDeduplicates(t *testing.T) {

	got, err := validateCORSOrigins([]string{
		"HTTPS://App.Acme.COM",
		" https://app.acme.com ",
		"https://other.acme.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"https://app.acme.com", "https://other.acme.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestValidateCORSOriginsEmptyListIsAllowed(t *testing.T) {

	got, err := validateCORSOrigins([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty list", got)
	}
}

func TestValidateCORSOriginsRejectsMalformedOrigins(t *testing.T) {

	cases := map[string]string{
		"wildcard":        "*",
		"trailing slash":  "https://app.acme.com/",
		"with path":       "https://app.acme.com/callback",
		"with query":      "https://app.acme.com?a=b",
		"with fragment":   "https://app.acme.com#x",
		"no scheme":       "app.acme.com",
		"bare host":       "//app.acme.com",
		"wrong scheme":    "ftp://app.acme.com",
		"empty":           "",
		"blank":           "   ",
		"credentials":     "https://user:pass@app.acme.com",
		"wildcard subdom": "https://*.acme.com",
	}

	for name, origin := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := validateCORSOrigins([]string{origin})
			if err == nil {
				t.Fatalf("validateCORSOrigins(%q) = %v, want an error", origin, got)
			}
			clientError, ok := err.(*errors.ClientError)
			if !ok {
				t.Fatalf("error is %T, want *errors.ClientError", err)
			}
			if clientError.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", clientError.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// One bad entry rejects the whole update, so a partially valid list is never stored.
func TestValidateCORSOriginsRejectsWholeListOnOneBadEntry(t *testing.T) {

	_, err := validateCORSOrigins([]string{"https://app.acme.com", "*"})
	if err == nil {
		t.Fatal("expected an error for a list containing a wildcard")
	}
}
