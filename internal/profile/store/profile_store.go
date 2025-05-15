package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"log"
	"strconv"
	"strings"
	"time"
)

// Unmarshal JSONB fields separately
func scanProfileRow(row map[string]interface{}) (model.Profile, error) {
	var (
		profile                       model.Profile
		traitsJSON, identityAttrsJSON []byte
	)

	profile.ProfileHierarchy = &model.ProfileHierarchy{}

	profile.ProfileId = row["profile_id"].(string)
	profile.OriginCountry = row["origin_country"].(string)
	profile.ProfileHierarchy.IsParent = row["is_parent"].(bool)
	profile.ProfileHierarchy.ParentProfileID = row["parent_profile_id"].(string)
	profile.ProfileHierarchy.ListProfile = row["list_profile"].(bool)
	traitsJSON = row["traits"].([]byte)
	identityAttrsJSON = row["identity_attributes"].([]byte)

	// Unmarshal JSON fields
	if err := json.Unmarshal(traitsJSON, &profile.Traits); err != nil {
		return model.Profile{}, fmt.Errorf("failed to unmarshal traits: %w", err)
	}
	if err := json.Unmarshal(identityAttrsJSON, &profile.IdentityAttributes); err != nil {
		return model.Profile{}, fmt.Errorf("failed to unmarshal identity attributes: %w", err)
	}

	return profile, nil
}

func InsertProfile(profile model.Profile) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	traitsJSON, _ := json.Marshal(profile.Traits)
	identityJSON, _ := json.Marshal(profile.IdentityAttributes)

	query := `
		INSERT INTO profiles (
		profile_id, origin_country, is_parent, parent_profile_id, list_profile, traits, identity_attributes
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (profile_id) DO NOTHING;`

	_, err = dbClient.ExecuteQuery(query,
		profile.ProfileId,
		profile.OriginCountry,
		profile.ProfileHierarchy.IsParent,
		profile.ProfileHierarchy.ParentProfileID,
		profile.ProfileHierarchy.ListProfile,
		traitsJSON,
		identityJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert profile: %w", err)
	}
	return nil
}

func InsertApplicationData(profileId string, apps []model.ApplicationData) error {

	for _, app := range apps {
		// Construct the update map
		updateMap := make(map[string]interface{})

		// Inject devices under top-level key
		if len(app.Devices) > 0 {
			updateMap["application_data.devices"] = app.Devices
		}

		// Flatten app-specific fields
		for k, v := range app.AppSpecificData {
			updateMap["application_data."+k] = v
		}

		// Use the existing upsert method
		err := UpsertAppDatum(profileId, app.AppId, updateMap)
		if err != nil {
			return fmt.Errorf("failed to upsert app_data for app %s: %w", app.AppId, err)
		}
	}
	return nil
}

func GetProfile(profileId string) (*model.Profile, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `
		SELECT profile_id, origin_country, is_parent, parent_profile_id, list_profile, traits, identity_attributes
		FROM profiles
		WHERE profile_id = $1;`

	results, err := dbClient.ExecuteQuery(query, profileId)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	profile, err := scanProfileRow(results[0])
	if err != nil {
		return nil, fmt.Errorf("error retrieving profile %s: %w", profileId, err)
	}
	profile.ApplicationData, _ = FetchApplicationData(profileId)
	return &profile, nil
}

func FetchApplicationData(profileId string) ([]model.ApplicationData, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()
	query := `SELECT app_id, application_data FROM application_data WHERE profile_id = $1;`
	results, err := dbClient.ExecuteQuery(query, profileId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app_data: %w", err)
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
			return nil, fmt.Errorf("failed to unmarshal app_data: %w", err)
		}

		apps = append(apps, model.ApplicationData{
			AppId:           appId,
			Devices:         appParsed.Devices,
			AppSpecificData: appParsed.AppSpecificData,
		})
	}
	return apps, nil
}

