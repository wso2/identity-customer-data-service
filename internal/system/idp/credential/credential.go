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

// Package credential holds the contract for how CDS authenticates itself to an
// identity provider.
//
// CDS asks for a credential that carries the permissions it needs and puts the
// result in an Authorization header. It does not know which grant produced it, what
// the provider's scopes are called, or whether a grant was involved at all - a
// provider that authenticates with a static key or a client certificate answers
// without a round trip.
//
// The package holds only the contract. The implementations live with the HTTP
// machinery that talks to each provider and register themselves here, so that this
// package stays free of provider-specific imports and can be depended on from
// anywhere.
package credential

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Identity provider types, as the idp.type setting names them. One CDS deployment
// works with one identity provider.
const (
	// TypeWSO2IS is a WSO2 Identity Server the deployment runs itself.
	TypeWSO2IS = "wso2is"
	// TypeAsgardeo is the WSO2 managed identity cloud.
	TypeAsgardeo = "asgardeo"
)

// Authorization schemes a credential can be presented under.
const (
	SchemeBearer    = "Bearer"
	SchemeSystemApp = "SystemApp"
)

// Permission names something CDS needs to do at the identity provider, in CDS's own
// terms rather than the provider's.
//
// CDS asks for ReadClaims; what that is called, which API resource grants it, and
// whether the provider has a notion of scopes at all is the provider's business.
type Permission string

const (
	// PermissionReadApplications reads application registrations.
	PermissionReadApplications Permission = "read:applications"
	// PermissionReadClaims reads the claim dialects the profile schema is built from.
	PermissionReadClaims Permission = "read:claims"
	// PermissionReadUsers reads user records.
	PermissionReadUsers Permission = "read:users"
)

// ManagementPermissions is what CDS needs to call an identity provider at all. It is
// the set a Source is asked for unless a caller needs something narrower.
func ManagementPermissions() []Permission {

	return []Permission{PermissionReadApplications, PermissionReadClaims, PermissionReadUsers}
}

// Token is a value CDS can put in an Authorization header.
//
// It is deliberately not an OAuth access token: Scheme carries whatever the provider
// expects in front of the value, and a provider that needs no token exchange returns
// one it already holds.
type Token struct {
	Value  string
	Scheme string
}

// Header renders the Authorization header value. An empty Scheme is Bearer, which is
// what a provider that does not say otherwise means.
func (t Token) Header() string {

	scheme := t.Scheme
	if scheme == "" {
		scheme = SchemeBearer
	}
	return scheme + " " + t.Value
}

// IsZero reports whether the token carries no value.
func (t Token) IsZero() bool {

	return t.Value == ""
}

// Source is how CDS authenticates itself to the identity provider of the deployment.
type Source interface {
	// Token returns a credential for calling the provider as orgHandle, carrying at
	// least the named permissions.
	//
	// It takes no context because the call chain above it carries none yet; see the
	// note on FetchToken in the identity client. Adding one here is a follow-up that
	// belongs with that plumbing.
	Token(orgHandle string, permissions []Permission) (Token, error)

	// Provision makes sure orgHandle has a credential this Source can use, creating one
	// at the provider where the provider expects CDS to create its own.
	//
	// A provider whose credentials are issued to CDS from outside - a control plane, an
	// operator filling in configuration - does nothing here and returns nil. Callers
	// therefore call it unconditionally, and it must be safe to call more than once for
	// the same organisation.
	Provision(ctx context.Context, orgHandle string) error
}

// Factory builds the Source of one identity provider type.
type Factory func() (Source, error)

var (
	mu           sync.RWMutex
	factories    = map[string]Factory{}
	source       Source
	resolvedType string
)

// Register records the Factory of an identity provider type. Call it from an init
// function in the package that implements the Source, so that importing that package
// is what makes the type available.
func Register(idpType string, factory Factory) {

	mu.Lock()
	defer mu.Unlock()
	factories[normalize(idpType)] = factory
}

// NewSource builds the Source of idpType. An empty type is TypeWSO2IS, which is the
// identity provider a deployment that says nothing is running.
func NewSource(idpType string) (Source, error) {

	resolved := normalize(idpType)
	if resolved == "" {
		resolved = TypeWSO2IS
	}

	mu.RLock()
	factory, found := factories[resolved]
	mu.RUnlock()
	if !found {
		return nil, fmt.Errorf("the identity provider type '%s' is not supported. Use one of: %s",
			idpType, strings.Join(registered(), ", "))
	}
	return factory()
}

// ResolveType returns the identity provider type of a deployment.
//
// auth_server.isSystemAppGrantEnabled is the deprecated way of saying the same thing.
// The system application grant is an Asgardeo flow, so the flag was only ever a way of
// saying "this deployment is Asgardeo" - one bit standing in for a choice that now has
// more than two answers. It is honoured when idp.type says nothing, so deployments
// carrying it keep working, and idp.type wins wherever both are set.
//
// deprecated reports whether the answer came from the flag, so the caller can say so
// once at start rather than on every token.
func ResolveType(idpType string, systemAppGrantEnabled bool) (resolved string, deprecated bool) {

	if normalize(idpType) != "" {
		return normalize(idpType), false
	}
	if systemAppGrantEnabled {
		return TypeAsgardeo, true
	}
	return TypeWSO2IS, false
}

// SetSource records the Source the deployment resolved, and the type it was resolved
// from. The server calls it once at start, and tests use it to put a fake identity
// provider in place.
func SetSource(idpType string, s Source) {

	mu.Lock()
	defer mu.Unlock()
	resolvedType = normalize(idpType)
	source = s
}

// ResolvedType is the identity provider type the deployment resolved at start.
//
// It exists for the few places that still have to branch on the provider rather than
// go through the Source - the introspection endpoint is one - so that they read the
// resolved answer instead of the deprecated flag, which a deployment on idp.type no
// longer sets.
func ResolvedType() string {

	mu.RLock()
	defer mu.RUnlock()
	return resolvedType
}

// GetSource returns the Source the deployment resolved.
//
// It fails when nothing has been set rather than falling back to a default identity
// provider. A deployment that reached this point unconfigured would otherwise
// silently authenticate as whichever provider happened to be the default, which is
// the coupling this package exists to remove.
func GetSource() (Source, error) {

	mu.RLock()
	s := source
	mu.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("no identity provider credential source is configured")
	}
	return s, nil
}

// registered lists the known types, for an error a reader can act on.
func registered() []string {

	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalize(idpType string) string {

	return strings.ToLower(strings.TrimSpace(idpType))
}
