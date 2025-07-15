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
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
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
	logger.Info("Fetching all profiles without filters")
	var profiles []model.ProfileResponse
	var err error
	// Build the filter from query params
	queryFilters := r.URL.Query()[constants.Filter] // Slice of filter params

	var filters []string
	for _, f := range queryFilters {
		// Split by " and " to support multiple conditions in a single filter param
		splitFilters := strings.Split(f, " and ")
		for _, sf := range splitFilters {
			sf = strings.TrimSpace(sf)
			if sf != "" {
				filters = append(filters, sf)
			}
		}
	}
	tenantId := utils.ExtractTenantIdFromPath(r)
	profilesProvider := provider.NewProfilesProvider()
	profilesService := profilesProvider.GetProfilesService()
	if len(queryFilters) > 0 {
		profiles, err = profilesService.GetAllProfilesWithFilter(tenantId, filters)
	} else {
		profiles, err = profilesService.GetAllProfiles(tenantId)
	}
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profiles)
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
		Name:     "cdm_profile_id",
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
					attributeKeyPath, err := extractAttributePathFromLocalURI(tenantId, claimURI)
					if err != nil {
						utils.HandleError(writer, fmt.Errorf("failed to extract attribute path from local URI: %w", err))
						return
					}
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
					attributeKeyPath, err := extractAttributePathFromLocalURI(tenantId, claimURI)
					if err != nil {
						utils.HandleError(writer, fmt.Errorf("failed to extract attribute path from local URI: %w", err))
						return
					}
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
					attributeKeyPath, err := extractAttributePathFromLocalURI(tenantId, claimURI)
					if err != nil {
						utils.HandleError(writer, fmt.Errorf("failed to extract attribute path from local URI: %w", err))
						return
					}
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
					attributeKeyPath, err := extractAttributePathFromLocalURI(tenantId, claimURI)
					if err != nil {
						utils.HandleError(writer, fmt.Errorf("failed to extract attribute path from local URI: %w", err))
						return
					}
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
				attributeKeyPath, err := extractAttributePathFromLocalURI(tenantId, claimURI)
				if err != nil {
					utils.HandleError(writer, fmt.Errorf("failed to extract attribute path from local URI: %w", err))
					return
				}
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

// extractAttributePathFromLocalURI extracts the claim key from a local URI.
func extractAttributePathFromLocalURI(tenantId, localURI string) (string, error) {

	profileSchemaService := schemaService.GetProfileSchemaService()
	claim, err := profileSchemaService.GetProfileSchemaAttributeByMappedLocalClaim(tenantId, localURI)
	if err != nil {
		return "", err
	}
	if claim.AttributeId == "" {
		return "", fmt.Errorf("claim not found for local URI: %s", localURI)
	}
	key := strings.TrimPrefix(claim.AttributeName, "identity_attributes.")
	return key, nil
}
