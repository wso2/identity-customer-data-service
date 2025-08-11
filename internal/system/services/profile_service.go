/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package services

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/profile/handler"
	"net/http"
	"strings"
)

type ProfileService struct {
	profileHandler *handler.ProfileHandler
}

//func NewProfileService(mux *http.ServeMux, apiBasePath string) *ProfileService {
//	instance := &ProfileService{
//		profileHandler: handler.NewProfileHandler(),
//	}
//
//	// Rewrite global paths → /t/carbon.super/...
//	mux.HandleFunc(apiBasePath, instance.globalToTenantedRewrite)
//
//	// Tenant-aware routing
//	mux.HandleFunc("/t/", instance.tenantRouteDispatcher)
//
//	return instance
//}
//
//func (s *ProfileService) globalToTenantedRewrite(w http.ResponseWriter, r *http.Request) {
//	newPath := "/t/carbon.super" + r.URL.Path
//	http.Redirect(w, r, newPath, http.StatusTemporaryRedirect)
//}
//
//// Handles /t/{tenant}/api/v1/...
//func (s *ProfileService) tenantRouteDispatcher(w http.ResponseWriter, r *http.Request) {
//	path := strings.TrimSuffix(r.URL.Path, "/")
//
//	if !strings.HasPrefix(path, "/t/") {
//		http.NotFound(w, r)
//		return
//	}
//
//	parts := strings.SplitN(path[len("/t/"):], "/", 2)
//	if len(parts) != 2 {
//		http.NotFound(w, r)
//		return
//	}
//
//	tenantID := parts[0]
//	remainingPath := "/" + parts[1]
//
//	ctx := context.WithValue(r.Context(), constants.TenantContextKey, tenantID)
//	r = r.WithContext(ctx)
//	r.URL.Path = remainingPath
//
//	s.routeToHandler(w, r)
//}
//
//func (s *ProfileService) routeToHandler(w http.ResponseWriter, r *http.Request) {
//	path := strings.TrimSuffix(r.URL.Path, "/")
//	method := r.Method
//
//	switch {
//	// List all profiles
//	case method == http.MethodGet && path == "/api/v1/profiles":
//		s.profileHandler.GetAllProfiles(w, r)
//
//	// Get current user profile
//	case method == http.MethodGet && path == "/api/v1/profiles/Me":
//		s.profileHandler.GetCurrentUserProfile(w, r)
//
//	// Specific profile (GET, PATCH, DELETE): /profiles/{id}
//	case strings.HasPrefix(path, "/api/v1/profiles/") && len(strings.Split(path, "/")) == 4:
//		profileId := strings.Split(path, "/")[3]
//
//		switch method {
//		case http.MethodGet:
//			s.profileHandler.GetProfile(w, r, profileId)
//		case http.MethodPatch:
//			s.profileHandler.UpdateProfile(w, r, profileId)
//		case http.MethodDelete:
//			s.profileHandler.DeleteProfile(w, r, profileId)
//		default:
//			http.NotFound(w, r)
//		}
//
//	// Create a profile (POST to /profiles/)
//	case method == http.MethodPost && path == "/api/v1/profiles":
//		s.profileHandler.CreateProfile(w, r)
//
//	default:
//		http.NotFound(w, r)
//	}
//}

func NewProfileService(mux *http.ServeMux, apiBasePath string) *ProfileService {

	instance := &ProfileService{
		profileHandler: handler.NewProfileHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)

	return instance
}

func (s *ProfileService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc(fmt.Sprintf("POST %s/profiles/initialize", apiBasePath), s.profileHandler.InitProfile)
	mux.HandleFunc(fmt.Sprintf("GET %s/profiles", apiBasePath), s.profileHandler.GetAllProfiles)

	mux.HandleFunc(fmt.Sprintf("GET %s/profiles/me", apiBasePath), s.profileHandler.GetCurrentUserProfile)
	mux.HandleFunc(fmt.Sprintf("PATCH %s/profiles/me", apiBasePath), s.profileHandler.PatchCurrentUserProfile)
	mux.HandleFunc(fmt.Sprintf("POST %s/profiles/sync", apiBasePath), s.profileHandler.SyncProfile)

	// handles all requests to /profiles/{id} and its sub resources
	mux.HandleFunc(fmt.Sprintf("%s/profiles/", apiBasePath), s.profileResourceHandler)
}

// profileResourceHandler handles all requests to /profiles/{id} and its sub resources
func (s *ProfileService) profileResourceHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the path to extract components
	path := r.URL.Path

	// Split the path into segments
	// Format: /api/v1/profiles/{profileId}[/consents]
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// We know the pattern starts with /profiles/
	// Determine if this is a direct profile request or a consent request
	if len(segments) < 3 {
		http.NotFound(w, r)
		return
	}

	// The profile ID is always the segment after "profiles"
	profileIdIndex := -1
	for i, segment := range segments {
		if segment == "profiles" {
			profileIdIndex = i + 1
			break
		}
	}

	// If we couldn't find "profiles" or there's no ID after it, return 404
	if profileIdIndex == -1 || profileIdIndex >= len(segments) {
		http.NotFound(w, r)
		return
	}

	// The profileId would be segments[profileIdIndex] but since we don't use it directly
	// in this function (each handler extracts it again from the URL path),
	// we avoid declaring an unused variable.

	// Check if this is a consent request
	// Format: /api/v1/profiles/{profileId}/consents
	isConsentRequest := profileIdIndex+1 < len(segments) && segments[profileIdIndex+1] == "consents"

	if isConsentRequest {
		// Handle consent request
		switch r.Method {
		case http.MethodGet:
			s.profileHandler.GetProfileConsents(w, r)
		case http.MethodPut:
			s.profileHandler.UpdateProfileConsents(w, r)
		default:
			http.Error(w, "Method not allowed for consents", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle direct profile request
	switch r.Method {
	case http.MethodGet:
		s.profileHandler.GetProfile(w, r)
	case http.MethodPut:
		s.profileHandler.UpdateProfile(w, r)
	case http.MethodPatch:
		s.profileHandler.PatchProfile(w, r)
	case http.MethodDelete:
		s.profileHandler.DeleteProfile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