func FetchApplicationDataWithAppId(profileId string, appId string) (model.ApplicationData, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return model.ApplicationData{}, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()
	query := `SELECT app_id, application_data FROM application_data WHERE profile_id = $1 AND app_id = $2;`
	results, err := dbClient.ExecuteQuery(query, profileId, appId)
	var app model.ApplicationData
	if err != nil {
		return app, fmt.Errorf("failed to fetch app_data: %w", err)
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
			return app, fmt.Errorf("failed to unmarshal app_data: %w", err)
		}

		app.AppId = appId
		app.Devices = appParsed.Devices
		app.AppSpecificData = appParsed.AppSpecificData
	}
	return app, nil
}

func UpdateProfile(profile model.Profile) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	traitsJSON, _ := json.Marshal(profile.Traits)
	identityJSON, _ := json.Marshal(profile.IdentityAttributes)

	query := `
		UPDATE profiles SET
			origin_country = $1,
			is_parent = $2,
			parent_profile_id = $3,
			list_profile = $4,
			traits = $5,
			identity_attributes = $6
		WHERE profile_id = $7;`

	result, err := dbClient.ExecuteQuery(query,
		profile.OriginCountry,
		profile.ProfileHierarchy.IsParent,
		profile.ProfileHierarchy.ParentProfileID,
		profile.ProfileHierarchy.ListProfile,
		traitsJSON,
		identityJSON,
		profile.ProfileId,
	)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}
	rows := len(result)
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func GetAllProfiles() ([]model.Profile, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `
		SELECT profile_id, origin_country, is_parent, parent_profile_id, list_profile, traits, identity_attributes
		FROM profiles
		WHERE list_profile = true;`

	results, err := dbClient.ExecuteQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profiles: %w", err)
	}

	var profiles []model.Profile
	for _, row := range results {
		var profile, _ = scanProfileRow(row)

		// Fetch app data
		apps, err := FetchApplicationData(profile.ProfileId)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch app data for profile %s: %w", profile.ProfileId, err)
		}
		profile.ApplicationData = apps

		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// DeleteProfile deletes a profile and its associated data
func DeleteProfile(profileId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	// Step 1: Delete application_data explicitly (optional if ON DELETE CASCADE not enabled)
	_, err = dbClient.ExecuteQuery(`DELETE FROM application_data WHERE profile_id = $1`, profileId)
	if err != nil {
		return fmt.Errorf("failed to delete application data for profile %s: %w", profileId, err)
	}

	// Step 2: Delete child relationships where this is a parent (optional safety, ON DELETE CASCADE already exists)
	_, err = dbClient.ExecuteQuery(
		`DELETE FROM child_profiles WHERE parent_profile_id = $1 OR child_profile_id = $1`, profileId)
	if err != nil {
		return fmt.Errorf("failed to delete child profile links for profile %s: %w", profileId, err)
	}

	// Step 3: Delete the profile itself
	result, err := dbClient.ExecuteQuery(`DELETE FROM profiles WHERE profile_id = $1`, profileId)
	if err != nil {
		return fmt.Errorf("failed to delete profile %s: %w", profileId, err)
	}

	rows := len(result)
	if rows == 0 {
		return sql.ErrNoRows
	}

	logger.Info(fmt.Sprintf("INFO: Profile %s and associated data deleted successfully", profileId))
	return nil
}

// UpsertIdentityAttribute updates or inserts attributes, enriching array values.
func UpsertIdentityAttribute(profileId string, updates map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	if err != nil {
		return fmt.Errorf("failed to fetch profile %s for identity attribute upsert: %w", profileId, err)
	}
	if profile == nil {
		return fmt.Errorf("profile doesn't exist for profile id %s", profileId)
	}

	if profile.IdentityAttributes == nil {
		profile.IdentityAttributes = make(map[string]interface{})
	}

	for field, incomingVal := range updates {
		attrName := strings.TrimPrefix(field, "identity_attributes.")
		existingVal := profile.IdentityAttributes[attrName]
		profile.IdentityAttributes[attrName] = enrichFieldValues(existingVal, incomingVal)
	}

	return UpdateProfile(*profile)
}
func UpsertTrait(profileId string, updates map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	if err != nil {
		return fmt.Errorf("failed to fetch profile %s for trait upsert: %w", profileId, err)
	}
	if profile == nil {
		return fmt.Errorf("profile doesn't exist for profile id %s", profileId)
	}

	if profile.Traits == nil {
		profile.Traits = make(map[string]interface{})
	}

	for field, incomingVal := range updates {
		traitName := strings.TrimPrefix(field, "traits.")
		existingVal := profile.Traits[traitName]
		profile.Traits[traitName] = enrichFieldValues(existingVal, incomingVal)
	}

	return UpdateProfile(*profile)
}

