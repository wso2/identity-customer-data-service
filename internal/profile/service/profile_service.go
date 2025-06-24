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
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	//schemaStore "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"net/http"
	"strconv"
	"strings"
	"time"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	UnificationModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

type ProfilesServiceInterface interface {
	DeleteProfile(profileId string) error
	GetAllProfiles(tenantId string) ([]profileModel.ProfileResponse, error)
	CreateProfile(profile profileModel.ProfileRequest, tenantId string) (profileModel.ProfileResponse, error)
	UpdateProfile(profileId string, update profileModel.ProfileRequest) (profileModel.ProfileResponse, error)
	GetProfile(profileId string) (*profileModel.ProfileResponse, error)
	GetAllProfilesWithFilter(tenantId string, filters []string) ([]profileModel.ProfileResponse, error)
}

// ProfilesService is the default implementation of the ProfilesServiceInterface.
type ProfilesService struct{}

// GetProfilesService creates a new instance of EventsService.
func GetProfilesService() ProfilesServiceInterface {

	return &ProfilesService{}
}

func ConvertAppData(input map[string]map[string]interface{}) []profileModel.ApplicationData {

	appDataList := make([]profileModel.ApplicationData, 0, len(input))

	for appID, data := range input {
		appSpecific := make(map[string]interface{})
		for key, value := range data {
			appSpecific[key] = value
		}
		appDataList = append(appDataList, profileModel.ApplicationData{
			AppId:           appID,
			AppSpecificData: appSpecific,
		})
	}

	return appDataList
}

// CreateProfile creates a new profile.
func (ps *ProfilesService) CreateProfile(profileRequest profileModel.ProfileRequest, tenantId string) (profileModel.ProfileResponse, error) {

	rawSchema, err := schemaService.GetProfileSchemaService().GetProfileSchema(tenantId)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error fetching profile schema for tenant: %s", tenantId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errMsg,
		}, err)
		return profileModel.ProfileResponse{}, serverError
	}

	var schema model.ProfileSchema
	schemaBytes, _ := json.Marshal(rawSchema) // serialize
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		errMsg := fmt.Sprintf("Invalid schema format for tenant: %s while validating for profile creation.", tenantId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errMsg,
		}, err)
		return profileModel.ProfileResponse{}, serverError
	}

	err = ValidateProfileAgainstSchema(profileRequest, profileModel.Profile{}, schema, false)
	if err != nil {
		return profileModel.ProfileResponse{}, err
	}

	// convert profile request to model
	createdTime := time.Now().UTC().Unix()
	profileId := uuid.New().String()
	profile := profileModel.Profile{
		ProfileId:          profileId,
		TenantId:           tenantId,
		UserId:             profileRequest.UserId,
		ApplicationData:    ConvertAppData(profileRequest.ApplicationData),
		Traits:             profileRequest.Traits,
		IdentityAttributes: profileRequest.IdentityAttributes,
		ProfileStatus: &profileModel.ProfileStatus{
			IsReferenceProfile: true,
			ListProfile:        true,
		},
		CreatedAt: createdTime,
		UpdatedAt: createdTime,
		Location:  utils.BuildProfileLocation(tenantId, profileId),
	}

	if err := profileStore.InsertProfile(profile); err != nil {
		logger.Debug(fmt.Sprintf("Error insertinng profile: %s", profile.ProfileId), log.Error(err))
		return profileModel.ProfileResponse{}, err
	}
	profileFetched, errWait := ps.GetProfile(profileId)
	if errWait != nil || profileFetched == nil {
		logger.Warn(fmt.Sprintf("Profile: %s not available after insertion: %v", profile.ProfileId, errWait))
		return profileModel.ProfileResponse{}, errWait
	}

	queue := &workers.ProfileWorkerQueue{}

	config := UnificationModel.DefaultConfig()

	if config.ProfileUnificationTrigger.TriggerType == constants.SyncProfileOnUpdate {
		queue.Enqueue(profile)
	}
	logger.Info("Profile available after insert/update: " + profileFetched.ProfileId)
	return *profileFetched, nil
}

