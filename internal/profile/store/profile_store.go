package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// Unmarshal JSONB fields separately
func scanProfileRow(row map[string]interface{}) (model.Profile, error) {
	var (
		profile                       model.Profile
		traitsJSON, identityAttrsJSON []byte
	)

	profile.ProfileStatus = &model.ProfileStatus{}

	profile.ProfileId = row["profile_id"].(string)
	profile.UserId = row["user_id"].(string)
	profile.TenantId = row["tenant_id"].(string)
	profile.CreatedAt = row["created_at"].(time.Time)
	profile.UpdatedAt = row["updated_at"].(time.Time)
	profile.Location = row["location"].(string)
	profileStatus := row["profile_status"].(string)
	if profileStatus != "" && profileStatus != "null" {
		if profileStatus == constants.WaitOnAdmin {
			profile.ProfileStatus.IsWaitingOnAdmin = true
		}
		if profileStatus == constants.WaitOnUser {
			profile.ProfileStatus.IsWaitingOnUser = true
		}
		if profileStatus == constants.ReferenceProfile {
			profile.ProfileStatus.IsReferenceProfile = true
		}
		if profileStatus == constants.MergedTo {
			profile.ProfileStatus.ReferenceReason = row["reference_reason"].(string)
			profile.ProfileStatus.ReferenceProfileId = row["reference_profile_id"].(string)
		}
	}

	profile.ProfileStatus.ListProfile = row["list_profile"].(bool)
	traitsJSON = row["traits"].([]byte)
	identityAttrsJSON = row["identity_attributes"].([]byte)

	logger := log.GetLogger()
	// Unmarshal JSON fields
	if err := json.Unmarshal(traitsJSON, &profile.Traits); err != nil {
		errorMsg := "Failed to unmarshal traits"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return model.Profile{}, serverError
	}
	if err := json.Unmarshal(identityAttrsJSON, &profile.IdentityAttributes); err != nil {
		errorMsg := "Failed to unmarshal identity attributes."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return model.Profile{}, serverError
	}
	return profile, nil
}

func scanProfileConsentRow(row map[string]interface{}) (model.ConsentRecord, error) {
	var profileConsent model.ConsentRecord

	profileConsent.CategoryIdentifier = row["category_id"].(string)
	profileConsent.IsConsented = row["consent_status"].(bool)
	profileConsent.ConsentedAt = row["consented_at"].(time.Time)
	return profileConsent, nil
}