func UpsertAppDatum(profileId string, appId string, updates map[string]interface{}) error {

	// Fetch existing application_data for the given app
	appData, err := FetchApplicationDataWithAppId(profileId, appId)
	if err != nil {
		return fmt.Errorf("failed to fetch existing application data: %w", err)
	}

	if appData.AppSpecificData == nil {
		appData.AppSpecificData = make(map[string]interface{})
	}

	// Separate handling for "devices" key (top-level)
	for key, incomingVal := range updates {
		actualKey := strings.TrimPrefix(key, "application_data.")

		if actualKey == "devices" {
			// Convert to []model.Devices
			devicesJSON, _ := json.Marshal(incomingVal)
			var newDevices []model.Devices
			if err := json.Unmarshal(devicesJSON, &newDevices); err != nil {
				return fmt.Errorf("failed to parse device list: %w", err)
			}
			appData.Devices = mergeDeviceLists(appData.Devices, newDevices)
		} else {
			// Merge into app_specific_data
			existingVal := appData.AppSpecificData[actualKey]
			appData.AppSpecificData[actualKey] = enrichFieldValues(existingVal, incomingVal)
		}
	}

	// Final wrapper for marshaling
	type ApplicationDataJSON struct {
		Devices         []model.Devices        `json:"devices,omitempty"`
		AppSpecificData map[string]interface{} `json:"app_specific_data,omitempty"`
	}

	wrapper := ApplicationDataJSON{
		Devices:         appData.Devices,
		AppSpecificData: appData.AppSpecificData,
	}

	jsonBytes, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to marshal application data: %w", err)
	}

	// Upsert into application_data table
	query := `
		INSERT INTO application_data (profile_id, app_id, application_data)
		VALUES ($1, $2, $3)
		ON CONFLICT (profile_id, app_id)
		DO UPDATE SET application_data = EXCLUDED.application_data;
	`

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	_, err = dbClient.ExecuteQuery(query, profileId, appId, jsonBytes)
	if err != nil {
		return fmt.Errorf("failed to upsert application data: %w", err)
	}

	return nil
}

// DetachChildProfileFromParent removes a child from a parent's child_profile_ids list
func DetachChildProfileFromParent(parentID, childID string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `DELETE FROM child_profiles WHERE parent_profile_id = $1 AND child_profile_id = $2;`
	result, err := dbClient.ExecuteQuery(query, parentID, childID)
	if err != nil {
		return fmt.Errorf("failed to delete child relationship: %w", err)
	}

	rowsAffected := len(result)
	if rowsAffected == 0 {
		logger.Info(fmt.Sprintf("INFO: No child profile %s found under parent %s to remove.", childID, parentID))
	}
	return nil
}

