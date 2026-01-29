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
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	UnificationModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

type ProfilesServiceInterface interface {
	DeleteProfile(profileId string) error
	GetAllProfilesCursor(orgHandle string, limit int, cursor *profileModel.ProfileCursor) ([]profileModel.ProfileResponse, bool, error)
	CreateProfile(profile profileModel.ProfileRequest, orgHandle string) (*profileModel.ProfileResponse, error)
	UpdateProfile(profileId, orgHandle string, update profileModel.ProfileRequest) (*profileModel.ProfileResponse, error)
	GetProfile(profileId string) (*profileModel.ProfileResponse, error)
	FindProfileByUserId(userId string) (*profileModel.ProfileResponse, error)
	GetAllProfilesWithFilterCursor(orgHandle string, filters []string, limit int, cursor *profileModel.ProfileCursor) ([]profileModel.ProfileResponse, bool, error)
	GetProfileConsents(profileId string) ([]profileModel.ConsentRecord, error)
	UpdateProfileConsents(profileId string, consents []profileModel.ConsentRecord) error
	PatchProfile(profileId, orgHandle string, data map[string]interface{}) (*profileModel.ProfileResponse, error)
	GetProfileCookieByProfileId(profileId string) (*profileModel.ProfileCookie, error)
	GetProfileCookie(cookie string) (*profileModel.ProfileCookie, error)
	CreateProfileCookie(profileId string) (*profileModel.ProfileCookie, error)
	UpdateCookieStatus(profileId string, isActive bool) error
	DeleteCookieByProfileId(profileId string) error
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
func (ps *ProfilesService) CreateProfile(profileRequest profileModel.ProfileRequest, orgHandle string) (*profileModel.ProfileResponse, error) {

	rawSchema, err := schemaService.GetProfileSchemaService().GetProfileSchema(orgHandle)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error fetching profile schema for organization: %s", orgHandle)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}

	var schema model.ProfileSchema
	schemaBytes, _ := json.Marshal(rawSchema) // serialize
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		errMsg := fmt.Sprintf("Invalid schema format for organization: %s while validating for profile creation.", orgHandle)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}

	err = ValidateProfileAgainstSchema(profileRequest, profileModel.Profile{}, schema, false)
	if err != nil {
		return nil, err
	}

	// convert profile request to model
	createdTime := time.Now().UTC()
	profileId := uuid.New().String()
	profile := profileModel.Profile{
		ProfileId:          profileId,
		OrgHandle:          orgHandle,
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
		Location:  utils.BuildProfileLocation(orgHandle, profileId),
	}

	if err := profileStore.InsertProfile(profile); err != nil {
		logger.Debug(fmt.Sprintf("Error inserting profile: %s", profile.ProfileId), log.Error(err))
		return nil, err
	}
	profileFetched, errWait := ps.GetProfile(profileId)
	if errWait != nil || profileFetched == nil {
		logger.Warn(fmt.Sprintf("Profile: %s not available after insertion: %v", profile.ProfileId, errWait))
		return nil, errWait
	}

	queue := &workers.ProfileWorkerQueue{}

	config := UnificationModel.DefaultConfig()

	if config.ProfileUnificationTrigger.TriggerType == constants.SyncProfileOnUpdate {
		// Set organization handle for the profile before enqueuing
		profile.OrgHandle = orgHandle
		queue.Enqueue(profile)
	}

	logger.Info(fmt.Sprintf("Profile created successfully with profile id: %s", profile.ProfileId))
	return profileFetched, nil
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
			if !(attr.AttributeName == "identity_attributes.modified" || attr.AttributeName == "identity_attributes.created" || attr.AttributeName == "identity_attributes.userid") {
				if err := validateMutability(attr.Mutability, isUpdate, existingProfile.IdentityAttributes[key], val); err != nil {
					return err
				}
			}
		} else {
			if !(attr.AttributeName == "identity_attributes.modified" || attr.AttributeName == "identity_attributes.created" || attr.AttributeName == "identity_attributes.userid") {
				if err := validateMutability(attr.Mutability, isUpdate, nil, val); err != nil {
					return err
				}
			}
		}
		if !isValidType(val, attr.ValueType, attr.MultiValued, nil) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("identity attribute '%s': type mismatch", key),
			}, http.StatusBadRequest)
			return clientError
		}
		if !isValidCanonicalValue(val, attr.CanonicalValues) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("identity attribute '%s': value not in canonical values", key),
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
		if !isValidType(val, attr.ValueType, attr.MultiValued, nil) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("trait '%s': type mismatch", key),
			}, http.StatusBadRequest)
			return clientError
		}
		if !isValidCanonicalValue(val, attr.CanonicalValues) {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: fmt.Sprintf("trait '%s': value not in canonical values", key),
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

			if !isValidType(val, attr.ValueType, attr.MultiValued, nil) {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_PROFILE.Code,
					Message:     errors2.UPDATE_PROFILE.Message,
					Description: fmt.Sprintf("application_data '%s.%s': type mismatch", appID, key),
				}, http.StatusBadRequest)
				return clientError
			}
			if !isValidCanonicalValue(val, attr.CanonicalValues) {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_PROFILE.Code,
					Message:     errors2.UPDATE_PROFILE.Message,
					Description: fmt.Sprintf("application data '%s': value not in canonical values", key),
				}, http.StatusBadRequest)
				return clientError
			}
		}
	}

	return nil
}

