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
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/provider"
	"github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/authn"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
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

	//todo: Uncomment this if you want to enforce auth
	//err := utils.AuthnAndAuthz(r, "profile:view")
	//if err != nil {
	//	utils.HandleError(w, err)
	//	return
	//}
	//
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	profile, err := profilesService.GetProfile(profileId)
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

	// todo: Should be abel to check from the profileId in the cookies as well.
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	err := utils.AuthnAndAuthz(r, "profile:view")
	if err != nil {
		utils.HandleError(w, err)
	}

	//  Get claims from cache
	claims, ok := authn.GetCachedClaims(token)
	if !ok {
		http.Error(w, "Token claims not found", http.StatusUnauthorized)
		return
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		http.Error(w, "Missing 'sub' in token", http.StatusUnauthorized)
		return
	}

	tenantId := utils.ExtractTenantIdFromPath(r)

	//  Fetch profile
	profile, err := service.FindProfileByUserName(tenantId, sub)
	if err != nil || profile == nil {
		utils.HandleError(w, err)
		return
	}
	//  Return JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// DeleteProfile handles profile deletion
func (ph *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusNotFound)
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
	tenantId := utils.ExtractTenantIdFromPath(r)
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Parse filters
	queryFilters := r.URL.Query()[constants.Filter]
	var filters []string
	for _, f := range queryFilters {
		splitFilters := strings.Split(f, " and ")
		for _, sf := range splitFilters {
			sf = strings.TrimSpace(sf)
			if sf != "" {
				filters = append(filters, sf)
			}
		}
	}

	// Parse selective attributes (e.g., ?attributes=identity_attributes.username,application_data.cart_items)
	requestedAttrs := parseRequestedAttributes(r)

	var profiles []model.ProfileResponse
	var err error

	if len(filters) > 0 {
		// Fall back to full response if filters are used
		logger.Info("Fetching profiles with filters")
		profiles, err = profilesService.GetAllProfilesWithFilter(tenantId, filters)
	} else {
		logger.Info("Fetching all profiles with requested attributes")
		profiles, err = profilesService.GetAllProfiles(tenantId)
	}

	listResponse, err := buildProfileListResponse(profiles, requestedAttrs), nil

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResponse)
}

func buildProfileListResponse(profiles []model.ProfileResponse, requestedAttrs map[string][]string) []model.ProfileListResponse {
	var result []model.ProfileListResponse

	for _, profile := range profiles {
		profileRes := model.ProfileListResponse{
			ProfileId: profile.ProfileId,
			Meta:      profile.Meta,
			UserId:    profile.UserId,
		}

		// Identity Attributes
		if fields, ok := requestedAttrs["identity_attributes"]; ok {
			filtered := make(map[string]interface{})
			for _, f := range fields {
				if f == "*" {
					filtered = profile.IdentityAttributes
					break
				}
				if val, exists := profile.IdentityAttributes[f]; exists {
					filtered[f] = val
				}
			}
			profileRes.IdentityAttributes = filtered
		} else if requestedAttrs == nil {
			profileRes.IdentityAttributes = profile.IdentityAttributes
		}

		// Traits
		if fields, ok := requestedAttrs["traits"]; ok {
			filtered := make(map[string]interface{})
			for _, f := range fields {
				if f == "*" {
					filtered = profile.Traits
					break
				}
				if val, exists := profile.Traits[f]; exists {
					filtered[f] = val
				}
			}
			profileRes.Traits = filtered
		} else if requestedAttrs == nil {
			profileRes.Traits = profile.Traits
		}

		// Application Data
		//appData := ConvertAppDataToMap(profile.ApplicationData)
		appData := profile.ApplicationData
		if len(appData) > 0 {
			filteredAppData := make(map[string]map[string]interface{})

			if requestedAttrs == nil || len(requestedAttrs["application_data"]) == 0 {
				filteredAppData = appData
			} else {
				fields := requestedAttrs["application_data"]
				for appKey, appFields := range appData {
					filteredAppData[appKey] = make(map[string]interface{})
					for _, f := range fields {
						if f == "*" {
							filteredAppData[appKey] = appFields
							break
						}
						if val, ok := appFields[f]; ok {
							filteredAppData[appKey][f] = val
						}
					}
				}
			}

			if len(filteredAppData) > 0 {
				profileRes.ApplicationData = filteredAppData
			}
		}

		result = append(result, profileRes)
	}

	return result
}

