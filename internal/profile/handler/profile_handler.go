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
	"net/http"
	"strings"
	"sync"

	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"

	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/authn"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
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

	err := utils.AuthnAndAuthz(r, "profile:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	profileId := r.PathValue("profileId")
	if profileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: "Invalid path for profile retrieval",
		}, http.StatusNotFound)
		utils.HandleError(w, clientError)
		return
	}
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

	logger := log.GetLogger()
	if err := utils.AuthnAndAuthz(r, "profile:view"); err != nil {
		utils.HandleError(w, err)
		return
	}

	var profileId string
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Try cookie-based resolution first
	cookie, err := r.Cookie(constants.ProfileCookie)
	if err == nil && cookie.Value != "" {
		cookieObj, err := profilesService.GetProfileCookie(cookie.Value)
		if err == nil && cookieObj != nil && cookieObj.IsActive {
			profileId = cookieObj.ProfileId
		}
	}

	// Fallback to token-based resolution
	if profileId == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

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

			// Lookup profile by username (sub)
			profile, err := profilesService.FindProfileByUserId(sub)
			if err != nil || profile == nil {
				http.Error(w, "Profile not found for token subject", http.StatusUnauthorized)
				return
			}
			profileId = profile.ProfileId
		}
	}

	// If still not resolved, unauthorized
	if profileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: "Unauthorized: no valid authentication method found",
		}, http.StatusUnauthorized)
		utils.HandleError(w, clientError)
		return
	}

	// Fetch the profile using the resolved profile ID
	profile, err := profilesService.GetProfile(profileId)
	if err != nil || profile == nil {
		logger.Debug(fmt.Sprintf("Profile not found for profileId: %s", profileId))
		utils.HandleError(w, err)
		return
	}

	// Return profile JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		errMsg := fmt.Sprintf("Failed to encode profile response for profileId: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: "Failed to encode profile response for profileId: " + profileId,
		}, err)
		utils.HandleError(w, serverError)
	}
}