func ValidateProfileAgainstSchema(profile profileModel.ProfileRequest, existingProfile profileModel.Profile,
	schema model.ProfileSchema, isUpdate bool) error {

	// Validate identity attributes
	for key, val := range profile.IdentityAttributes {
		attrName := "identity_attributes." + key
		attr, found := findAttributeInSchema(schema.IdentityAttributes, attrName)
		if !found {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("identity attribute '%s' not defined in schema", attrName),
			}, http.StatusBadRequest)
			return clientError
		}
		if isUpdate && existingProfile.IdentityAttributes != nil {
			if err := validateMutability(attr.Mutability, isUpdate, existingProfile.IdentityAttributes[key], val); err != nil {
				return err
			}
		} else {
			if err := validateMutability(attr.Mutability, isUpdate, nil, val); err != nil {
				return err
			}
		}
		if !isValidType(val, attr.ValueType) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("identity attribute '%s': type mismatch", key),
			}, http.StatusBadRequest)
			return clientError
		}
	}

	// Validate traits
	for key, val := range profile.Traits {
		attrName := "traits." + key
		attr, found := findAttributeInSchema(schema.Traits, attrName)
		if !found {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("trait '%s' not defined in schema", attrName),
			}, http.StatusBadRequest)
			return clientError
		}
		if isUpdate && existingProfile.Traits != nil {
			if err := validateMutability(attr.Mutability, isUpdate, existingProfile.Traits[key], val); err != nil {
				return err
			}
		} else {
			if err := validateMutability(attr.Mutability, isUpdate, nil, val); err != nil {
				return err
			}
		}
		if !isValidType(val, attr.ValueType) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("trait '%s': type mismatch", key),
			}, http.StatusBadRequest)
			return clientError
		}
	}

	// Validate application data
	for appID, attrs := range profile.ApplicationData {
		for key, val := range attrs {
			attrName := "application_data." + key
			attr, found := findAppAttributeInSchema(schema.ApplicationData, appID, attrName)
			if !found {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_PROFILE.Code,
					Message:     errors2.UPDATE_PROFILE.Message,
					Description: fmt.Sprintf("application_data '%s.%s' not defined in schema", appID, key),
				}, http.StatusBadRequest)
				return clientError
			}

			var existingVal interface{}
			if isUpdate {
				existingVal, _ = getAppDataValue(existingProfile.ApplicationData, appID, key)
			}

			if err := validateMutability(attr.Mutability, isUpdate, existingVal, val); err != nil {
				return err
			}

			if !isValidType(val, attr.ValueType) {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_PROFILE.Code,
					Message:     errors2.UPDATE_PROFILE.Message,
					Description: fmt.Sprintf("application_data '%s.%s': type mismatch", appID, key),
				}, http.StatusBadRequest)
				return clientError
			}
		}
	}

	return nil
}

func getAppDataValue(appDataList []profileModel.ApplicationData, appID, key string) (interface{}, bool) {
	for _, appData := range appDataList {
		if appData.AppId == appID {
			val, ok := appData.AppSpecificData[key]
			return val, ok
		}
	}
	return nil, false
}

func findAttributeInSchema(attrs []model.ProfileSchemaAttribute, name string) (model.ProfileSchemaAttribute, bool) {
	for _, attr := range attrs {
		if attr.AttributeName == name {
			return attr, true
		}
	}
	return model.ProfileSchemaAttribute{}, false
}

func findAppAttributeInSchema(appSchema map[string][]model.ProfileSchemaAttribute, appId, name string) (model.ProfileSchemaAttribute, bool) {
	attrs, ok := appSchema[appId]
	if !ok {
		return model.ProfileSchemaAttribute{}, false
	}
	for _, attr := range attrs {
		if attr.AttributeName == name {
			return attr, true
		}
	}
	return model.ProfileSchemaAttribute{}, false
}

func validateMutability(mutability string, isUpdate bool, oldVal, newVal interface{}) error {
	switch mutability {
	case constants.MutabilityReadOnly:
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: "field is read-only or computed",
		}, http.StatusBadRequest)
	case constants.MutabilityImmutable:
		if isUpdate && oldVal != newVal {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: "immutable field cannot be updated",
			}, http.StatusBadRequest)
		}
	case constants.MutabilityWriteOnce:
		if isUpdate && oldVal != nil && oldVal != "" && oldVal != newVal {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: "write-once field cannot be changed after being set",
			}, http.StatusBadRequest)
		}
	case constants.MutabilityReadWrite, constants.MutabilityWriteOnly:
		return nil
	default:
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: fmt.Sprintf("unknown mutability: %s\", mutability"),
		}, http.StatusBadRequest)
	}
	return nil
}