// InsertProfile inserts a new profile into the database
func InsertProfile(profile model.Profile) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client for adding a profile"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	traitsJSON, _ := json.Marshal(profile.Traits)
	identityJSON, _ := json.Marshal(profile.IdentityAttributes)
	var profileStatus string
	if profile.ProfileStatus.IsReferenceProfile {
		profileStatus = constants.ReferenceProfile
	} else if profile.ProfileStatus.IsWaitingOnUser {
		profileStatus = constants.WaitOnUser
	} else if profile.ProfileStatus.IsWaitingOnAdmin {
		profileStatus = constants.WaitOnAdmin
	} else {
		profileStatus = constants.MergedTo
	}

	query := scripts.InsertProfile[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query,
		profile.ProfileId,
		profile.UserId,
		profile.TenantId,
		profile.CreatedAt,
		profile.UpdatedAt,
		profile.Location,
		profile.ProfileStatus.ListProfile,
		false, // delete_profile is not used in this context, set to false
		traitsJSON,
		identityJSON,
	)

	if err != nil {
		errorMsg := fmt.Sprintf("Failed to insert profile with Id: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	referenceQuery := scripts.InsertProfileReference[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(referenceQuery,
		profile.ProfileId,
		profileStatus,
		profile.ProfileStatus.ReferenceProfileId,
		profile.ProfileStatus.ReferenceReason,
		profile.TenantId,
		profile.TenantId,
	)

	if err != nil {
		errorMsg := fmt.Sprintf("Failed to insert profile referencea with Id: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	err = InsertApplicationData(profile.ProfileId, profile.ApplicationData)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to insert profile with Id: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info("Profile added successfully: " + profile.ProfileId)
	return nil
}

func InsertApplicationData(profileId string, apps []model.ApplicationData) error {

	for _, app := range apps {
		// Construct the update map
		updateMap := make(map[string]interface{})

		// Flatten app-specific fields
		for k, v := range app.AppSpecificData {
			updateMap["application_data."+k] = v
		}

		// Use the existing upsert method
		err := UpsertAppDatum(profileId, app.AppId, updateMap)
		logger := log.GetLogger()
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to insert application data for profile with Id: %s and appId: %s",
				profileId, app.AppId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_APP_DATA.Code,
				Message:     errors2.ADD_APP_DATA.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}
	return nil
}

// GetProfile retrieves a profile by its Id
func GetProfile(profileId string) (*model.Profile, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while fetching profile with Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileById[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, profileId)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug(fmt.Sprintf("No profile found with the given Id: %s", profileId))
		// todo: should we return a client error with 404 here?
		return nil, nil
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile found with the given Id: %s", profileId))
		// todo: should we return a client error with 404 here?
		return nil, nil
	}
	profile, err := scanProfileRow(results[0])
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching profile with Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	profile.ApplicationData, _ = FetchApplicationData(profileId)
	return &profile, nil
}

// GetProfileConsents retrieves the consents of a profile by its profileId
func GetProfileConsents(profileId string) ([]model.ConsentRecord, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while fetching profile consents with Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileConsentsByProfileId[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, profileId)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug(fmt.Sprintf("No profile found with the given Id: %s", profileId))
		// todo: should we return a client error with 404 here?
		return nil, nil
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile found with the given Id: %s", profileId))
		var profile, _ = GetProfile(profileId)
		if profile != nil {
			// If no consents found and the user exists, return an empty slice instead of nil
			return []model.ConsentRecord{}, nil
		} else {
			return nil, nil
		}
	}
	var profileConsents []model.ConsentRecord
	for _, row := range results {

		profileConsent, err := scanProfileConsentRow(row)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed fetching profile consents with Id: %s", profileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		profileConsents = append(profileConsents, profileConsent)
	}
	return profileConsents, nil
}

func FetchApplicationData(profileId string) ([]model.ApplicationData, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed getting db client for fetching application data for profile with"+
			" Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_APP_DATA.Code,
			Message:     errors2.GET_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()
	query := scripts.GetAppDataByProfileId[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, profileId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching application data for profile with Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_APP_DATA.Code,
			Message:     errors2.GET_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var apps []model.ApplicationData
	for _, row := range results {
		var (
			appId     string
			appBlob   []byte
			appParsed model.ApplicationData
		)

		appId = row["app_id"].(string)
		appBlob = row["application_data"].([]byte)

		if err := json.Unmarshal(appBlob, &appParsed); err != nil {
			errorMsg := fmt.Sprintf("Failed to un marshal application data for profile with Id: %s", profileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_APP_DATA.Code,
				Message:     errors2.GET_APP_DATA.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}

		apps = append(apps, model.ApplicationData{
			AppId:           appId,
			AppSpecificData: appParsed.AppSpecificData,
		})
	}
	return apps, nil
}

func FetchApplicationDataWithAppId(profileId string, appId string) (model.ApplicationData, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed getting db client for fetching application data of app:%s for profile: %s", appId, profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_APP_DATA.Code,
			Message:     errors2.GET_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return model.ApplicationData{}, serverError
	}
	defer dbClient.Close()
	query := scripts.GetAppDataByAppId[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, profileId, appId)
	var app model.ApplicationData
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching application data of app:%s for profile: %s", appId, profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_APP_DATA.Code,
			Message:     errors2.GET_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return app, serverError
	}

	for _, row := range results {
		var (
			appId     string
			appBlob   []byte
			appParsed model.ApplicationData
		)

		appId = row["app_id"].(string)
		appBlob = row["application_data"].([]byte)

		if err := json.Unmarshal(appBlob, &appParsed); err != nil {
			errorMsg := fmt.Sprintf("Failed unmarshalling application data of app:%s for profile: %s", appId, profileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_APP_DATA.Code,
				Message:     errors2.GET_APP_DATA.Message,
				Description: errorMsg,
			}, err)
			return app, serverError
		}

		app.AppId = appId
		app.AppSpecificData = appParsed.AppSpecificData
	}
	return app, nil
}