// DeleteProfile handles profile deletion
func (ph *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {

	err := utils.AuthnAndAuthz(r, "profile:delete")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	profileId := r.PathValue("profileId")
	if profileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: "Invalid path for profile deletion",
		}, http.StatusNotFound)
		utils.HandleError(w, clientError)
		return
	}
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	err = profilesService.DeleteProfile(profileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAllProfiles handles profile retrieval with and without filters
func (ph *ProfileHandler) GetAllProfiles(w http.ResponseWriter, r *http.Request) {

	err := utils.AuthnAndAuthz(r, "profile:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
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

	// Add ?userId= filter if present
	userID := r.URL.Query().Get("userId")
	if userID != "" {
		filters = append(filters, fmt.Sprintf("identity_attributes.userid eq %s", userID))
	}

	// Parse selective attributes (e.g., ?attributes=identity_attributes.username,application_data.cart_items)
	requestedAttrs := parseRequestedAttributes(r)

	var profiles []model.ProfileResponse
	if len(filters) > 0 {
		// Fall back to full response if filters are used
		logger.Info("Fetching profiles with filters")
		profiles, err = profilesService.GetAllProfilesWithFilter(tenantId, filters)
	} else {
		logger.Info("Fetching all profiles with requested attributes")
		profiles, err = profilesService.GetAllProfiles(tenantId)
	}

	listResponse := buildProfileListResponse(profiles, requestedAttrs)

	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResponse)
}

func buildProfileListResponse(profiles []model.ProfileResponse, requestedAttrs map[string][]string) []model.ProfileListResponse {

	result := []model.ProfileListResponse{}

	for _, profile := range profiles {
		profileRes := model.ProfileListResponse{
			ProfileId: profile.ProfileId,
			Meta:      profile.Meta,
			UserId:    profile.UserId,
		}

		if requestedAttrs == nil {
			// If no specific attributes requested, return only metadata.
			result = append(result, profileRes)
			continue
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
		}

		// Application Data
		appData := profile.ApplicationData
		if len(appData) > 0 {
			filteredAppData := make(map[string]map[string]interface{})

			if len(requestedAttrs["application_data"]) == 0 {
				filteredAppData = appData
			} else {
				fields := requestedAttrs["application_data"]
				for appKey, appFields := range appData {
					temp := make(map[string]interface{})
					for _, f := range fields {
						if f == "*" {
							temp = appFields
							break
						}
						if val, ok := appFields[f]; ok {
							temp[f] = val
						}
					}
					if len(temp) > 0 {
						filteredAppData[appKey] = temp
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

// InitProfile initializes a new profile based on the request body and sets a cookie
func (ph *ProfileHandler) InitProfile(w http.ResponseWriter, r *http.Request) {

	err := utils.AuthnAndAuthz(r, "profile:create")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	orgId := utils.ExtractTenantIdFromPath(r)
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	var profile model.ProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&profile)

	if err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: utils.HandleDecodeError(err, "profile"),
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}

	// Check if a valid cookie exists
	profileCookie, err := r.Cookie(constants.ProfileCookie)
	if err == nil && profileCookie.Value != "" {
		if ph.handleExistingCookie(w, r, profileCookie.Value) {
			return
		}
	}

	// If no valid cookie, create a new profile and cookie
	profileResponse, err := profilesService.CreateProfile(profile, orgId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	cookie, err := profilesService.CreateProfileCookie(profileResponse.ProfileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	// Create and set a new cookie
	err = setProfileCookie(w, cookie.CookieId, r)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Construct Location header for created resource
	location := fmt.Sprintf("%s://%s%s/profiles/%s",
		detectScheme(r),
		r.Host,
		constants.ApiBasePath+"/v1",
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
			//todo: Revisit this logic
			_, err = profilesService.CreateProfileCookie(profileResponse.ProfileId)
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

func setProfileCookie(w http.ResponseWriter, cookieId string, r *http.Request) error {
	cookie := &http.Cookie{
		Name:     constants.ProfileCookie,
		Value:    cookieId,
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

	err := utils.AuthnAndAuthz(request, "profile:update")
	if err != nil {
		utils.HandleError(writer, err)
		return
	}

	profileId := request.PathValue("profileId")
	if profileId == "" {
		http.Error(writer, "Invalid path", http.StatusNotFound)
		return
	}

	var profile model.ProfileRequest
	err = json.NewDecoder(request.Body).Decode(&profile)
	if err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: utils.HandleDecodeError(err, "profile"),
		}, http.StatusBadRequest)
		utils.HandleError(writer, clientError)
		return
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	tenantId := utils.ExtractTenantIdFromPath(request)
	_, err = profilesService.UpdateProfile(profileId, tenantId, profile)
	if err != nil {
		utils.HandleError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusOK)
	//todo: should we not return the updated profile?
	_, _ = writer.Write([]byte(`{"status": "updated"}`))
}

// PatchProfile handles partial updates to a profile
func (ph *ProfileHandler) PatchProfile(w http.ResponseWriter, r *http.Request) {

	err := utils.AuthnAndAuthz(r, "profile:update")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	profileId := r.PathValue("profileId")
	if profileId == "" {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}

	var patchData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: utils.HandleDecodeError(err, "profile"),
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
	}

	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	tenantId := utils.ExtractTenantIdFromPath(r)
	updatedProfile, err := profilesService.PatchProfile(profileId, tenantId, patchData)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	//todo: should we not return the updated profile?
	err = json.NewEncoder(w).Encode(updatedProfile)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to encode profile response for profileId: %s", profileId)
		log.GetLogger().Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, err)
		utils.HandleError(w, serverError)
	}
}

// PatchCurrentUserProfile handles partial updates to the current user's profile
func (ph *ProfileHandler) PatchCurrentUserProfile(w http.ResponseWriter, r *http.Request) {

	logger := log.GetLogger()
	if err := utils.AuthnAndAuthz(r, "profile:update"); err != nil {
		utils.HandleError(w, err)
		return
	}

	var profileId string
	var err error
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Try cookie-based profileId resolution (preferred)
	cookie, err := r.Cookie(constants.ProfileCookie)
	if err == nil && cookie.Value != "" {
		cookieObj, err := profilesService.GetProfileCookie(cookie.Value)
		if err == nil && cookieObj != nil && cookieObj.IsActive {
			profileId = cookieObj.ProfileId
		}
	}

	// Fallback: token-based flow (get sub → lookup profile → extract profileId)
	if profileId == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

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

			// Lookup profile by sub (username)
			profile, err := profilesService.FindProfileByUserId(sub)
			if err != nil || profile == nil {
				http.Error(w, "Profile not found for token subject", http.StatusUnauthorized)
				return
			}
			profileId = profile.ProfileId
		}
	}

	// If still no profileId, reject
	if profileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: "Invalid authentication method. No valid profileId found.",
		}, http.StatusUnauthorized)
		utils.HandleError(w, clientError)
		return
	}

	// Parse patch payload
	var patchData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: utils.HandleDecodeError(err, "profile"),
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}

	tenantId := utils.ExtractTenantIdFromPath(r)
	// Apply patch
	updatedProfile, err := profilesService.PatchProfile(profileId, tenantId, patchData)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Return updated profile
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedProfile); err != nil {
		errMsg := fmt.Sprintf("Failed to encode profile response for profileId: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, err)
		utils.HandleError(w, serverError)
	}
}