func isValidType(value interface{}, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "int":
		_, ok1 := value.(int)
		_, ok2 := value.(float64)
		return ok1 || ok2
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "date":
		_, ok := value.(string) // optionally parse date
		return ok
	case "arrayOfString":
		arr, ok := value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range arr {
			if _, ok := v.(string); !ok {
				return false
			}
		}
		return true
	case "arrayOfInt":
		arr, ok := value.([]interface{})
		if !ok {
			return false
		}
		for _, v := range arr {
			if _, ok := v.(float64); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// UpdateProfile creates or updates a profile
func (ps *ProfilesService) UpdateProfile(profileId string, updatedProfile profileModel.ProfileRequest) (profileModel.ProfileResponse, error) {

	profile, err := profileStore.GetProfile(profileId) //todo: need to get the reference to see what to updatedProfile (see if its the master)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error fetching profile for updatedProfile: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, err)
		return profileModel.ProfileResponse{}, serverError
	}

	if profile == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return profileModel.ProfileResponse{}, clientError
	}

	rawSchema, err := schemaService.GetProfileSchemaService().GetProfileSchema(profile.TenantId)
	if err != nil {
		return profileModel.ProfileResponse{}, err
	}

	// Convert map[string]interface{} → model.ProfileSchema
	var schema model.ProfileSchema
	schemaBytes, _ := json.Marshal(rawSchema) // serialize
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return profileModel.ProfileResponse{}, fmt.Errorf("invalid schema format: %w", err)
	}

	err = ValidateProfileAgainstSchema(updatedProfile, *profile, schema, true)
	if err != nil {
		return profileModel.ProfileResponse{}, err
	}

	var profileToUpDate profileModel.Profile
	updatedTime := time.Now().UTC().Unix()
	if profile.ProfileStatus.IsReferenceProfile {
		// convert profile request to model
		profileToUpDate = profileModel.Profile{
			ProfileId:          profileId,
			UserId:             updatedProfile.UserId,
			ApplicationData:    ConvertAppData(updatedProfile.ApplicationData),
			Traits:             updatedProfile.Traits,
			IdentityAttributes: updatedProfile.IdentityAttributes,
			UpdatedAt:          updatedTime,
			CreatedAt:          profile.CreatedAt,
			Location:           profile.Location,
			ProfileStatus:      profile.ProfileStatus,
		}
	} else {
		// If it is a child profile, we need to update the master profile
		masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
		if err != nil {
			errMsg := fmt.Sprintf("Error fetching master profile for updatedProfile: %s", profile.ProfileId)
			logger.Debug(errMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: errMsg,
			}, err)
			return profileModel.ProfileResponse{}, serverError
		}

		profileToUpDate = profileModel.Profile{
			ProfileId:          masterProfile.ProfileId,
			UserId:             updatedProfile.UserId,
			ApplicationData:    ConvertAppData(updatedProfile.ApplicationData),
			Traits:             updatedProfile.Traits,
			IdentityAttributes: updatedProfile.IdentityAttributes,
			UpdatedAt:          updatedTime,
			CreatedAt:          masterProfile.CreatedAt,
			Location:           masterProfile.Location,
			ProfileStatus:      masterProfile.ProfileStatus,
		}
	}

	if err := profileStore.UpdateProfile(profileToUpDate); err != nil {
		logger.Error(fmt.Sprintf("Error inserting/updating profile: %s", profile.ProfileId), log.Error(err))
		return profileModel.ProfileResponse{}, err
	}

	profileFetched, errWait := ps.GetProfile(profile.ProfileId)
	if errWait != nil || profileFetched == nil {
		logger.Warn(fmt.Sprintf("Profile: %s not visible after insert/updatedProfile: %v", profile.ProfileId, errWait))
		// todo: should we throw an error here?
		return profileModel.ProfileResponse{}, errWait
	}

	config := UnificationModel.DefaultConfig()
	queue := &workers.ProfileWorkerQueue{}
	if config.ProfileUnificationTrigger.TriggerType == constants.SyncProfileOnUpdate {
		queue.Enqueue(profileToUpDate)
	}
	logger.Info("Successfully updated profile: " + profileFetched.ProfileId)
	return *profileFetched, nil
}

// ProfileUnificationQueue is an interface for the profile unification queue.
type ProfileUnificationQueue interface {
	Enqueue(profile profileModel.Profile)
}

