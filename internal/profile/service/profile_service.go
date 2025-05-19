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

package service

import (
	"fmt"
	repositories "github.com/wso2/identity-customer-data-service/internal/events/store"
	"github.com/wso2/identity-customer-data-service/internal/system/database/client"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	eventModel "github.com/wso2/identity-customer-data-service/internal/events/model"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
)

type ProfilesServiceInterface interface {
	DeleteProfile(profileId string) error
	GetAllProfiles() ([]profileModel.Profile, error)
	CreateOrUpdateProfile(event eventModel.Event) error
	GetProfile(profileId string) (*profileModel.Profile, error)
	WaitForProfile(profileID string, maxRetries int, retryDelay time.Duration) (*profileModel.Profile, error)
	GetAllProfilesWithFilter(filters []string) ([]profileModel.Profile, error)
}

// ProfilesService is the default implementation of the ProfilesServiceInterface.
type ProfilesService struct{}

// GetProfilesService creates a new instance of EventsService.
func GetProfilesService() ProfilesServiceInterface {

	return &ProfilesService{}
}

func (ps *ProfilesService) CreateOrUpdateProfile(event eventModel.Event) error {

	// todo: should we throw an error here at all?

	// Create a lock tied to this connection
	lock := client.NewPostgresLock()
	lockIdentifier := event.ProfileId

	//  Attempt to acquire the lock with retry
	var acquired bool
	var err error
	logger := log.GetLogger()
	for i := 0; i < constants.MaxRetryAttempts; i++ {
		acquired, err = lock.Acquire(lockIdentifier)
		if err != nil {
			logger.Error(fmt.Sprintf("Error acquiring lock for %s: %v", event.ProfileId, err))
			// todo: should we throw an error here?
		}
		if acquired {
			break
		}
		time.Sleep(constants.RetryDelay)
	}
	if !acquired {
		// todo: should we throw an error here?
		logger.Error(fmt.Sprintf("Failed to acquire lock for %s after %d attempts", event.ProfileId, constants.MaxRetryAttempts))
	}
	defer func() {
		_ = lock.Release(lockIdentifier) //  Always attempt to release
	}()

	//  Insert/update using standard DB (does not have to use same conn unless needed)
	profileToUpsert := profileModel.Profile{
		ProfileId: event.ProfileId,
		ProfileHierarchy: &profileModel.ProfileHierarchy{
			IsParent:    true,
			ListProfile: true,
		},
		IdentityAttributes: make(map[string]interface{}),
		Traits:             make(map[string]interface{}),
		ApplicationData:    []profileModel.ApplicationData{},
	}

	if err := profileStore.InsertProfile(profileToUpsert); err != nil {
		return err
	}

	profileFetched, errWait := ps.WaitForProfile(event.ProfileId, constants.MaxRetryAttempts, constants.RetryDelay)
	if errWait != nil || profileFetched == nil {
		logger.Warn(fmt.Sprintf("Profile: %s not visible after insert/update: %v", event.ProfileId, errWait))
		// todo: should we throw an error here?
		return nil
	}

	return nil
}

// GetProfile retrieves a profile
func (ps *ProfilesService) GetProfile(ProfileId string) (*profileModel.Profile, error) {

	profile, err := profileStore.GetProfile(ProfileId)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrProfileNotFound.Code,
			Message:     errors2.ErrProfileNotFound.Message,
			Description: errors2.ErrProfileNotFound.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}

	if profile.ProfileHierarchy.IsParent {
		return profile, nil
	} else {
		// fetching merged master profile
		masterProfile, err := profileStore.GetProfile(profile.ProfileHierarchy.ParentProfileID)
		// todo: app context should be restricted for apps that is requesting these

		if err != nil {
			return nil, err
		}
		masterProfile.ApplicationData, err = profileStore.FetchApplicationData(masterProfile.ProfileId)

		if err != nil {
			return nil, err
		}

		// building the hierarchy
		masterProfile.ProfileHierarchy.ChildProfiles, err = profileStore.FetchChildProfiles(masterProfile.ProfileId)
		masterProfile.ProfileHierarchy.ParentProfileID = masterProfile.ProfileId
		masterProfile.ProfileId = profile.ProfileId

		if err != nil {
			return nil, err
		}
		return masterProfile, nil
	}
}