func ConvertAppDataToMap(appDataList []model.ApplicationData) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	for _, appData := range appDataList {
		appMap := make(map[string]interface{})

		// Add all app-specific key-values
		for k, v := range appData.AppSpecificData {
			appMap[k] = v
		}

		result[appData.AppId] = appMap
	}

	return result
}

func parseRequestedAttributes(r *http.Request) map[string][]string {
	attrs := r.URL.Query().Get("attributes")
	if attrs == "" {
		return nil
	}

	result := make(map[string][]string)
	for _, attr := range strings.Split(attrs, ",") {
		attr = strings.TrimSpace(attr)
		parts := strings.SplitN(attr, ".", 2)
		scope := parts[0]
		if len(parts) == 2 {
			result[scope] = append(result[scope], parts[1])
		} else {
			result[scope] = append(result[scope], "*")
		}
	}
	return result
}

// GetProfileWithUserId handles profile retrieval with userId as the filter.
func (ph *ProfileHandler) GetProfileWithUserId(w http.ResponseWriter, r *http.Request) {

	tenantId := utils.ExtractTenantIdFromPath(r)

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Check if userID query param is present
	userID := r.URL.Query().Get("userID")

	var (
		profiles []model.ProfileResponse
		err      error
	)

	if userID != "" {
		// Build filter for userID
		filter := fmt.Sprintf("identity_attributes.user_id eq %s", userID)
		profiles, err = profilesService.GetAllProfilesWithFilter(tenantId, []string{filter})
	}

	if err != nil {
		utils.HandleError(w, err)
		return
	}

	if len(profiles) == 0 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("No profiles found for userID: %s", userID),
		}, http.StatusNotFound)
		utils.HandleError(w, clientError)
		return
	}
	if len(profiles) > 1 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.MULTIPLE_PROFILES_FOUND.Code,
			Message:     errors2.MULTIPLE_PROFILES_FOUND.Message,
			Description: fmt.Sprintf("Multiple profiles found for userID: %s", userID),
		}, http.StatusConflict)
		utils.HandleError(w, clientError)
		return
	}
	profile := profiles[0] // Assuming we want the first profile found
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

func (ph *ProfileHandler) CreateProfile(writer http.ResponseWriter, request *http.Request) {

	// todo: Uncomment this if you want to enforce auth
	//err := utils.AuthnAndAuthz(request, "profile:create")
	//if err != nil {
	//	utils.HandleError(writer, err)
	//	return
	//}

	orgId := utils.ExtractTenantIdFromPath(request)

	var profile model.ProfileRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&profile)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid request body. %v", err)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errMsg,
		}, http.StatusBadRequest)

		utils.WriteErrorResponse(writer, clientError)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	profileResponse, err := profilesService.CreateProfile(profile, orgId)
	if err != nil {
		utils.HandleError(writer, err)
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     constants.ProfileCookie,
		Value:    profileResponse.ProfileId,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // set to false if testing on localhost without https
		SameSite: http.SameSiteLaxMode,
	})

	scheme := "https"
	if strings.HasPrefix(request.Host, "localhost") {
		scheme = "http"
	}

	location := fmt.Sprintf("%s://%s%s/profiles/%s",
		scheme, //todo: request.URL.Scheme // always empty in Go’s standard `net/http`
		request.Host,
		constants.ApiBasePath,
		profileResponse.ProfileId,
	)
	writer.Header().Set("Location", location)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(profileResponse)
}