// isValidCanonicalValue checks if the value is valid against the canonical values defined in the schema.
func isValidCanonicalValue(val interface{}, values []model.CanonicalValue) bool {
	if len(values) == 0 {
		return true // no restriction
	}

	// Build a lookup set
	allowed := make(map[string]bool)
	for _, v := range values {
		allowed[v.Value] = true
	}

	switch v := val.(type) {
	case string:
		return allowed[v]
	case []interface{}:
		for _, item := range v {
			str, ok := item.(string)
			if !ok || !allowed[str] {
				return false
			}
		}
		return true
	default:
		return false
	}
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
		log.GetLogger().Warn("Unknown mutability type: " + mutability)
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: fmt.Sprintf("Unknown mutability: %s", mutability),
		}, http.StatusBadRequest)
	}
	return nil
}

func isValidType(value interface{}, expected string, multiValued bool, subAttrs []model.ProfileSchemaAttribute) bool {
	log.GetLogger().Info("Validating value type", log.String("expected", expected), log.Any("value", value))
	switch expected {
	case constants.StringDataType:
		if multiValued {
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
		}
		_, ok := value.(string)
		return ok

	case constants.DecimalDataType:
		if multiValued {
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
		}
		_, ok := value.(float64)
		return ok

	case constants.IntegerDataType:
		if multiValued {
			arr, ok := value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range arr {
				if num, ok := v.(float64); !ok || num != float64(int(num)) {
					return false
				}
			}
			return true
		}
		switch v := value.(type) {
		case float64:
			return v == float64(int(v)) // JSON numbers are float64
		case int:
			return true
		default:
			return false
		}

	case constants.BooleanDataType:
		if multiValued {
			arr, ok := value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range arr {
				if _, ok := v.(bool); !ok {
					return false
				}
			}
			return true
		}
		_, ok := value.(bool)
		return ok

	case constants.EpochDataType:
		if multiValued {
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
		}
		_, ok := value.(string)
		return ok

	case constants.DateTimeDataType:
		if multiValued {
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
		}
		_, ok := value.(string) // optionally: validate ISO 8601
		return ok

	case constants.DateDataType:
		if multiValued {
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
		}
		_, ok := value.(string)
		return ok

	case constants.ComplexDataType:
		if multiValued {
			arr, ok := value.([]interface{})
			if !ok {
				return false
			}
			for _, item := range arr {
				_, ok := item.(map[string]interface{})
				if !ok {
					return false
				}
			}
			return true
		} else {
			_, ok := value.(map[string]interface{})
			return ok
		}
		// todo: dont we need to validate the data within complex data - as in the sub attributes
	default:
		return false
	}
}