// DeleteProfile removes a profile from MongoDB by `perma_id`
func (ps *ProfilesService) DeleteProfile(ProfileId string) error {

	// Fetch the existing profile before deletion
	profile, err := profileStore.GetProfile(ProfileId)
	logger := log.GetLogger()
	if profile == nil {
		logger.Warn(fmt.Sprintf("Profile with profile_id: %s that is requested for deletion is not found",
			ProfileId))
		return nil
	}
	if err != nil {
		errorMsg := fmt.Sprintf("Error deleting profile with profile_id: %s", ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	//  Delete related events
	if err := repositories.DeleteEventsByProfileId(ProfileId); err != nil {
		errorMsg := fmt.Sprintf("Error deleting events for the profile with profile_id: %s", ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	if profile.ProfileHierarchy.IsParent {
		// fetching the child if its parent
		profile.ProfileHierarchy.ChildProfiles, _ = profileStore.FetchChildProfiles(profile.ProfileId)
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) == 0 {
		// Delete the parent with no children
		err = profileStore.DeleteProfile(ProfileId)
		if err != nil {
			errorMsg := fmt.Sprintf("Error deleting profile with profile_id: %s which is a parent and no children", ProfileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.DELETE_PROFILE.Code,
				Message:     errors2.DELETE_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		return nil
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) > 0 {
		//get all child profiles and delete
		for _, childProfile := range profile.ProfileHierarchy.ChildProfiles {
			profile, err := profileStore.GetProfile(childProfile.ChildProfileId)
			if profile == nil {
				errorMsg := fmt.Sprintf("Child profile with profile_id: %s that is being deleted is not found",
					childProfile.ChildProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting Child profile with profile_id: %s that is being deleted is not found",
					childProfile.ChildProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			err = profileStore.DeleteProfile(childProfile.ChildProfileId)
			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting profile with profile_id: %s ", childProfile.ChildProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
		}
		// now delete master
		err = profileStore.DeleteProfile(ProfileId)
		if err != nil {
			errorMsg := fmt.Sprintf("Error while deleting parent profile: %s ", ProfileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.DELETE_PROFILE.Code,
				Message:     errors2.DELETE_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		return nil
	}

	// If it is a child profile, delete it
	if !(profile.ProfileHierarchy.IsParent) {
		parentProfile, err := profileStore.GetProfile(profile.ProfileHierarchy.ParentProfileID)
		if err != nil {
			errorMsg := fmt.Sprintf("Error while deleting the child profile: %s ", ProfileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.DELETE_PROFILE.Code,
				Message:     errors2.DELETE_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
		parentProfile.ProfileHierarchy.ChildProfiles, _ = profileStore.FetchChildProfiles(parentProfile.ProfileId)

		if len(parentProfile.ProfileHierarchy.ChildProfiles) == 1 {
			// delete the parent as this is the only child
			err = profileStore.DeleteProfile(profile.ProfileHierarchy.ParentProfileID)
			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting the master profile: %s ", ProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			err = profileStore.DeleteProfile(ProfileId)
			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting the  profile: %s ", ProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
		} else {
			err = profileStore.DetachChildProfileFromParent(profile.ProfileHierarchy.ParentProfileID, ProfileId)
			if err != nil {
				errorMsg := fmt.Sprintf("Error while current profile from parent: %s ", ProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
			err = profileStore.DeleteProfile(ProfileId)
			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting the current profile: %s ", ProfileId)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.DELETE_PROFILE.Code,
					Message:     errors2.DELETE_PROFILE.Message,
					Description: errorMsg,
				}, err)
				return serverError
			}
		}

	}

	return nil
}

func (ps *ProfilesService) WaitForProfile(profileID string, maxRetries int, retryDelay time.Duration) (*profileModel.Profile, error) {

	var profile *profileModel.Profile
	var lastErr error
	logger := log.GetLogger()
	for i := 0; i < maxRetries; i++ {
		if i > 0 { // Only sleep on subsequent retries
			time.Sleep(retryDelay)
		}
		profile, lastErr = profileStore.GetProfile(profileID) // Assuming GetProfile is a method on profileRepo
		if profile != nil {
			return profile, nil
		}
		if lastErr != nil {
			logger.Error(fmt.Sprintf("Error fetching profile : %s", profileID))
		}
	}

	// logger.Error("waitForProfile: Profile not visible after all retries", "profileId", profileID, "attempts", maxRetries)
	if lastErr != nil {
		return nil, fmt.Errorf("profile %s not visible after %d retries, last error: %w", profileID, maxRetries, lastErr)
	}
	return nil, fmt.Errorf("profile %s not visible after %d retries", profileID, maxRetries)
}

// GetAllProfiles retrieves all profiles
func (ps *ProfilesService) GetAllProfiles() ([]profileModel.Profile, error) {

	existingProfiles, err := profileStore.GetAllProfiles()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		return []profileModel.Profile{}, nil
	}

	// todo: app context should be restricted for apps that is requesting these

	var result []profileModel.Profile
	for _, profile := range existingProfiles {
		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and assign current profile ID
			master, err := profileStore.GetProfile(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}

			master.ApplicationData, _ = profileStore.FetchApplicationData(master.ProfileId)

			// building the hierarchy
			master.ProfileHierarchy.ChildProfiles, _ = profileStore.FetchChildProfiles(master.ProfileId)
			master.ProfileId = profile.ProfileId
			master.ProfileHierarchy.ParentProfileID = master.ProfileId

			result = append(result, *master)
		}
	}
	return result, nil
}

// GetAllProfilesWithFilter handles fetching all profiles with filter
func (ps *ProfilesService) GetAllProfilesWithFilter(filters []string) ([]profileModel.Profile, error) {

	// Step 1: Fetch enrichment rules to extract value types
	rules, err := store.GetProfileEnrichmentRules()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfileEnrichmentRules, err)
	}

	// Step 2: Build field → valueType mapping
	propertyTypeMap := make(map[string]string)
	for _, rule := range rules {
		propertyTypeMap[rule.PropertyName] = rule.ValueType
	}

	// Step 3: Rewrite filters using typed conversion
	var rewrittenFilters []string
	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}

		field, operator, rawValue := parts[0], parts[1], parts[2]
		valueType := propertyTypeMap[field]
		typedVal := parseTypedValueForFilters(valueType, rawValue)

		var valueStr string
		switch v := typedVal.(type) {
		case string:
			valueStr = v
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		rewrittenFilters = append(rewrittenFilters, fmt.Sprintf("%s %s %s", field, operator, valueStr))
	}

	// Step 4: Fetch matching profiles with `list_profile = true`
	filteredProfiles, err := profileStore.GetAllProfilesWithFilter(rewrittenFilters)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if filteredProfiles == nil {
		filteredProfiles = []profileModel.Profile{}
	}

	var result []profileModel.Profile
	for _, profile := range filteredProfiles {
		if !profile.ProfileHierarchy.ListProfile {
			continue
		}

		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and attach current profile context
			master, err := profileStore.GetProfile(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}

			master.ApplicationData, _ = profileStore.FetchApplicationData(master.ProfileId)
			master.ProfileHierarchy.ChildProfiles, _ = profileStore.FetchChildProfiles(master.ProfileId)

			// Override for visual reference to the child
			master.ProfileId = profile.ProfileId
			master.ProfileHierarchy.ParentProfileID = profile.ProfileId

			result = append(result, *master)
		}
	}

	return result, nil
}

func parseTypedValueForFilters(valueType string, raw string) interface{} {
	switch valueType {
	case "int":
		i, _ := strconv.Atoi(raw)
		return i
	case "float", "double":
		f, _ := strconv.ParseFloat(raw, 64)
		return f
	case "boolean":
		return raw == "true"
	case "string":
		return raw
	default:
		return raw
	}
}