func (ph *ProfileHandler) InitProfile(w http.ResponseWriter, r *http.Request) {

	orgId := utils.ExtractTenantIdFromPath(r)
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Check if a valid cookie exists
	profileCookie, err := r.Cookie(constants.ProfileCookie)
	if err == nil && profileCookie.Value != "" {
		if ph.handleExistingCookie(w, r, profileCookie.Value) {
			return
		}
	}

	// If no valid cookie, create a new profile and cookie
	profile := model.ProfileRequest{}
	profileResponse, err := profilesService.CreateProfile(profile, orgId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Create and set a new cookie
	err = setProfileCookie(w, profileResponse.ProfileId, r)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Construct Location header for created resource
	location := fmt.Sprintf("%s://%s%s/profiles/%s",
		detectScheme(r),
		r.Host,
		constants.ApiBasePath,
		profileResponse.ProfileId,
	)

	w.Header().Set("Location", location)
	respondJSON(w, http.StatusCreated, profileResponse)
}

// Handles existing cookie logic, returns true if response was already written
func (ph *ProfileHandler) handleExistingCookie(w http.ResponseWriter, r *http.Request, cookieVal string) bool {

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	cookieObj, err := profilesService.GetProfileCookie(cookieVal)
	if err != nil || cookieObj == nil {
		return false
	}

	var profileResponse *model.ProfileResponse
	if cookieObj.IsActive {
		profileResponse, err = profilesService.GetProfile(cookieObj.ProfileId)
	} else {
		profile := model.ProfileRequest{}
		profileResponse, err = profilesService.CreateProfile(profile, utils.ExtractTenantIdFromPath(r))
		if err == nil {
			cookieObj, err = profilesService.CreateProfileCookie(profileResponse.ProfileId)
		}
	}
	if err != nil {
		utils.HandleError(w, err)
		return true
	}

	_ = setProfileCookie(w, profileResponse.ProfileId, r)
	respondJSON(w, http.StatusOK, profileResponse)
	return true
}

func setProfileCookie(w http.ResponseWriter, profileId string, r *http.Request) error {
	cookie := &http.Cookie{
		Name:     constants.ProfileCookie,
		Value:    profileId,
		Path:     "/",
		HttpOnly: true,
		Secure:   !strings.HasPrefix(r.Host, "localhost"),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return nil
}

func detectScheme(r *http.Request) string {
	if strings.HasPrefix(r.Host, "localhost") {
		return "http"
	}
	return "https"
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (ph *ProfileHandler) UpdateProfile(writer http.ResponseWriter, request *http.Request) {

	pathParts := strings.Split(request.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(writer, "Invalid path", http.StatusNotFound)
		return
	}
	profileId := pathParts[len(pathParts)-1]

	// Uncomment this if you want to enforce auth
	// err := utils.AuthnAndAuthz(request, "profile:update")
	// if err != nil {
	//	utils.HandleError(writer, err)
	//	return
	//}

	var profile model.ProfileRequest
	err := json.NewDecoder(request.Body).Decode(&profile)
	if err != nil {
		http.Error(writer, "Invalid request body", http.StatusBadRequest)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	_, err = profilesService.UpdateProfile(profileId, profile)
	if err != nil {
		utils.HandleError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(`{"status": "updated"}`))
}

// PatchProfile handles partial updates to a profile
func (ph *ProfileHandler) PatchProfile(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}
	profileId := pathParts[len(pathParts)-1]

	var patchData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	updatedProfile, err := profilesService.PatchProfile(profileId, patchData)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedProfile)
}

// PatchCurrentUserProfile handles partial updates to a profile
func (ph *ProfileHandler) PatchCurrentUserProfile(w http.ResponseWriter, r *http.Request) {

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}

	cookie, err := r.Cookie(constants.ProfileCookie)
	if err != nil {

		http.Error(w, "Missing or invalid cdm_profile_id cookie", http.StatusUnauthorized)
		return
	}
	profileId := cookie.Value

	var patchData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	updatedProfile, err := profilesService.PatchProfile(profileId, patchData)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedProfile)
}