// UpdateProfile updates the profile
func UpdateProfile(profile model.Profile) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating profile: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	traitsJSON, _ := json.Marshal(profile.Traits)
	identityJSON, _ := json.Marshal(profile.IdentityAttributes)

	var profileStatus string
	if profile.ProfileStatus.IsReferenceProfile {
		profileStatus = constants.ReferenceProfile
	} else if profile.ProfileStatus.IsWaitingOnUser {
		profileStatus = constants.WaitOnUser
	} else if profile.ProfileStatus.IsWaitingOnAdmin {
		profileStatus = constants.WaitOnAdmin
	} else {
		profileStatus = constants.MergedTo
	}

	query := scripts.UpdateProfile[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query,
		profile.UserId,
		profile.ProfileStatus.ListProfile,
		profile.ProfileStatus.DeleteProfile,
		traitsJSON,
		identityJSON,
		profile.UpdatedAt,
		profile.ProfileId,
	)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed updating the profile: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query = scripts.UpsertProfileReference[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query,
		profile.ProfileId,
		profileStatus,
		profile.ProfileStatus.ReferenceProfileId,
		profile.ProfileStatus.ReferenceReason,
		profile.ProfileId,
	)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed updating the profile: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	// Update application data
	err = InsertApplicationData(profile.ProfileId, profile.ApplicationData)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to insert profile with Id: %s", profile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_PROFILE.Code,
			Message:     errors2.ADD_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// GetAllProfiles retrieves all profiles