// UpdateProfile creates or updates a profile
func (ps *ProfilesService) UpdateProfile(profileId, orgHandle string, updatedProfile profileModel.ProfileRequest) (*profileModel.ProfileResponse, error) {

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
		return nil, serverError
	}

	if profile == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}

	rawSchema, err := schemaService.GetProfileSchemaService().GetProfileSchema(profile.OrgHandle)
	if err != nil {
		return nil, err
	}

	// Convert map[string]interface{} → model.ProfileSchema
	var schema model.ProfileSchema
	schemaBytes, _ := json.Marshal(rawSchema) // serialize
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		errMsg := fmt.Sprintf("Invalid schema format for profile: %s while validating for profile update.", profile.ProfileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}

	err = ValidateProfileAgainstSchema(updatedProfile, *profile, schema, true)
	if err != nil {
		return nil, err
	}

	var profileToUpDate profileModel.Profile
	updatedTime := time.Now().UTC()
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
			return nil, serverError
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
		return nil, err
	}

	profileFetched, errWait := ps.GetProfile(profile.ProfileId)
	if errWait != nil || profileFetched == nil {
		logger.Warn(fmt.Sprintf("Profile: %s not visible after insert/updatedProfile: %v", profile.ProfileId, errWait))
		// todo: should we throw an error here?
		return nil, errWait
	}

	config := UnificationModel.DefaultConfig()
	queue := &workers.ProfileWorkerQueue{}
	if config.ProfileUnificationTrigger.TriggerType == constants.SyncProfileOnUpdate {
		// Set organization handle for the profile before enqueuing
		profileToUpDate.OrgHandle = orgHandle
		queue.Enqueue(profileToUpDate)
	}
	logger.Info("Successfully updated profile: " + profileFetched.ProfileId)
	return profileFetched, nil
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

		alias, err := profileStore.FetchReferencedProfiles(ProfileId)

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

		if err != nil {
			return nil, err
		}
		if masterProfile != nil {
			masterProfile.ApplicationData, err = profileStore.FetchApplicationData(masterProfile.ProfileId)
			if err != nil {
				return nil, err
			}

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

// GetProfileConsents retrieves a profile
func (ps *ProfilesService) GetProfileConsents(ProfileId string) ([]profileModel.ConsentRecord, error) {

	consentRecords, err := profileStore.GetProfileConsents(ProfileId)
	if err != nil {
		return nil, err
	}
	if consentRecords == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: errors2.PROFILE_NOT_FOUND.Description,
		}, http.StatusNotFound)
		return nil, clientError
	}

	return consentRecords, nil
}

// UpdateProfileConsents updates the consent records for a profile
func (ps *ProfilesService) UpdateProfileConsents(profileId string, consents []profileModel.ConsentRecord) error {
	logger := log.GetLogger()

	// Set the consent timestamp if not already set
	currentTime := time.Now().UTC()
	for i := range consents {
		if consents[i].ConsentedAt.IsZero() {
			consents[i].ConsentedAt = currentTime
		}
	}

	// Update the consents in the database
	err := profileStore.UpdateProfileConsents(profileId, consents)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to update consents for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		return err
	}

	return nil
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
		profile.ProfileStatus.References, _ = profileStore.FetchReferencedProfiles(profile.ProfileId)
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
		parentProfile.ProfileStatus.References, _ = profileStore.FetchReferencedProfiles(parentProfile.ProfileId)

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
			//todo: Ensure the need to detach the referer profile from the reference
			//err = profileStore.DetachRefererProfileFromReference(profile.ProfileStatus.ReferenceProfileId, ProfileId)
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
func (ps *ProfilesService) GetAllProfilesCursor(
	orgHandle string,
	limit int,
	cursor *profileModel.ProfileCursor,
) ([]profileModel.ProfileResponse, bool, error) {

	existingProfiles, hasMore, err := profileStore.GetAllProfiles(orgHandle, limit, cursor)
	if err != nil {
		return nil, false, err
	}
	if existingProfiles == nil {
		return []profileModel.ProfileResponse{}, false, nil
	}

	if len(existingProfiles) > limit {
		hasMore = true
		existingProfiles = existingProfiles[:limit]
	}

	result := make([]profileModel.ProfileResponse, 0, len(existingProfiles))

	for _, profile := range existingProfiles {

		//  Base row meta must be preserved for cursor correctness
		baseMeta := profileModel.Meta{
			CreatedAt: profile.CreatedAt,
			UpdatedAt: profile.UpdatedAt,
			Location:  profile.Location,
		}

		alias, err := profileStore.FetchReferencedProfiles(profile.ProfileId)
		if err != nil {
			errorMsg := fmt.Sprintf("Error fetching references for profile: %s", profile.ProfileId)
			logger := log.GetLogger()
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, false, serverError
		}
		if len(alias) == 0 {
			alias = nil
		}

		if profile.ProfileStatus.IsReferenceProfile {
			result = append(result, profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             profile.UserId,
				ApplicationData:    ConvertAppDataToMap(profile.ApplicationData),
				Traits:             profile.Traits,
				IdentityAttributes: profile.IdentityAttributes,
				Meta:               baseMeta,
				MergedFrom:         alias,
			})
			continue
		}

		// Non-master: fetch master and return master’s identity/traits/app data
		masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
		if err != nil || masterProfile == nil {
			continue
		}

		masterProfile.ApplicationData, _ = profileStore.FetchApplicationData(masterProfile.ProfileId)
		masterProfile.ProfileStatus.References, _ = profileStore.FetchReferencedProfiles(masterProfile.ProfileId)

		ref := profileModel.Reference{
			ProfileId: profile.ProfileStatus.ReferenceProfileId,
			Reason:    profile.ProfileStatus.ReferenceReason,
		}

		result = append(result, profileModel.ProfileResponse{
			ProfileId:          profile.ProfileId,
			UserId:             masterProfile.UserId,
			ApplicationData:    ConvertAppDataToMap(masterProfile.ApplicationData),
			Traits:             masterProfile.Traits,
			IdentityAttributes: masterProfile.IdentityAttributes,
			Meta:               baseMeta,
			MergedTo:           ref,
		})
	}

	return result, hasMore, nil
}