// InsertMergedMasterProfileAppData adds or updates application-specific context data.
func InsertMergedMasterProfileAppData(profileId string, newAppCtx model.ApplicationData) error {

	profile, err := GetProfile(profileId)
	if err != nil {
		return fmt.Errorf("failed to fetch profile %s for app context update: %w", profileId, err)
	}
	if profile == nil {
		// If profile doesn't exist, we might want to create it with this app data
		// For now, let's assume profile must exist. Or adjust as per requirements.
		return fmt.Errorf("profile %s not found for app context update", profileId)
	}

	updated := false
	var resultAppData []model.ApplicationData

	for _, existing := range profile.ApplicationData {
		if existing.AppId == newAppCtx.AppId {
			// Merge devices
			existing.Devices = mergeDeviceLists(existing.Devices, newAppCtx.Devices) // Uses your helper

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

// mergeDeviceLists helper (from original code, assumed correct)
func mergeDeviceLists(existing, incoming []model.Devices) []model.Devices {
	deviceMap := make(map[string]model.Devices)
	for _, d := range existing {
		if d.DeviceId == "" { // Ensure DeviceId is present for map key
			continue
		}
		deviceMap[d.DeviceId] = d
	}
	for _, d := range incoming {
		if d.DeviceId == "" {
			continue
		}
		deviceMap[d.DeviceId] = d
	}
	var merged []model.Devices
	for _, d := range deviceMap {
		merged = append(merged, d)
	}
	return merged
}

// InsertMergedMasterProfileTraitData replaces (PUT) the traits data inside Profile
func InsertMergedMasterProfileTraitData(profileId string, traitsData map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	if err != nil {
		return fmt.Errorf("failed to get profile %s for traits update: %w", profileId, err)
	}

	if profile == nil { // Profile doesn't exist, create a new one with these traits
		logger.Info(fmt.Sprintf("INFO: Profile %s not found. Creating new profile", profileId))
		return nil
	}

	profile.Traits = traitsData
	return UpdateProfile(*profile) // Update existing profile
}

// MergeIdentityDataOfProfiles replaces or adds to identity_attributes in Profile
func MergeIdentityDataOfProfiles(profileId string, identityData map[string]interface{}) error {

	profile, err := GetProfile(profileId)
	if err != nil {
		return fmt.Errorf("failed to get profile %s for identity data upsert: %w", profileId, err)
	}

	if profile == nil { // Profile doesn't exist, create new one
		newProfile := model.Profile{
			ProfileId:          profileId,
			IdentityAttributes: identityData,
		}
		logger.Info(fmt.Sprintf("INFO: Profile %s not found. Creating new profile for MergeIdentityDataOfProfiles.", profileId))
		return InsertProfile(newProfile)
	}

	if profile.IdentityAttributes == nil {
		profile.IdentityAttributes = make(map[string]interface{})
	}
	for k, v := range identityData {
		profile.IdentityAttributes[k] = v // Overwrites or adds
	}

	return UpdateProfile(*profile)
}

func GetAllProfilesWithFilter(filters []string) ([]model.Profile, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	var conditions []string
	var args []interface{}
	argID := 1
	joinedAppIDs := map[string]bool{}

	baseSQL := `
		SELECT DISTINCT p.profile_id, p.origin_country, p.is_parent, p.parent_profile_id,
						p.list_profile, p.traits, p.identity_attributes
		FROM profiles p
	`

	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, value := parts[0], parts[1], parts[2]

		scopeKey := strings.SplitN(field, ".", 2)
		if len(scopeKey) != 2 {
			continue
		}
		scope, key := scopeKey[0], scopeKey[1]

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

	finalSQL := baseSQL + "\n" + whereClause

	log.Println("FINAL SQL:", finalSQL)
	log.Println("ARGS:", args)

	results, err := dbClient.ExecuteQuery(finalSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute filtered query: %w", err)
	}

	var profiles []model.Profile
	for _, row := range results {
		profile, err := scanProfileRow(row)
		if err != nil {
			return nil, fmt.Errorf("error scanning profile row: %w", err)
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func GetAllMasterProfilesExceptForCurrent(currentProfile model.Profile) ([]model.Profile, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	query := `
		SELECT profile_id, origin_country, is_parent, parent_profile_id, list_profile, traits, identity_attributes
		FROM profiles
		WHERE is_parent = true AND profile_id != $1;
	`

	results, err := dbClient.ExecuteQuery(query, currentProfile.ProfileId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch master profiles: %w", err)
	}

	var profiles []model.Profile
	for _, row := range results {
		var (
			profile                        model.Profile
			traitsJSON, identityJSON       []byte
			isParent, listProfile          bool
			parentProfileID, originCountry string
		)

		profile.ProfileId = row["profile_id"].(string)
		originCountry = row["origin_country"].(string)
		isParent = row["is_parent"].(bool)
		parentProfileID = row["parent_profile_id"].(string)
		listProfile = row["list_profile"].(bool)
		traitsJSON = row["traits"].([]byte)
		identityJSON = row["identity_attributes"].([]byte)

		profile.OriginCountry = originCountry
		profile.ProfileHierarchy = &model.ProfileHierarchy{
			IsParent:        isParent,
			ParentProfileID: parentProfileID,
			ListProfile:     listProfile,
		}

		if err := json.Unmarshal(traitsJSON, &profile.Traits); err != nil {
			return nil, fmt.Errorf("failed to unmarshal traits: %w", err)
		}
		if err := json.Unmarshal(identityJSON, &profile.IdentityAttributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal identity attributes: %w", err)
		}

		profile.ApplicationData, _ = FetchApplicationData(profile.ProfileId)

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// UpdateParent sets parent_profile_id and is_parent=false for a profile.
func UpdateParent(master model.Profile, targetProfile model.Profile) error {
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profileToUpdate, err := GetProfile(targetProfile.ProfileId)
	if err != nil {
		return fmt.Errorf("failed to get profile %s for parent update: %w", targetProfile.ProfileId, err)
	}
	if profileToUpdate == nil {
		return fmt.Errorf("profile %s not found for parent update", targetProfile.ProfileId)
	}

	if profileToUpdate.ProfileHierarchy == nil {
		profileToUpdate.ProfileHierarchy = &model.ProfileHierarchy{}
	}
	profileToUpdate.ProfileHierarchy.ParentProfileID = master.ProfileId
	profileToUpdate.ProfileHierarchy.IsParent = false // Explicitly setting child not to be a parent

	return UpdateProfile(*profileToUpdate)
}

// FindProfileByUserName retrieves a profile by identity_attributes.user_name
func FindProfileByUserName(userName string) (*model.Profile, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	sqlStatement := `
        SELECT profile_data
        FROM profiles
        WHERE profile_data -> 'identity_attributes' ->> 'user_name' = $1;`

	_, err = dbClient.ExecuteQuery(sqlStatement, userName)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("error finding profile by username %s: %w", userName, err)
	}

	var profile model.Profile
	//if err := unmarshalProfile(profileDataBytes, &profile); err != nil {
	//	return nil, fmt.Errorf("error unmarshaling profile for username %s: %w", userName, err)
	//}
	return &profile, nil
}

func AddChildProfiles(parentProfile model.Profile, children []model.ChildProfile) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	query := `
		INSERT INTO child_profiles (parent_profile_id, child_profile_id, rule_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (parent_profile_id, child_profile_id) DO NOTHING;
	`

	for _, child := range children {
		_, err := tx.Exec(query, parentProfile.ProfileId, child.ChildProfileId, child.RuleName)
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				return err
			}
			return fmt.Errorf("failed to add child %s to parent %s: %w", child.ChildProfileId, parentProfile.ProfileId, err)
		}
	}

	return tx.Commit()
}

func FetchChildProfiles(parentProfileId string) ([]model.ChildProfile, error) {
	logger.Info(fmt.Sprintf("Fetching child profiles for parent: %s", parentProfileId))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	defer dbClient.Close()
	query := `
		SELECT child_profile_id, rule_name 
		FROM child_profiles 
		WHERE parent_profile_id = $1;
	`

	results, err := dbClient.ExecuteQuery(query, parentProfileId)
	if err != nil {
		log.Println("Database query failed", "parentProfileId", parentProfileId, "error", err)
		return nil, fmt.Errorf("failed to fetch child profiles: %w", err)
	}

	var children []model.ChildProfile
	for _, row := range results {
		var child model.ChildProfile
		child.ChildProfileId = row["child_profile_id"].(string)
		child.RuleName = row["rule_name"].(string)
		logger.Debug("Fetched child profile", "childProfileId", child.ChildProfileId, "ruleName", child.RuleName)
		children = append(children, child)
	}

	logger.Info("Successfully fetched child profiles", "parentProfileId", parentProfileId, "count", len(children))
	return children, nil
}

func enrichFieldValues(existingVal, incomingVal interface{}) interface{} {

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
		logger.Info(fmt.Sprintf("WARN: enrichFieldValues encountered unhandled type for incomingVal: %T", incomingVal))
		return incoming
	}
}

func toStringSlice(val interface{}) []string {
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
				logger.Info(fmt.Sprintf("WARN: toStringSlice: item in []interface{} is not a string: %T", item))
			}
		}
		return result
	default:
		logger.Info(fmt.Sprintf("WARN: toStringSlice: value is not []string or []interface{}: %T", val))
		return []string{}
	}
}

func toIntSlice(val interface{}) []int {
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
				logger.Info(fmt.Sprintf("WARN: toIntSlice: item in []interface{} cannot be converted to int: %T", item))
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
