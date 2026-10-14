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

package credential

import (
	"context"
	"strings"
	"testing"
)

type stubSource struct{}

func (stubSource) Token(string, []Permission) (Token, error) { return Token{Value: "t"}, nil }
func (stubSource) Provision(context.Context, string) error   { return nil }

func TestTokenHeader(t *testing.T) {

	cases := map[string]struct {
		token Token
		want  string
	}{
		"explicit scheme": {Token{Value: "abc", Scheme: SchemeSystemApp}, "SystemApp abc"},
		"bearer":          {Token{Value: "abc", Scheme: SchemeBearer}, "Bearer abc"},
		// A provider that says nothing means Bearer, so a Source that only fills in the
		// value still produces a usable header rather than one starting with a space.
		"no scheme": {Token{Value: "abc"}, "Bearer abc"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.token.Header(); got != tc.want {
				t.Fatalf("Header() = %q, want %q", got, tc.want)
			}
		})
	}
}

// idp.type is the honest selector; isSystemAppGrantEnabled said the same thing in one
// bit. The flag has to keep working for deployments that carry it, and lose to idp.type
// wherever both are set.
func TestResolveType(t *testing.T) {

	cases := []struct {
		name           string
		idpType        string
		systemAppGrant bool
		want           string
		wantDeprecated bool
	}{
		{"unset is the self-managed server", "", false, TypeWSO2IS, false},
		{"the flag alone means asgardeo", "", true, TypeAsgardeo, true},
		{"idp.type wins over the flag", TypeWSO2IS, true, TypeWSO2IS, false},
		{"idp.type alone", TypeAsgardeo, false, TypeAsgardeo, false},
		{"case and padding do not matter", "  AsGaRdEo ", false, TypeAsgardeo, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, deprecated := ResolveType(tc.idpType, tc.systemAppGrant)
			if got != tc.want {
				t.Fatalf("ResolveType = %q, want %q", got, tc.want)
			}
			if deprecated != tc.wantDeprecated {
				t.Fatalf("deprecated = %v, want %v", deprecated, tc.wantDeprecated)
			}
		})
	}
}

func TestNewSourceUsesTheRegisteredFactory(t *testing.T) {

	Register("stub-provider", func() (Source, error) { return stubSource{}, nil })

	source, err := NewSource("stub-provider")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if _, ok := source.(stubSource); !ok {
		t.Fatalf("got %T, want the registered stub", source)
	}
}

// An unsupported type has to name what is supported. An operator who typed "thunder"
// cannot act on "not supported" alone.
func TestNewSourceRefusesAnUnknownTypeAndListsTheKnownOnes(t *testing.T) {

	Register(TypeWSO2IS, func() (Source, error) { return stubSource{}, nil })

	_, err := NewSource("thunder")
	if err == nil {
		t.Fatal("expected an unknown identity provider type to be refused")
	}
	if !strings.Contains(err.Error(), "thunder") || !strings.Contains(err.Error(), TypeWSO2IS) {
		t.Fatalf("error names neither the bad value nor the valid ones: %v", err)
	}
}

// Falling back to a default provider when nothing was configured would silently
// authenticate as whichever one happened to be first, which is the coupling this
// package exists to remove.
func TestGetSourceFailsWhenNothingIsConfigured(t *testing.T) {

	SetSource("", nil)
	if _, err := GetSource(); err == nil {
		t.Fatal("expected GetSource to fail when no source is configured")
	}

	SetSource(TypeWSO2IS, stubSource{})
	t.Cleanup(func() { SetSource("", nil) })

	if _, err := GetSource(); err != nil {
		t.Fatalf("GetSource after SetSource: %v", err)
	}
	if ResolvedType() != TypeWSO2IS {
		t.Fatalf("ResolvedType = %q, want %q", ResolvedType(), TypeWSO2IS)
	}
}
