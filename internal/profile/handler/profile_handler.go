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

package handler

import (
	"encoding/json"
	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/provider"
	"github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/authentication"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
	"strings"
	"sync"
)

type ProfileHandler struct {
	store map[string]model.Profile
	mu    *sync.RWMutex
}

func NewProfileHandler() *ProfileHandler {

	return &ProfileHandler{
		store: make(map[string]model.Profile),
		mu:    &sync.RWMutex{},
	}
}

// GetProfile handles profile retrieval requests
func (ph *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}
	profileId := pathParts[len(pathParts)-1]
	var profile *model.Profile
	var err error
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	profile, err = profilesService.GetProfile(profileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

// GetCurrentUserProfile handles retrieval of the current user's profile
func (ph *ProfileHandler) GetCurrentUserProfile(w http.ResponseWriter, r *http.Request) {

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	//  Validate token
	isValid, err := authentication.ValidateAuthentication(r)
	if err != nil || !isValid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	//  Get claims from cache
	claims, ok := authentication.GetCachedClaims(token)
	if !ok {
		http.Error(w, "Token claims not found", http.StatusUnauthorized)
		return
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		http.Error(w, "Missing 'sub' in token", http.StatusUnauthorized)
		return
	}

	//  Fetch profile
	profile, err := service.FindProfileByUserName(sub)
	if err != nil || profile == nil {
		log.Print("error fetching profile: ", err)
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	//  Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// DeleteProfile handles profile deletion
func (ph *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	profileId := pathParts[len(pathParts)-1]
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	err := profilesService.DeleteProfile(profileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAllProfiles handles profile retrieval with and without filters
func (ph *ProfileHandler) GetAllProfiles(w http.ResponseWriter, r *http.Request) {

	logger := log.GetLogger()
	logger.Info("Fetching all profiles without filters")
	var profiles []model.Profile
	var err error
	// Build the filter from query params
	filter := r.URL.Query()[constants.Filter] // Handles multiple filters
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	if len(filter) > 0 {
		profiles, err = profilesService.GetAllProfilesWithFilter(filter)
	} else {
		profiles, err = profilesService.GetAllProfiles()
	}
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profiles)
}