func (ph *ProfileHandler) SyncProfile(writer http.ResponseWriter, request *http.Request) {
	var profileSync model.ProfileSync
	err := json.NewDecoder(request.Body).Decode(&profileSync)
	if err != nil {
		http.Error(writer, "Invalid request body", http.StatusBadRequest)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	profileId := profileSync.ProfileId
	identityClaims := profileSync.Claims
	tenantId := profileSync.TenantId

	if tenantId == "" {
		//tenantId = utils.ExtractTenantIdFromPath(request)
		//todo: should we expect tenant id in the path or as body param
		utils.HandleError(writer, fmt.Errorf("Tenant id cannot be empty: %w", err))
		return
	}

	var existingProfile *model.ProfileResponse

	if profileSync.Event == "POST_ADD_USER" {
		if profileSync.ProfileId != "" && profileSync.UserId != "" {

			// This sceario is when the user anonymously tried and then trying to signup or login. So profile with profile id exists
			existingProfile, err = profilesService.GetProfile(profileSync.ProfileId)
			if existingProfile != nil {
				// Update identity attributes based on claim URIs
				if existingProfile.IdentityAttributes == nil {
					existingProfile.IdentityAttributes = make(map[string]interface{})
				}

				for claimURI, value := range identityClaims {
					attributeKeyPath := extractClaimKeyFromLocalURI(claimURI)
					setNestedMapValue(existingProfile.IdentityAttributes, attributeKeyPath, value)
				}

				profileRequest := model.ProfileRequest{
					UserId:             profileSync.UserId,
					IdentityAttributes: existingProfile.IdentityAttributes,
					Traits:             existingProfile.Traits,
					ApplicationData:    existingProfile.ApplicationData,
				}

				// Save updated profile
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, profileRequest)
				if err != nil {
					utils.HandleError(writer, fmt.Errorf("failed to update profile: %w", err))
					return
				}
				return
			}
			return
		} else if profileSync.ProfileId == "" {
			// this is when we create a profile for a new user created in IS
			existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
			if existingProfile == nil {
				identityAttributes := make(map[string]interface{})
				for claimURI, value := range identityClaims {
					attributeKeyPath := extractClaimKeyFromLocalURI(claimURI)
					setNestedMapValue(identityAttributes, attributeKeyPath, value)
				}

				profileRequest := model.ProfileRequest{
					UserId:             profileSync.UserId,
					IdentityAttributes: identityAttributes,
				}
				_, err := profilesService.CreateProfile(profileRequest, tenantId)
				if err != nil {
					utils.HandleError(writer, fmt.Errorf("failed to create profile: %w", err))
					return
				}
			}
			return
		}
		return
		// if needed can ensure if profile got created
	}

	if profileSync.Event == "AUTHENTICATION_SUCCESS" {
		log.GetLogger().Info("Authentication success event received for user: " + profileSync.UserId)
		if profileSync.ProfileId != "" && profileSync.UserId != "" {
			// This scenario is when the user logs in with a profileId existing.
			existingProfile, err = profilesService.GetProfile(profileSync.ProfileId)
			if existingProfile != nil {
				// Update identity attributes based on claim URIs
				if existingProfile.IdentityAttributes == nil {
					existingProfile.IdentityAttributes = make(map[string]interface{})
				}

				// This is to update userId
				//todo: See if we need to fetch the identity data as well.

				profileRequest := model.ProfileRequest{
					UserId:             profileSync.UserId,
					IdentityAttributes: existingProfile.IdentityAttributes,
					Traits:             existingProfile.Traits,
					ApplicationData:    existingProfile.ApplicationData,
				}
				// Save updated profile
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, profileRequest)
				if err != nil {
					utils.HandleError(writer, fmt.Errorf("failed to update profile: %w", err))
					return
				}
				return
			}
			return
		}
	}

	if profileSync.Event == "POST_DELETE_USER_WITH_ID" {
		existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
		if existingProfile == nil {
			utils.HandleError(writer, fmt.Errorf("profile not found for user: %s", profileSync.UserId))
			return
		}
		err := profilesService.DeleteProfile(existingProfile.ProfileId)
		if err != nil {
			utils.HandleError(writer, fmt.Errorf("failed to delete profile: %w", err))
			return
		}
		return
		// if needed can ensure if profile got created
	}

	if profileSync.Event == "POST_SET_USER_CLAIM_VALUES_WITH_ID" {

		if profileSync.ProfileId == "" && profileSync.UserId != "" {

			existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
			if existingProfile == nil {
				log.GetLogger().Info("creating new profile for user: " + profileSync.UserId)
				identityAttributes := make(map[string]interface{})

				for claimURI, value := range identityClaims {
					attributeKeyPath := extractClaimKeyFromLocalURI(claimURI)
					setNestedMapValue(identityAttributes, attributeKeyPath, value)
				}

				profileRequest := model.ProfileRequest{
					UserId:             profileSync.UserId,
					IdentityAttributes: identityAttributes,
				}
				_, err := profilesService.CreateProfile(profileRequest, tenantId)

				if err != nil {
					return
				}
				return

			} else {
				log.GetLogger().Info("sdfgvf----")
				//existingProfile, err = profilesService.GetProfile(existingProfile.ProfileId)
				// Update identity attributes based on claim URIs
				if existingProfile.IdentityAttributes == nil {
					existingProfile.IdentityAttributes = make(map[string]interface{})
				}

				for claimURI, value := range identityClaims {
					attributeKeyPath := extractClaimKeyFromLocalURI(claimURI)
					setNestedMapValue(existingProfile.IdentityAttributes, attributeKeyPath, value)
				}

				profileRequest := model.ProfileRequest{
					UserId:             existingProfile.UserId,
					IdentityAttributes: existingProfile.IdentityAttributes,
					Traits:             existingProfile.Traits,
					ApplicationData:    existingProfile.ApplicationData,
				}

				// Save updated profile
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, profileRequest)
				if err != nil {
					utils.HandleError(writer, fmt.Errorf("failed to update profile: %w", err))
					return
				}
			}
		} else {
			existingProfile, err = profilesService.GetProfile(profileId)
			// Update identity attributes based on claim URIs
			if existingProfile.IdentityAttributes == nil {
				existingProfile.IdentityAttributes = make(map[string]interface{})
			}

			for claimURI, value := range identityClaims {
				attributeKeyPath := extractClaimKeyFromLocalURI(claimURI)
				setNestedMapValue(existingProfile.IdentityAttributes, attributeKeyPath, value)
			}

			profileRequest := model.ProfileRequest{
				UserId:             existingProfile.UserId,
				IdentityAttributes: existingProfile.IdentityAttributes,
				Traits:             existingProfile.Traits,
				ApplicationData:    existingProfile.ApplicationData,
			}

			// Save updated profile
			_, err = profilesService.UpdateProfile(profileId, profileRequest)
			if err != nil {
				utils.HandleError(writer, fmt.Errorf("failed to update profile: %w", err))
				return
			}
		}
	}

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(`{"status": "updated"}`))
}

func setNestedMapValue(m map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
		} else {
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				next := make(map[string]interface{})
				current[part] = next
				current = next
			}
		}
	}
	// todo: ensure the value type and also try how we merge the values here.
}

func extractClaimKeyFromLocalURI(localURI string) string {
	parts := strings.Split(localURI, "/")
	return parts[len(parts)-1]
}