func GetAllProfiles(tenantId string) ([]model.Profile, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client for fetching all profiles"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfilesByOrgId[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, tenantId)
	if err != nil {
		errorMsg := "Failed fetching all profiles"
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var profiles []model.Profile
	for _, row := range results {
		var profile, _ = scanProfileRow(row)

		// Fetch app data
		apps, err := FetchApplicationData(profile.ProfileId)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed fetching application data for the profile: %s", profile.ProfileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		profile.ApplicationData = apps

		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// DeleteProfile deletes a profile and its associated data
func DeleteProfile(profileId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed getting db client for deleting the profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	// Step 1: Delete application_data explicitly (optional if ON DELETE CASCADE not enabled)
	_, err = dbClient.ExecuteQuery(scripts.DeleteProfileByProfileId[provider.NewDBProvider().GetDBType()], profileId)
	if err != nil {
		errorMsg := fmt.Sprintf("failed to delete application data for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	// Step 2: Delete child relationships where this is a parent (optional safety, ON DELETE CASCADE already exists)
	//:todo: Need to decide if its needed
	//_, err = dbClient.ExecuteQuery(
	//	`DELETE FROM profiles WHERE reference_profile_id = $1`, profileId)
	//if err != nil {
	//	errorMsg := fmt.Sprintf("failed to delete child profile links for profile: %s", profileId)
	//	logger.Debug(errorMsg, log.Error(err))
	//	serverError := errors2.NewServerError(errors2.ErrorMessage{
	//		Code:        errors2.DELETE_PROFILE.Code,
	//		Message:     errors2.DELETE_PROFILE.Message,
	//		Description: errorMsg,
	//	}, err)
	//	return serverError
	//}

	// Step 3: Delete the profile itself
	result, err := dbClient.ExecuteQuery(`DELETE FROM profiles WHERE profile_id = $1`, profileId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Debug(fmt.Sprintf("No profile found with the given Id: %s", profileId))
			return nil
		}
		errorMsg := fmt.Sprintf("failed to delete  profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	rows := len(result)
	if rows == 0 {
		logger.Debug(fmt.Sprintf("No profile found with the given Id: %s. Probably would have been deleted already", profileId))
	}

	logger.Info(fmt.Sprintf("Profile: %s and associated data deleted successfully", profileId))
	return nil
}

func UpsertAppDatum(profileId string, appId string, updates map[string]interface{}) error {

	// Fetch existing application_data for the given app
	appData, err := FetchApplicationDataWithAppId(profileId, appId)
	if err != nil {
		return err
	}

	if appData.AppSpecificData == nil {
		appData.AppSpecificData = make(map[string]interface{})
	}

	logger := log.GetLogger()
	// Separate handling for "devices" key (top-level)
	for key, incomingVal := range updates {
		actualKey := strings.TrimPrefix(key, "application_data.")
		// Merge into app_specific_data
		existingVal := appData.AppSpecificData[actualKey]
		log.GetLogger().Info("Merging key: " + actualKey)
		appData.AppSpecificData[actualKey] = enrichFieldValues(existingVal, incomingVal)

	}

	// Final wrapper for marshaling
	type ApplicationDataJSON struct {
		AppSpecificData map[string]interface{} `json:"app_specific_data,omitempty"`
	}

	wrapper := ApplicationDataJSON{
		AppSpecificData: appData.AppSpecificData,
	}

	jsonBytes, err := json.Marshal(wrapper)
	errorMsg := fmt.Sprintf("Failed to marshal devices for application data for profile: %s", profileId)
	if err != nil {
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_APP_DATA.Code,
			Message:     errors2.UPDATE_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	// Upsert into application_data table
	query := scripts.InsertApplicationData[provider.NewDBProvider().GetDBType()]

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for upserting application data for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_APP_DATA.Code,
			Message:     errors2.UPDATE_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	_, err = dbClient.ExecuteQuery(query, profileId, appId, jsonBytes)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to upsert application data for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_APP_DATA.Code,
			Message:     errors2.UPDATE_APP_DATA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// DetachRefererProfileFromReference removes a child from a parent's child_profile_ids list
func DetachRefererProfileFromReference(referenceProfileId, profileId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for deleting child relationship of child: %s of parent: %s", referenceProfileId, profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	// todo: decide if we need to delete the references as well.
	query := scripts.DeleteProfileReference[provider.NewDBProvider().GetDBType()]
	result, err := dbClient.ExecuteQuery(query, referenceProfileId, profileId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to delete child relationship of child: %s of parent: %s",
			referenceProfileId, profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE.Code,
			Message:     errors2.DELETE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	rowsAffected := len(result)
	if rowsAffected == 0 {
		logger.Info(fmt.Sprintf("No child profile %s found under parent %s to remove.", profileId, referenceProfileId))
	}
	return nil
}

// InsertMergedMasterProfileAppData adds or updates application-specific context data.
func InsertMergedMasterProfileAppData(profileId string, newAppCtx model.ApplicationData) error {

	profile, err := GetProfile(profileId)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch profile %s for app data update.", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	if profile == nil {
		errorMsg := fmt.Sprintf("Profile: %s not found for app data update", profileId)
		logger.Debug(errorMsg, log.Error(err))
		clientError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return clientError
	}

	updated := false
	var resultAppData []model.ApplicationData

	for _, existing := range profile.ApplicationData {
		if existing.AppId == newAppCtx.AppId {
			// Merge app-specific fields
			if existing.AppSpecificData == nil {
				existing.AppSpecificData = make(map[string]interface{})
			}
			for k, v := range newAppCtx.AppSpecificData {
				existing.AppSpecificData[k] = v
			}
			resultAppData = append(resultAppData, existing)
			updated = true
		} else {
			resultAppData = append(resultAppData, existing)
		}
	}

	if !updated {
		resultAppData = append(resultAppData, newAppCtx)
	}

	profile.ApplicationData = resultAppData
	// this inserts the entire application_data blob with the update.
	return InsertApplicationData(profile.ProfileId, profile.ApplicationData)
}

// InsertMergedMasterProfileTraitData replaces (PUT) the traits data inside Profile
func InsertMergedMasterProfileTraitData(profileId string, traitsData map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch profile %s for trait data update.", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	if profile == nil {
		errorMsg := fmt.Sprintf("Profile: %s not found for trait data update", profileId)
		logger.Debug(errorMsg, log.Error(err))
		clientError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return clientError
	}

	profile.Traits = traitsData
	return UpdateProfile(*profile) // Update existing profile
}

// MergeIdentityDataOfProfiles replaces or adds to identity_attributes in Profile
func MergeIdentityDataOfProfiles(profileId string, identityData map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch profile %s for identity data update.", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	if profile == nil {
		errorMsg := fmt.Sprintf("Profile: %s not found for identity data update", profileId)
		logger.Debug(errorMsg, log.Error(err))
		clientError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return clientError
	}

	if profile.IdentityAttributes == nil {
		profile.IdentityAttributes = make(map[string]interface{})
	}
	for k, v := range identityData {
		profile.IdentityAttributes[k] = v // Overwrites or adds
	}

	return UpdateProfile(*profile)
}

func GetAllProfilesWithFilter(tenantId string, filters []string) ([]model.Profile, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get database client filtering profiles."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FILTER_PROFILE.Code,
			Message:     errors2.FILTER_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	var conditions []string
	var args []interface{}
	argID := 1
	joinedAppIDs := map[string]bool{}

	baseSQL := scripts.GetAllProfilesWithFilter[provider.NewDBProvider().GetDBType()]

	// Always ensure tenant_id condition first
	conditions = append(conditions, fmt.Sprintf("p.tenant_id = $%d", argID))
	args = append(args, tenantId)
	argID++

	// Always ensure list_profile = true
	conditions = append(conditions, "p.list_profile = true")

	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, value := parts[0], parts[1], parts[2]

		var scope, key string
		if field == "user_id" {
			scope = "user_id"
			key = ""
		} else {
			scopeKey := strings.SplitN(field, ".", 2)
			if len(scopeKey) != 2 {
				continue
			}
			scope, key = scopeKey[0], scopeKey[1]
		}

		var clause string

		switch scope {
		case "identity_attributes", "traits":
			jsonCol := "p." + scope
			switch operator {
			case "eq":
				clause = fmt.Sprintf("%s ->> '%s' = $%d", jsonCol, key, argID)
				args = append(args, value)
			case "co":
				clause = fmt.Sprintf("%s ->> '%s' ILIKE $%d", jsonCol, key, argID)
				args = append(args, "%"+value+"%")
			case "sw":
				clause = fmt.Sprintf("%s ->> '%s' ILIKE $%d", jsonCol, key, argID)
				args = append(args, value+"%")
			default:
				continue
			}
			conditions = append(conditions, clause)
			argID++

		case "user_id":
			switch operator {
			case "eq":
				clause = fmt.Sprintf("p.user_id = $%d", argID)
				args = append(args, value)
			case "co":
				clause = fmt.Sprintf("p.user_id ILIKE $%d", argID)
				args = append(args, "%"+value+"%")
			case "sw":
				clause = fmt.Sprintf("p.user_id ILIKE $%d", argID)
				args = append(args, value+"%")
			default:
				continue
			}
			conditions = append(conditions, clause)
			argID++

		case "application_data":
			var appAlias, appKey string

			if strings.Contains(key, ".") {
				appScope := strings.SplitN(key, ".", 2)
				appID := appScope[0]
				appKey = appScope[1]
				appAlias = "a_" + appID

				if !joinedAppIDs[appID] {
					baseSQL += fmt.Sprintf(`
						INNER JOIN application_data %s
						ON %s.profile_id = p.profile_id AND %s.app_id = '%s'
					`, appAlias, appAlias, appAlias, appID)
					joinedAppIDs[appID] = true
				}
			} else {
				appKey = key
				appAlias = "a"
				if !joinedAppIDs["__generic"] {
					baseSQL += `
						INNER JOIN application_data a
						ON a.profile_id = p.profile_id
					`
					joinedAppIDs["__generic"] = true
				}
			}

			switch operator {
			case "eq":
				clause = fmt.Sprintf("%s.application_data -> 'app_specific_data' ->> '%s' = $%d", appAlias, appKey, argID)
				args = append(args, value)
			case "co":
				clause = fmt.Sprintf("%s.application_data -> 'app_specific_data' ->> '%s' ILIKE $%d", appAlias, appKey, argID)
				args = append(args, "%"+value+"%")
			case "sw":
				clause = fmt.Sprintf("%s.application_data -> 'app_specific_data' ->> '%s' ILIKE $%d", appAlias, appKey, argID)
				args = append(args, value+"%")
			default:
				continue
			}
			conditions = append(conditions, clause)
			argID++
		}
	}

	// Always ensure list_profile = true
	conditions = append(conditions, "p.list_profile = true")

	// Final query
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	// todo: need to add tenant id

	finalSQL := baseSQL + "\n" + whereClause

	results, err := dbClient.ExecuteQuery(finalSQL, args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to execute filtered query: %s", err)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FILTER_PROFILE.Code,
			Message:     errors2.FILTER_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var profiles []model.Profile
	for _, row := range results {
		profile, err := scanProfileRow(row)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to scan profile row: %s", err)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.FILTER_PROFILE.Code,
				Message:     errors2.FILTER_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		profile.ApplicationData, err = FetchApplicationData(profile.ProfileId)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to fetch application data: %s", err)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.FILTER_PROFILE.Code,
				Message:     errors2.FILTER_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func GetAllReferenceProfilesExceptForCurrent(currentProfile model.Profile) ([]model.Profile, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching master profiles for profile: %s",
			currentProfile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetAllReferenceProfileExceptCurrent[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, currentProfile.ProfileId, currentProfile.TenantId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching all master profiles except for current profile: %s", currentProfile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var profiles []model.Profile
	for _, row := range results {
		var (
			profile                                                      model.Profile
			traitsJSON, identityJSON                                     []byte
			isReferenceProfile, isWaitOnUser, isWaitOnAdmin, listProfile bool
			referenceProfileId, profileStatus                            string
		)

		profile.UserId = row["user_id"].(string)
		profile.ProfileId = row["profile_id"].(string)
		referenceProfileId = row["reference_profile_id"].(string)
		listProfile = row["list_profile"].(bool)
		deleteProfile := row["delete_profile"].(bool)
		traitsJSON = row["traits"].([]byte)
		identityJSON = row["identity_attributes"].([]byte)
		profileStatus = row["profile_status"].(string) // Assuming profile_status is a boolean field
		if profileStatus == constants.ReferenceProfile {
			isReferenceProfile = true
		}
		if profileStatus == constants.WaitOnUser {
			isWaitOnUser = true
		}
		if profileStatus == constants.WaitOnAdmin {
			isWaitOnAdmin = true
		}

		profile.ProfileStatus = &model.ProfileStatus{
			IsReferenceProfile: isReferenceProfile,
			IsWaitingOnAdmin:   isWaitOnAdmin,
			IsWaitingOnUser:    isWaitOnUser,
			ReferenceProfileId: referenceProfileId,
			ListProfile:        listProfile,
			DeleteProfile:      deleteProfile,
		}

		if err := json.Unmarshal(traitsJSON, &profile.Traits); err != nil {
			errMsg := fmt.Sprintf("Failed to unmarshal traits for profile: %s", profile.ProfileId)
			logger.Debug(errMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errMsg,
			}, err)
			return nil, serverError
		}
		if err := json.Unmarshal(identityJSON, &profile.IdentityAttributes); err != nil {
			errMsg := fmt.Sprintf("Failed to unmarshal identity attributes for profile: %s", profile.ProfileId)
			logger.Debug(errMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.GET_PROFILE.Code,
				Message:     errors2.GET_PROFILE.Message,
				Description: errMsg,
			}, err)
			return nil, serverError
		}

		profile.ApplicationData, _ = FetchApplicationData(profile.ProfileId)

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// UpdateProfileReferences updates the references of a parent profile with the provided child profiles.
func UpdateProfileReferences(parentProfile model.Profile, children []model.Reference) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for adding child profiles for parent: %s",
			parentProfile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for adding child profiles for parent: %s",
			parentProfile.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	query := scripts.UpdateProfileReference[provider.NewDBProvider().GetDBType()]

	for _, child := range children {
		_, err := tx.Exec(query, parentProfile.ProfileId, child.Reason, constants.MergedTo, child.ProfileId)
		if err != nil {
			errRoll := tx.Rollback()
			if errRoll != nil {
				errorMsg := fmt.Sprintf("Failed to rollback transaction after error: %s", err)
				logger.Debug(errorMsg, log.Error(err))
				serverError := errors2.NewServerError(errors2.ErrorMessage{
					Code:        errors2.UPDATE_PROFILE.Code,
					Message:     errors2.UPDATE_PROFILE.Message,
					Description: errorMsg,
				}, errRoll)
				return serverError
			}
			errorMsg := fmt.Sprintf("Failed to insert referenced profile: %s for parent profile: %s", child.ProfileId, parentProfile.ProfileId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}

	return tx.Commit()
}

func FetchReferencedProfiles(referenceProfileId string) ([]model.Reference, error) {

	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Fetching referenced profiles for profile: %s", referenceProfileId))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching child profiles for parent: %s",
			referenceProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()
	query := scripts.FetchReferencedProfiles[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, referenceProfileId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching referenced profiles for profile: %s", referenceProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var children []model.Reference
	for _, row := range results {
		var reference model.Reference
		reference.ProfileId = row["profile_id"].(string)
		reference.Reason = row["reference_reason"].(string)
		children = append(children, reference)
	}

	if len(children) == 0 {
		logger.Info(fmt.Sprintf("No referenced profiles found for parent profile: %s", referenceProfileId))
	} else {
		logger.Info(fmt.Sprintf("Successfully fetched child profiles for parent profile: %s", referenceProfileId))
	}
	return children, nil
}

func enrichFieldValues(existingVal, incomingVal interface{}) interface{} {
	logger := log.GetLogger()
	switch incoming := incomingVal.(type) {
	case []string:
		existing := toStringSlice(existingVal)
		for _, item := range incoming {
			if !containsString(existing, item) {
				existing = append(existing, item)
			}
		}
		return existing

	case []int:
		existing := toIntSlice(existingVal)
		for _, item := range incoming {
			if !containsInt(existing, item) {
				existing = append(existing, item)
			}
		}
		return existing

	case string, int, bool:
		return incoming // overwrite simple types

	default:
		logger.Warn(fmt.Sprintf("EnrichFieldValues encountered unhandled type for incomingVal: %T", incomingVal))
		logger.Warn(fmt.Sprintf("EnrichFieldValues encountered unhandled type for existing: %T", existingVal))
		return incoming
	}
}

func toStringSlice(val interface{}) []string {

	logger := log.GetLogger()
	if val == nil {
		return []string{}
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				logger.Warn(fmt.Sprintf("WARN: toStringSlice: item in []interface{} is not a string: %T", item))
			}
		}
		return result
	default:
		logger.Warn(fmt.Sprintf("WARN: toStringSlice: value is not []string or []interface{}: %T", val))
		return []string{}
	}
}

func toIntSlice(val interface{}) []int {

	logger := log.GetLogger()
	if val == nil {
		return []int{}
	}
	switch v := val.(type) {
	case []int:
		return v
	case []interface{}:
		var result []int
		for _, item := range v {
			if i, ok := toInt(item); ok {
				result = append(result, i)
			} else {
				logger.Warn(fmt.Sprintf("toIntSlice: item in []interface{} cannot be converted to int: %T", item))
			}
		}
		return result
	default:
		logger.Info(fmt.Sprintf("WARN: toIntSlice: value is not []int or []interface{}: %T", val))
		return []int{}
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func containsInt(slice []int, i int) bool {
	for _, item := range slice {
		if item == i {
			return true
		}
	}
	return false
}

func toInt(val interface{}) (int, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		// Be cautious about potential overflow if int64 > max int
		return int(v), true
	case float32:
		return int(v), true // Potential loss of precision
	case float64:
		return int(v), true // Potential loss of precision
	case json.Number:
		if i64, err := v.Int64(); err == nil {
			return int(i64), true // Potential overflow
		}
		if f64, err := v.Float64(); err == nil {
			return int(f64), true // Potential loss of precision
		}
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

func GetProfileWithUserId(userId string) (*model.Profile, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while fetching profile with userId: %s", userId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileByUserId[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, userId)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug(fmt.Sprintf("No profile found with the given userId: %s", userId))
		// todo: should we return a client error with 404 here?
		return nil, nil
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile found with the given userId: %s", userId))
		// todo: should we return a client error with 404 here?
		return nil, nil
	}
	profile, err := scanProfileRow(results[0])
	if err != nil {
		errorMsg := fmt.Sprintf("Failed fetching profile with Id: %s", userId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE.Code,
			Message:     errors2.GET_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	profile.ApplicationData, _ = FetchApplicationData(profile.ProfileId)
	return &profile, nil
}

// CreateProfileCookie creates a new profile cookie
func CreateProfileCookie(profileCookie model.ProfileCookie) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while creating profile cookie with profileId: %s", profileCookie.ProfileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.CREATE_PROFILE_COOKIE.Code,
			Message:     errors2.CREATE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.InsertCookie[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query, profileCookie.CookieId, profileCookie.ProfileId, profileCookie.IsActive)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed creating the profile cookie with Id: %s", profileCookie.CookieId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.CREATE_PROFILE_COOKIE.Code,
			Message:     errors2.CREATE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// GetProfileCookieByProfileId retrieves a profile cookie by profileId
func GetProfileCookieByProfileId(profileId string) (*model.ProfileCookie, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while fetching profile cookie with profileId: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_COOKIE.Code,
			Message:     errors2.GET_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetCookieByProfileId[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, profileId)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug(fmt.Sprintf("No profile cookie found with the given profileId: %s", profileId))
		return nil, nil
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile cookie found with the given profileId: %s", profileId))
		return nil, nil
	}

	profileCookie := &model.ProfileCookie{
		CookieId:  results[0]["cookie_id"].(string),
		ProfileId: results[0]["profile_id"].(string),
		IsActive:  results[0]["is_active"].(bool),
	}
	return profileCookie, nil
}

// GetProfileCookie retrieves a profile cookie by profileId
func GetProfileCookie(cookie string) (*model.ProfileCookie, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while fetching profile cookie: %s", cookie)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_COOKIE.Code,
			Message:     errors2.GET_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetCookieByCookieId[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, cookie)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug(fmt.Sprintf("No profile cookie found with the given cookie: %s", cookie))
		return nil, nil
	}
	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile cookie found with the given cookie: %s", cookie))
		return nil, nil
	}

	profileCookie := &model.ProfileCookie{
		CookieId:  results[0]["cookie_id"].(string),
		ProfileId: results[0]["profile_id"].(string),
		IsActive:  results[0]["is_active"].(bool),
	}
	return profileCookie, nil
}

// UpdateProfileCookie updates the status of a profile cookie
func UpdateProfileCookie(profileId string, isActive bool) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while updating profile cookie with profileId: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE_COOKIE.Code,
			Message:     errors2.UPDATE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.UpdateCookieStatusByProfileId[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query, isActive, profileId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed updating the profile cookie with profile Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE_COOKIE.Code,
			Message:     errors2.UPDATE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// DeleteProfileCookieByProfile deletes a profile cookie by profileId
func DeleteProfileCookieByProfile(profileId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client while deleting profile cookie with profileId: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE_COOKIE.Code,
			Message:     errors2.DELETE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.DeleteCookieByProfileId[provider.NewDBProvider().GetDBType()]

	_, err = dbClient.ExecuteQuery(query, profileId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed deleting the profile cookie with Id: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.DELETE_PROFILE_COOKIE.Code,
			Message:     errors2.DELETE_PROFILE_COOKIE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	return nil
}

// UpdateProfileConsents updates or creates consent records for a profile
func UpdateProfileConsents(profileId string, consents []model.ConsentRecord) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for updating profile consents for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	// Start a transaction to ensure atomicity of consent updates
	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for updating consents for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	// First, delete existing consents for this profile to ensure a clean slate

	deleteQuery := scripts.DeleteProfileConsentsByProfileId[provider.NewDBProvider().GetDBType()]
	_, err = tx.Exec(deleteQuery, profileId)
	if err != nil {
		_ = tx.Rollback()
		errorMsg := fmt.Sprintf("Failed to delete existing consents for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	// Insert new consent records
	insertQuery := scripts.InsertProfileConsentsByProfileId[provider.NewDBProvider().GetDBType()]
	for _, consent := range consents {

		_, err = tx.Exec(insertQuery,
			profileId,
			consent.CategoryIdentifier,
			consent.IsConsented,
			consent.ConsentedAt)

		if err != nil {
			_ = tx.Rollback()
			errorMsg := fmt.Sprintf("Failed to insert consent for profile: %s, category: %s",
				profileId, consent.CategoryIdentifier)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.UPDATE_PROFILE.Code,
				Message:     errors2.UPDATE_PROFILE.Message,
				Description: errorMsg,
			}, err)
			return serverError
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		errorMsg := fmt.Sprintf("Failed to commit transaction for updating consents for profile: %s", profileId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info(fmt.Sprintf("Successfully updated consents for profile: %s", profileId))
	return nil
}
