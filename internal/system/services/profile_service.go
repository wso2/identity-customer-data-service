/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/wso2/identity-customer-data-service/internal/profile/handler"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
)

type ProfileService struct {
	profileHandler *handler.ProfileHandler
}

func NewProfileService() *ProfileService {
	return &ProfileService{
		profileHandler: handler.NewProfileHandler(),
	}
}

// Route handles all tenant-aware profile-related endpoints
func (s *ProfileService) Route(w http.ResponseWriter, r *http.Request) {

	log.GetLogger().Info("Registering services with base path: " + r.URL.Path)
	path := strings.TrimSuffix(r.URL.Path, "/") // Just clean the trailing /
	method := r.Method

	switch {
	case method == http.MethodGet && path == "/profiles":
		s.profileHandler.GetAllProfiles(w, r)

	case method == http.MethodPost && path == "/profiles":
		s.profileHandler.CreateProfile(w, r)

	case method == http.MethodGet && path == "/profiles/Me":
		s.profileHandler.GetCurrentUserProfile(w, r)

	case method == http.MethodPost && path == "/profiles/sync":
		s.profileHandler.SyncProfile(w, r)

	case strings.HasPrefix(path, "/profiles/"):
		switch method {
		case http.MethodGet:
			s.profileHandler.GetProfile(w, r)
		case http.MethodPatch:
			s.profileHandler.PatchProfile(w, r)
		case http.MethodPut:
			s.profileHandler.UpdateProfile(w, r)
		case http.MethodDelete:
			s.profileHandler.DeleteProfile(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}

	default:
		http.NotFound(w, r)
	}
}