func (ps *ProfilesService) GetAllProfilesWithFilterCursor(
	orgHandle string,
	filters []string,
	limit int,
	cursor *profileModel.ProfileCursor,
) ([]profileModel.ProfileResponse, bool, error) {

	propertyTypeMap := make(map[string]string)

	// Rewrite filters (keep your logic)
	rewrittenFilters := make([]string, 0, len(filters))
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

	// Fetch matching profiles WITH cursor + limit
	filteredProfiles, hasMore, err := profileStore.GetAllProfilesWithFilter(orgHandle, rewrittenFilters, limit, cursor)
	if err != nil {
		return nil, false, err
	}
	if filteredProfiles == nil {
		return []profileModel.ProfileResponse{}, false, nil
	}

	// Optional safety if store forgot trimming
	if len(filteredProfiles) > limit {
		hasMore = true
		filteredProfiles = filteredProfiles[:limit]
	}

	result := make([]profileModel.ProfileResponse, 0, len(filteredProfiles))

	for _, profile := range filteredProfiles {

		// meta data must be preserved for cursor correctness
		baseMeta := profileModel.Meta{
			CreatedAt: profile.CreatedAt,
			UpdatedAt: profile.UpdatedAt,
			Location:  profile.Location,
		}

		if profile.ProfileStatus.IsReferenceProfile {
			result = append(result, profileModel.ProfileResponse{
				ProfileId:          profile.ProfileId,
				UserId:             profile.UserId,
				ApplicationData:    ConvertAppDataToMap(profile.ApplicationData),
				Traits:             profile.Traits,
				IdentityAttributes: profile.IdentityAttributes,
				Meta:               baseMeta,
			})
			continue
		}

		// Non-master: fetch master and return master's data
		masterProfile, err := profileStore.GetProfile(profile.ProfileStatus.ReferenceProfileId)
		if err != nil || masterProfile == nil {
			continue
		}

		masterProfile.ApplicationData, _ = profileStore.FetchApplicationData(masterProfile.ProfileId)
		masterProfile.ProfileStatus.References, _ = profileStore.FetchReferencedProfiles(masterProfile.ProfileId)

		// Override for visual reference to the child
		masterProfile.ProfileId = profile.ProfileId
		masterProfile.ProfileStatus.ReferenceProfileId = profile.ProfileId

		ref := profileModel.Reference{
			ProfileId: masterProfile.ProfileId,
			Reason:    profile.ProfileStatus.ReferenceReason,
		}

		result = append(result, profileModel.ProfileResponse{
			ProfileId:          profile.ProfileId,
			UserId:             masterProfile.UserId,
			ApplicationData:    ConvertAppDataToMap(masterProfile.ApplicationData),
			Traits:             masterProfile.Traits,
			IdentityAttributes: masterProfile.IdentityAttributes,
			Meta:               baseMeta,
			MergedTo:           ref,
		})
	}

	return result, hasMore, nil
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

// FindProfileByUserId retrieves a profile by user_id
func (ps *ProfilesService) FindProfileByUserId(userId string) (*profileModel.ProfileResponse, error) {

	profile, err := profileStore.GetProfileWithUserId(userId)
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

	alias, err := profileStore.FetchReferencedProfiles(profile.ProfileId)

	if err != nil {
		return nil, err
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
}

// PatchProfile applies a partial update to an existing profile
func (ps *ProfilesService) PatchProfile(profileId, orgHandle string, patch map[string]interface{}) (*profileModel.ProfileResponse, error) {

	existingProfile, err := profileStore.GetProfile(profileId)
	if err != nil {
		return nil, err
	}
	if existingProfile == nil {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Profile %s not found", profileId),
		}, http.StatusNotFound)
	}

	// Convert the full profile to map to allow patching
	fullData, _ := json.Marshal(existingProfile)
	var merged map[string]interface{}
	_ = json.Unmarshal(fullData, &merged)

	// Handle deep merge for nested objects first
	if traitsPatch, ok := patch["traits"].(map[string]interface{}); ok {
		if existingTraits, ok := merged["traits"].(map[string]interface{}); ok {
			merged["traits"] = DeepMerge(existingTraits, traitsPatch)
		} else {
			merged["traits"] = traitsPatch
		}
	}

	if identityPatch, ok := patch["identity_attributes"].(map[string]interface{}); ok {
		if existingIdentity, ok := merged["identity_attributes"].(map[string]interface{}); ok {
			merged["identity_attributes"] = DeepMerge(existingIdentity, identityPatch)
		} else {
			merged["identity_attributes"] = identityPatch
		}
	}

	if appDataPatch, ok := patch["application_data"].(map[string]interface{}); ok {
		if existingAppData, ok := merged["application_data"].(map[string]interface{}); ok {
			merged["application_data"] = DeepMerge(existingAppData, appDataPatch)
		} else {
			merged["application_data"] = appDataPatch
		}
	}

	// Now apply top-level scalar fields
	for k, v := range patch {
		if k == "traits" || k == "identity_attributes" || k == "application_data" {
			continue // already handled
		}
		merged[k] = v
	}
	// Convert merged data back to ProfileRequest
	mergedBytes, _ := json.Marshal(merged)
	var updatedProfileReq profileModel.ProfileRequest
	if err := json.Unmarshal(mergedBytes, &updatedProfileReq); err != nil {
		logger := log.GetLogger()
		errMsg := fmt.Sprintf("Error unmarshalling merged profile data for profile_id: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}

	// Reuse the PUT logic to update the profile
	return ps.UpdateProfile(profileId, orgHandle, updatedProfileReq)
}

func (ps *ProfilesService) GetProfileCookieByProfileId(profileId string) (*profileModel.ProfileCookie, error) {

	cookie, err := profileStore.GetProfileCookieByProfileId(profileId)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error fetching profile cookie by profile_id: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_COOKIE.Code,
			Message:     errors2.GET_PROFILE_COOKIE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}
	if cookie == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_COOKIE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_COOKIE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Profile cookie for profile_id %s not found", profileId),
		}, http.StatusNotFound)
		return nil, clientError
	}
	return cookie, nil
}