func (ph *ProfileHandler) SyncProfile(writer http.ResponseWriter, request *http.Request) {

	err := utils.AuthnAndAuthz(request, "profile:update")
	if err != nil {
		utils.HandleError(writer, err)
		return
	}

	var profileSync model.ProfileSync
	err = json.NewDecoder(request.Body).Decode(&profileSync)
	if err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: utils.HandleDecodeError(err, "profile"),
		}, http.StatusBadRequest)
		utils.HandleError(writer, clientError)
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
		errMsg := fmt.Sprintf("Tenant id cannot be empty in profile sync event: %s", profileSync.Event)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, http.StatusBadRequest)
		utils.HandleError(writer, clientError)
		return
	}

	var existingProfile *model.ProfileResponse

	if profileSync.Event == "POST_ADD_USER" {
		if profileSync.ProfileId != "" && profileSync.UserId != "" {

			// This scenario is when the user anonymously tried and then trying to signup or login. So profile with profile id exists
			existingProfile, err = profilesService.GetProfile(profileSync.ProfileId)
			if err != nil {
				//todo: decide if we need to write the response back even for fire and forget
				utils.HandleError(writer, err)
				return
			}
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
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, tenantId, profileRequest)
				if err != nil {
					utils.HandleError(writer, err)
					return
				}
				return
			}
			return
		} else if profileSync.ProfileId == "" {
			// this is when we create a profile for a new user created in IS
			existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
			if err != nil {
				utils.HandleError(writer, err)
				return
			}
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
					utils.HandleError(writer, err)
					return
				}
			}
			return
		}
		return
		// if needed can ensure if profile got created
	}

	logger := log.GetLogger()
	if profileSync.Event == "AUTHENTICATION_SUCCESS" {
		logger.Info("Authentication success event received for user: " + profileSync.UserId)
		if profileSync.ProfileId != "" && profileSync.UserId != "" {
			// This scenario is when the user logs in with a profileId existing.
			existingProfile, err = profilesService.GetProfile(profileSync.ProfileId)
			if err != nil {
				utils.HandleError(writer, err)
				return
			}
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
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, tenantId, profileRequest)
				if err != nil {
					utils.HandleError(writer, err)
					return
				}
				return
			}
			return
		}
	}

	if profileSync.Event == "POST_DELETE_USER_WITH_ID" {
		existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
		if err != nil {
			utils.HandleError(writer, err)
			return
		}
		if existingProfile == nil {
			logger.Debug("No profile found for user: " + profileSync.UserId)
			return
		}
		err := profilesService.DeleteProfile(existingProfile.ProfileId)
		if err != nil {
			utils.HandleError(writer, err)
			return
		}
		return
		// if needed can ensure if profile got created
	}

	if profileSync.Event == "POST_SET_USER_CLAIM_VALUES_WITH_ID" {

		if profileSync.ProfileId == "" && profileSync.UserId != "" {

			existingProfile, err = profilesService.FindProfileByUserId(profileSync.UserId)
			if err != nil {
				utils.HandleError(writer, err)
				return
			}
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
				_, err = profilesService.UpdateProfile(existingProfile.ProfileId, tenantId, profileRequest)
				if err != nil {
					utils.HandleError(writer, err)
					return
				}
			}
		} else {
			existingProfile, err = profilesService.GetProfile(profileId)
			if err != nil {
				utils.HandleError(writer, err)
				return
			}
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
			_, err = profilesService.UpdateProfile(profileId, tenantId, profileRequest)
			if err != nil {
				utils.HandleError(writer, err)
				return
			}
		}
	}

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(`{"status": "updated"}`))
}

// GetProfileConsents handles retrieving consents for a specific profile
func (ph *ProfileHandler) GetProfileConsents(w http.ResponseWriter, r *http.Request) {

	profileId := r.PathValue("profileId")
	if profileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_CONSENT.Code,
			Message:     errors2.GET_PROFILE_CONSENT.Message,
			Description: "Invalid path for profile consents retrieval",
		}, http.StatusNotFound)
		utils.HandleError(w, clientError)
		return
	}

	err := utils.AuthnAndAuthz(r, "profile:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Get the profiles provider and service
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Verify profile exists first
	consentRecords, err := profilesService.GetProfileConsents(profileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(consentRecords)
}

// UpdateProfileConsents handles updating consents for a specific profile
func (ph *ProfileHandler) UpdateProfileConsents(w http.ResponseWriter, r *http.Request) {

	profileId := r.PathValue("profileId")
	if profileId == "" {
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}

	err := utils.AuthnAndAuthz(r, "profile:update")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Get the profiles provider and service
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()

	// Verify profile exists first
	_, err = profilesService.GetProfile(profileId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	// Parse the request body
	var consentUpdate []model.ConsentRecord
	err = json.NewDecoder(r.Body).Decode(&consentUpdate)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = profilesService.UpdateProfileConsents(profileId, consentUpdate)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(consentUpdate)
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