func ConvertAppDataToMap(appDataList []profileModel.ApplicationData) map[string]map[string]interface{} {
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

// GetProfile retrieves a profile
func (ps *ProfilesService) GetProfile(ProfileId string) (*profileModel.ProfileResponse, error) {

	profile, err := profileStore.GetProfile(ProfileId)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}

	if profile.ProfileStatus.IsReferenceProfile {

		alias, err := profileStore.FetchProfilesThatAreReferenced(ProfileId)

		if err != nil {
			errorMsg := fmt.Sprintf("Error fetching references for profile: %s", ProfileId)
			logger := log.GetLogger()
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		if len(alias) == 0 {
			alias = nil
		}

		profileResponse := &profileModel.ProfileResponse{
			ProfileId:          profile.ProfileId,
			UserId:             profile.UserId,
			ApplicationData:    ConvertAppDataToMap(profile.ApplicationData),
			Traits:             profile.Traits,
			IdentityAttributes: profile.IdentityAttributes,
			Meta: profileModel.Meta{
				CreatedAt: profile.CreatedAt,
				UpdatedAt: profile.UpdatedAt,
				Location:  profile.Location,
			},
			MergedFrom: alias,
		}
		return profileResponse, nil
	} else {
		// fetching merged master profile
		masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
		// todo: app context should be restricted for apps that is requesting these

		if err != nil {
			return nil, err
		}
		if masterProfile != nil {
			masterProfile.ApplicationData, err = profileStore.FetchApplicationData(masterProfile.ProfileId)

			alias := &profileModel.Reference{
				ProfileId: profile.ProfileStatus.ReferenceProfileId,
				Reason:    profile.ProfileStatus.ReferenceReason,
			}

			profileResponse := &profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             masterProfile.UserId,
				ApplicationData:    ConvertAppDataToMap(masterProfile.ApplicationData),
				Traits:             masterProfile.Traits,
				IdentityAttributes: masterProfile.IdentityAttributes,
				Meta: profileModel.Meta{
					CreatedAt: masterProfile.CreatedAt,
					UpdatedAt: masterProfile.UpdatedAt,
					Location:  masterProfile.Location,
				},
				MergedTo: *alias,
			}

			return profileResponse, nil
		}
		return nil, err
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

	if profile.ProfileStatus.IsReferenceProfile {
		// fetching the child if its parent
		profile.ProfileStatus.References, _ = profileStore.FetchProfilesThatAreReferenced(profile.ProfileId)
	}

	if profile.ProfileStatus.IsReferenceProfile && len(profile.ProfileStatus.References) == 0 {
		logger.Info(fmt.Sprintf("Deleting parent profile: %s with no children", ProfileId))
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

	if profile.ProfileStatus.IsReferenceProfile && len(profile.ProfileStatus.References) > 0 {
		//get all child profiles and delete
		for _, childProfile := range profile.ProfileStatus.References {
			//	profile, err := profileStore.GetProfile(childProfile.ProfileId)
			//	if profile == nil {
			//		errorMsg := fmt.Sprintf("Child profile with profile_id: %s that is being deleted is not found",
			//			childProfile.ProfileId)
			//		logger.Debug(errorMsg, log.Error(err))
			//		serverError := errors2.NewServerError(errors2.ErrorMessage{
			//			Code:        errors2.DELETE_PROFILE.Code,
			//			Message:     errors2.DELETE_PROFILE.Message,
			//			Description: errorMsg,
			//		}, err)
			//		return serverError
			//	}
			//	if err != nil {
			//		errorMsg := fmt.Sprintf("Error while deleting Child profile with profile_id: %s that is being deleted is not found",
			//			childProfile.ProfileId)
			//		logger.Debug(errorMsg, log.Error(err))
			//		serverError := errors2.NewServerError(errors2.ErrorMessage{
			//			Code:        errors2.DELETE_PROFILE.Code,
			//			Message:     errors2.DELETE_PROFILE.Message,
			//			Description: errorMsg,
			//		}, err)
			//		return serverError
			//	}
			err = profileStore.DeleteProfile(childProfile.ProfileId)
			logger.Info(fmt.Sprintf("Deleting child  profile: %s with of parent: %s",
				childProfile.ProfileId, ProfileId))

			if err != nil {
				errorMsg := fmt.Sprintf("Error while deleting profile with profile_id: %s ", childProfile.ProfileId)
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
		logger.Info(fmt.Sprintf("Deleting parent profile: %s with children", ProfileId))
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
	if !(profile.ProfileStatus.IsReferenceProfile) {

		logger.Info(fmt.Sprintf("Deleting child profile: %s with parent: %s", ProfileId,
			profile.ProfileStatus.ReferenceProfileId))
		parentProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
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
		parentProfile.ProfileStatus.References, _ = profileStore.FetchProfilesThatAreReferenced(parentProfile.ProfileId)

		if len(parentProfile.ProfileStatus.References) == 1 {
			// delete the parent as this is the only child
			logger.Info(fmt.Sprintf("Deleting parent profile: %s with of current : %s",
				profile.ProfileStatus.ReferenceProfileId, ProfileId))
			err = profileStore.DeleteProfile(profile.ProfileStatus.ReferenceProfileId)
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
			logger.Info(fmt.Sprintf("Deleted current profile: %s with parent: %s", ProfileId,
				profile.ProfileStatus.ReferenceProfileId))
		} else {
			err = profileStore.DetachRefererProfileFromReference(profile.ProfileStatus.ReferenceProfileId, ProfileId)
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
			logger.Debug(fmt.Sprintf("Detaching current profile: %s from parent: %s", ProfileId,
				profile.ProfileStatus.ReferenceProfileId))
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
			logger.Info(fmt.Sprintf("Deleted current profile: %s with parent: %s",
				ProfileId, profile.ProfileStatus.ReferenceProfileId))
		}

	}

	return nil
}

// GetAllProfiles retrieves all profiles
func (ps *ProfilesService) GetAllProfiles(tenantId string) ([]profileModel.ProfileResponse, error) {

	existingProfiles, err := profileStore.GetAllProfiles(tenantId)
	if err != nil {
		return nil, err
	}
	if existingProfiles == nil {
		return []profileModel.ProfileResponse{}, nil
	}

	// todo: app context should be restricted for apps that is requesting these

	var result []profileModel.ProfileResponse
	for _, profile := range existingProfiles {
		if profile.ProfileStatus.IsReferenceProfile {
			profileResponse := &profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             profile.UserId,
				ApplicationData:    ConvertAppDataToMap(profile.ApplicationData),
				Traits:             profile.Traits,
				IdentityAttributes: profile.IdentityAttributes,
				Meta: profileModel.Meta{
					CreatedAt: profile.CreatedAt,
					UpdatedAt: profile.UpdatedAt,
					Location:  profile.Location,
				},
			}
			result = append(result, *profileResponse)
		} else {
			// Fetch master and assign current profile ID
			masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
			if err != nil || masterProfile == nil {
				continue
			}

			masterProfile.ApplicationData, _ = profileStore.FetchApplicationData(masterProfile.ProfileId)

			// building the hierarchy
			masterProfile.ProfileStatus.References, _ = profileStore.FetchProfilesThatAreReferenced(masterProfile.ProfileId)

			profileResponse := &profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             masterProfile.UserId,
				ApplicationData:    ConvertAppDataToMap(masterProfile.ApplicationData),
				Traits:             masterProfile.Traits,
				IdentityAttributes: masterProfile.IdentityAttributes,
				Meta: profileModel.Meta{
					CreatedAt: masterProfile.CreatedAt,
					UpdatedAt: masterProfile.UpdatedAt,
					Location:  masterProfile.Location,
				},
				MergedTo: profileModel.Reference{
					ProfileId: masterProfile.ProfileId,
					Reason:    profile.ProfileStatus.ReferenceReason,
				},
			}

			result = append(result, *profileResponse)
		}
	}
	return result, nil
}

// GetAllProfilesWithFilter handles fetching all profiles with filter
func (ps *ProfilesService) GetAllProfilesWithFilter(tenantId string, filters []string) ([]profileModel.ProfileResponse, error) {

	// Step 1: Fetch enrichment rules to extract value types
	//rules, err := pss.GetProfileSchemaService()
	//if err != nil {
	//	return nil, err
	//}
	//
	//// Step 2: Build field → valueType mapping
	propertyTypeMap := make(map[string]string)
	//for _, rule := range rules {
	//	propertyTypeMap[rule.PropertyName] = rule.ValueType
	//}

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
	filteredProfiles, err := profileStore.GetAllProfilesWithFilter(tenantId, rewrittenFilters)
	if err != nil {
		return nil, err
	}
	if filteredProfiles == nil {
		filteredProfiles = []profileModel.Profile{}
	}

	var result []profileModel.ProfileResponse
	for _, profile := range filteredProfiles {
		if profile.ProfileStatus.IsReferenceProfile {
			profileResponse := &profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             profile.UserId,
				ApplicationData:    ConvertAppDataToMap(profile.ApplicationData),
				Traits:             profile.Traits,
				IdentityAttributes: profile.IdentityAttributes,
				Meta: profileModel.Meta{
					CreatedAt: profile.CreatedAt,
					UpdatedAt: profile.UpdatedAt,
					Location:  profile.Location,
				},
			}
			result = append(result, *profileResponse)
		} else {
			// Fetch master and attach current profile context
			masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
			if err != nil || masterProfile == nil {
				continue
			}

			masterProfile.ApplicationData, _ = profileStore.FetchApplicationData(masterProfile.ProfileId)
			masterProfile.ProfileStatus.References, _ = profileStore.FetchProfilesThatAreReferenced(masterProfile.ProfileId)

			// Override for visual reference to the child
			masterProfile.ProfileId = profile.ProfileId
			masterProfile.ProfileStatus.ReferenceProfileId = profile.ProfileId

			profileResponse := &profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             masterProfile.UserId,
				ApplicationData:    ConvertAppDataToMap(masterProfile.ApplicationData),
				Traits:             masterProfile.Traits,
				IdentityAttributes: masterProfile.IdentityAttributes,
				Meta: profileModel.Meta{
					CreatedAt: masterProfile.CreatedAt,
					UpdatedAt: masterProfile.UpdatedAt,
					Location:  masterProfile.Location,
				},
				MergedTo: profileModel.Reference{
					ProfileId: masterProfile.ProfileId,
					Reason:    profile.ProfileStatus.ReferenceReason, //todo: this has to be fetch from db
				},
			}

			result = append(result, *profileResponse)
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

// FindProfileByUserName retrieves a profile by user_id
func FindProfileByUserName(tenantId, sub string) (interface{}, error) {

	// TODO: Restrict app-specific fields via client_id from JWT (if available)
	//  TODO: currently userId is defined as a string [] so CONTAINS - but need to decide
	filter := fmt.Sprintf("identity_attributes.user_id co %s", sub)
	profiles, err := profileStore.GetAllProfilesWithFilter(tenantId, []string{filter})
	logger := log.GetLogger()
	if err != nil {
		logger.Debug(fmt.Sprintf("Error fetching profile by user_id:%s ", sub), log.Error(err))
		return nil, fmt.Errorf("failed to query profiles: %w", err)
	}

	if len(profiles) == 0 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}

	//  Track unique parent profile IDs
	parentProfileIDSet := make(map[string]struct{})

	for _, profile := range profiles {
		if profile.ProfileStatus.IsReferenceProfile {
			parentProfileIDSet[profile.ProfileId] = struct{}{}
		} else {
			parentProfileIDSet[profile.ProfileStatus.ReferenceProfileId] = struct{}{}
		}
	}

	if len(parentProfileIDSet) > 1 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.MULTIPLE_PROFILE_FOUND.Code,
			Message:     errors2.MULTIPLE_PROFILE_FOUND.Message,
			Description: errors2.MULTIPLE_PROFILE_FOUND.Description,
		}, http.StatusConflict)
		return nil, clientError
	}

	// Extract the single master profile ID
	var masterProfileID string
	for id := range parentProfileIDSet {
		masterProfileID = id
		break
	}

	master, err := profileStore.GetProfile(masterProfileID)
	if err != nil {
		errorMsg := fmt.Sprintf("Error fetching master profile by user_id: %s", sub)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.MULTIPLE_PROFILE_FOUND.Code,
			Message:     errors2.MULTIPLE_PROFILE_FOUND.Message,
			Description: errors2.MULTIPLE_PROFILE_FOUND.Description,
		}, err)
		return nil, serverError
	}
	if master == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}
	//  Load app context (if any)
	master.ApplicationData, _ = profileStore.FetchApplicationData(master.ProfileId)

	//  Wipe hierarchy for the /me response
	master.ProfileStatus = &profileModel.ProfileStatus{}

	return master, nil
}