func (ps *ProfilesService) GetProfileCookie(cookie string) (*profileModel.ProfileCookie, error) {

	cookieObj, err := profileStore.GetProfileCookie(cookie)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error fetching profile cookie : %s", cookie)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_COOKIE.Code,
			Message:     errors2.GET_PROFILE_COOKIE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}
	if cookieObj == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_COOKIE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_COOKIE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Profile cookie : %s not found", cookie),
		}, http.StatusNotFound)
		return nil, clientError
	}
	return cookieObj, nil
}

// CreateProfileCookie creates a new profile cookie
func (ps *ProfilesService) CreateProfileCookie(profileId string) (*profileModel.ProfileCookie, error) {

	cookie := profileModel.ProfileCookie{
		ProfileId: profileId,
		CookieId:  uuid.New().String(),
		IsActive:  true,
	}
	err := profileStore.CreateProfileCookie(cookie)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating profile cookie by profile_id: %s", cookie.ProfileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_COOKIE.Code,
			Message:     errors2.GET_COOKIE.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
	}
	return &cookie, nil
}

// UpdateCookieStatus updates the status of a profile cookie
func (ps *ProfilesService) UpdateCookieStatus(profileId string, isActive bool) error {

	err := profileStore.UpdateProfileCookie(profileId, isActive)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error creating profile cookie by profile_id: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_COOKIE.Code,
			Message:     errors2.UPDATE_COOKIE.Message,
			Description: errMsg,
		}, err)
		return serverError
	}
	return nil
}

// DeleteCookieByProfileId deletes a profile cookie by profile_id
func (ps *ProfilesService) DeleteCookieByProfileId(profileId string) error {

	err := profileStore.DeleteProfileCookieByProfile(profileId)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("Error deleting profile cookie by profile_id: %s", profileId)
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_COOKIE.Code,
			Message:     errors2.DELETE_COOKIE.Message,
			Description: errMsg,
		}, err)
		return serverError
	}
	return nil
}

// DeepMerge merges two maps recursively, with src overwriting dst
func DeepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		if vMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				dst[k] = DeepMerge(dstMap, vMap)
			} else {
				dst[k] = vMap
			}
		} else {
			dst[k] = v
		}
	}
	return dst
}
