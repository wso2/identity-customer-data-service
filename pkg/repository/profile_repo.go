package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/logger"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// ProfileRepository handles MongoDB operations for profiles
type ProfileRepository struct {
	Collection *mongo.Collection
}

// NewProfileRepository creates a new repository instance
func NewProfileRepository(db *mongo.Database, collectionName string) *ProfileRepository {
	return &ProfileRepository{
		Collection: db.Collection(collectionName),
	}
}

// InsertProfile saves a profile in MongoDB
func (repo *ProfileRepository) InsertProfile(profile models.Profile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"profile_id": profile.ProfileId}
	update := bson.M{"$setOnInsert": profile}

	opts := options.Update().SetUpsert(true)

	_, err := repo.Collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// UpdateProfile saves a profile in MongoDB
func (repo *ProfileRepository) UpdateProfile(profile models.Profile) (*mongo.InsertOneResult, error) {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := repo.Collection.InsertOne(ctx, profile)

	if err != nil {
		//logger.LogMessage("ERROR", "Failed to insert profile: "+err.Error())
		return nil, err
	}
	//logger.LogMessage("INFO", "Profile inserted with TraitId: "+result.InsertedID.(string))
	return result, nil
}

// GetProfile retrieves a profile by `profile_id`
func (repo *ProfileRepository) GetProfile(profileId string) (*models.Profile, error) {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			//logger.LogMessage("INFO", "Profile not found for profileId: "+profileId)
			return nil, nil // Return `nil` instead of error
		}
		//logger.LogMessage("ERROR", "Error finding profile: "+err.Error())
		return nil, err
	}

	//logger.LogMessage("INFO", "Profile retrieved for profileId: "+profileId)
	return &profile, nil
}

// FindProfileByID retrieves a profile by `profile_id`
func (repo *ProfileRepository) FindProfileByID(profileId string) (*models.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Profile not found is not an error
		}
		return nil, err
	}
	return &profile, nil
}

// DeleteProfile removes a profile from MongoDB using `profile_id`
func (repo *ProfileRepository) DeleteProfile(profileId string) error {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"profile_id": profileId}

	result, err := repo.Collection.DeleteOne(ctx, filter)
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to delete profile: "+err.Error())
		return err
	}

	if result.DeletedCount == 0 {
		//logger.LogMessage("INFO", "No profile found to delete")
		return mongo.ErrNoDocuments
	}

	//logger.LogMessage("INFO", "Profile deleted successfully")
	return nil
}

func (repo *ProfileRepository) DetachChildFromParent(parentID, childID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$pull": bson.M{
			"profile_hierarchy.child_profile_ids": bson.M{
				"child_profile_id": childID,
			},
		},
	}
	_, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": parentID}, update)
	return err
}

func (repo *ProfileRepository) DetachPeer(profileID, peerToRemove string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$pull": bson.M{
			"profile_hierarchy.peer_profile_ids": peerToRemove,
		},
	}
	_, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileID}, update)
	return err
}

func (repo *ProfileRepository) AddOrUpdateAppContext(profileId string, newAppCtx models.ApplicationData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Fetch full profile
	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Step 2: Prepare or update application data
	updated := false
	var updatedAppData []models.ApplicationData

	for _, existing := range profile.ApplicationData {
		if existing.AppId == newAppCtx.AppId {
			// Merge devices
			existing.Devices = mergeDeviceLists(existing.Devices, newAppCtx.Devices)

			// Merge app-specific fields
			if existing.AppSpecificData == nil {
				existing.AppSpecificData = map[string]interface{}{}
			}
			for k, v := range newAppCtx.AppSpecificData {
				existing.AppSpecificData[k] = v
			}

			updatedAppData = append(updatedAppData, existing)
			updated = true
		} else {
			updatedAppData = append(updatedAppData, existing)
		}
	}

	// If no existing entry matched, add new one
	if !updated {
		updatedAppData = append(updatedAppData, newAppCtx)
	}

	// Step 3: Persist the updated application data
	update := bson.M{
		"$set": bson.M{
			"application_data": updatedAppData,
		},
	}
	_, err = repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileId}, update)
	if err != nil {
		return fmt.Errorf("failed to update application_data: %w", err)
	}

	return nil
}

func (repo *ProfileRepository) PatchAppContext(profileId, appID string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{fmt.Sprintf("application_data.%s", appID): bson.M{"$exists": true}, "profile_id": profileId}
	update := bson.M{"$set": bson.M{fmt.Sprintf("application_data.%s", appID): updates}}

	_, err := repo.Collection.UpdateOne(ctx, filter, update)
	return err
}

//func (repo *ProfileRepository) GetAppContext(profileId, appID string) (*models.ApplicationData, error) {
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	filter := bson.M{"profile_id": profileId}
//	projection := bson.M{"application_data": 1}
//
//	var profile models.Profile
//	err := repo.Collection.FindOne(ctx, filter, options.FindOne().SetProjection(projection)).Decode(&profile)
//	if err != nil {
//		if err == mongo.ErrNoDocuments {
//			return nil, nil
//		}
//		return nil, err
//	}
//
//	appData, exists := profile.ApplicationData[appID]
//	if !exists {
//		return nil, nil
//	}
//
//	return &appData, nil
//}

func decodeDevices(raw interface{}) []models.Devices {
	var devices []models.Devices

	if rawList, ok := raw.([]interface{}); ok {
		for _, item := range rawList {
			if deviceMap, ok := item.(map[string]interface{}); ok {
				device := models.Devices{}
				data, _ := json.Marshal(deviceMap)
				_ = json.Unmarshal(data, &device)
				devices = append(devices, device)
			}
		}
	}

	return devices
}

func mergeDeviceLists(existing, incoming []models.Devices) []models.Devices {
	deviceMap := make(map[string]models.Devices)
	for _, d := range existing {
		deviceMap[d.DeviceId] = d
	}
	for _, d := range incoming {
		deviceMap[d.DeviceId] = d
	}
	var merged []models.Devices
	for _, d := range deviceMap {
		merged = append(merged, d)
	}
	return merged
}

//func (repo *ProfileRepository) GetListOfAppContext(profileId string) ([]models.ApplicationData, error) {
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	filter := bson.M{"profile_id": profileId}
//	projection := bson.M{"application_data": 1}
//
//	var profile models.Profile
//	err := repo.Collection.FindOne(ctx, filter, options.FindOne().SetProjection(projection)).Decode(&profile)
//	if err == mongo.ErrNoDocuments {
//		return nil, nil
//	} else if err != nil {
//		return nil, fmt.Errorf("failed to retrieve application data: %w", err)
//	}
//
//	var flattened []map[string]interface{}
//
//	for _, app := range profile.ApplicationData {
//		flat := map[string]interface{}{
//			"app_id":  app.AppId,
//			"devices": app.Devices,
//		}
//
//		// Merge app_specific_data into the flat map
//		for k, v := range app.AppSpecificData {
//			flat[k] = v
//		}
//
//		flattened = append(flattened, flat)
//	}
//
//	return flattened, nil
//}

// AddOrUpdateTraitsData replaces (PUT) the personality data inside Profile
func (repo *ProfileRepository) AddOrUpdateTraitsData(profileId string, personalityData map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"profile_id": profileId}
	update := bson.M{"$set": bson.M{"traits": personalityData}}

	opts := options.Update().SetUpsert(true)
	_, err := repo.Collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	return nil
}

// UpsertIdentityData replaces (PUT) the personality data inside Profile
func (repo *ProfileRepository) UpsertIdentityData(profileId string, identityData map[string]interface{}) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"profile_id": profileId}
	updateFields := bson.M{}
	for k, v := range identityData {
		updateFields["identity_attributes."+k] = v
	}

	update := bson.M{"$set": updateFields}

	opts := options.Update().SetUpsert(true) // Insert if not found
	_, err := repo.Collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		logger.Error(err, "Failed to update personality data")
		return err
	}

	logger.Info("Identity Attributes data updated for user " + profileId)
	return nil
}

// GetAllProfiles retrieves all profiles from MongoDB
func (repo *ProfileRepository) GetAllProfiles() ([]models.Profile, error) {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Exclude profiles where profile_hierarchy.is_master == true
	// Only fetch profiles with profile_hierarchy.list_profile == true
	filter := bson.M{
		"profile_hierarchy.list_profile": true,
	}

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to fetch profiles: "+err.Error())
		return nil, err
	}
	defer cursor.Close(ctx)

	var profiles []models.Profile
	// 🔹 Decode all profiles
	if err = cursor.All(ctx, &profiles); err != nil {
		logger.Error(err, "Error decoding profiles: "+err.Error())
		return nil, err
	}

	logger.Info("Successfully fetched profiles")
	return profiles, nil
}

func (repo *ProfileRepository) GetAllProfilesWithMongoFilter(filter bson.M) ([]models.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var profiles []models.Profile
	if err := cursor.All(ctx, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func (repo *ProfileRepository) GetAllProfilesWithFilter(filters []string) ([]models.Profile, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, value := parts[0], strings.ToLower(parts[1]), parts[2]

		switch operator {
		case "eq":
			filter[field] = value
		case "sw":
			filter[field] = bson.M{"$regex": fmt.Sprintf("^%s", value)}
		case "co":
			filter[field] = bson.M{"$regex": value}
		}
	}

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var profile []models.Profile
	if err := cursor.All(ctx, &profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// GetAllMasterProfilesExceptForCurrent retrieves all master profiles excluding the current profile's parent
func (repo *ProfileRepository) GetAllMasterProfilesExceptForCurrent(currentProfile models.Profile) ([]models.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	excludedIDs := []string{}
	excludedIDs = append(excludedIDs, currentProfile.ProfileId)

	// Fetch only master profiles excluding the parent of the current profile
	filter := bson.M{
		"profile_hierarchy.is_parent": true,
		"profile_id": bson.M{
			"$nin": excludedIDs,
		},
	}

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var profiles []models.Profile
	if err = cursor.All(ctx, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

func (repo *ProfileRepository) UpdateParent(master models.Profile, newProfile models.Profile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateProfile := bson.M{
		"$set": bson.M{
			"profile_hierarchy.parent_profile_id": master.ProfileId,
			"profile_hierarchy.is_parent":         false,
		},
	}
	if _, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": newProfile.ProfileId}, updateProfile); err != nil {
		return fmt.Errorf("failed to update profile %s: %w", newProfile.ProfileId, err)
	}

	return nil
}

// LinkPeers creates a bidirectional link between two peer profiles
func (repo *ProfileRepository) LinkPeers(peerprofileId1 string, peerprofileId2 string, ruleName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// AddEventSchema peer2 to peer1
	peer2 := models.ChildProfile{
		ChildProfileId: peerprofileId2,
		RuleName:       ruleName,
	}
	updateProfile1 := bson.M{
		"$addToSet": bson.M{
			"profile_hierarchy.peer_profile_ids": peer2,
		},
	}
	if _, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": peerprofileId1}, updateProfile1); err != nil {
		return fmt.Errorf("failed to update peer profile for %s: %w", peerprofileId1, err)
	}

	// AddEventSchema peer1 to peer2
	peer1 := models.ChildProfile{
		ChildProfileId: peerprofileId1, // ✅ Corrected
		RuleName:       ruleName,
	}
	updateProfile2 := bson.M{
		"$addToSet": bson.M{
			"profile_hierarchy.peer_profile_ids": peer1,
		},
	}
	if _, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": peerprofileId2}, updateProfile2); err != nil {
		return fmt.Errorf("failed to update peer profile for %s: %w", peerprofileId2, err)
	}

	return nil
}

func (repo *ProfileRepository) FindProfileByUserName(userID string) (*models.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"identity.user_name": userID}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Profile not found is not an error
		}
		return nil, err
	}
	return &profile, nil
}

func (repo *ProfileRepository) AddChildProfile(parentProfile models.Profile, child models.ChildProfile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"profile_id": parentProfile.ProfileId}
	update := bson.M{
		"$addToSet": bson.M{
			"profile_hierarchy.child_profile_ids": child,
		},
	}

	_, err := repo.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to add child profile to parent %s: %w", parentProfile.ProfileId, err)
	}
	return nil
}

func (repo *ProfileRepository) UpsertIdentityAttribute(profileId string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch the current profile
	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		logger.Error(err, "Failed to fetch profile for identity update")
		return err
	}

	finalUpdates := bson.M{}

	for field, incomingVal := range updates {
		property := strings.Split(field, ".")
		propertyName := property[1]
		existingVal := profile.IdentityAttributes[propertyName]
		merged := enrichFieldValues(existingVal, incomingVal)
		finalUpdates[field] = merged
	}

	if len(finalUpdates) == 0 {
		return nil
	}

	_, err = repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileId}, bson.M{"$set": finalUpdates})
	if err != nil {
		logger.Error(err, "Failed to update identity attributes")
		return err
	}

	logger.Info("Identity attribute updated for user " + profileId)
	return nil
}

func (repo *ProfileRepository) UpsertTrait(profileId string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	finalUpdates := bson.M{}
	for field, incomingVal := range updates {
		traitPath := strings.Split(field, ".")
		traitName := traitPath[1]
		existingVal := profile.Traits[traitName]
		finalUpdates[field] = enrichFieldValues(existingVal, incomingVal)
	}

	if len(finalUpdates) == 0 {
		return nil
	}

	_, err = repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileId}, bson.M{"$set": finalUpdates})
	return err
}

func (repo *ProfileRepository) UpsertAppDatum(profileId string, appId string, update bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var profile models.Profile
	err := repo.Collection.FindOne(ctx, bson.M{"profile_id": profileId}).Decode(&profile)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	appIndex := -1
	for i, app := range profile.ApplicationData {
		if app.AppId == appId {
			appIndex = i
			break
		}
	}

	if appIndex != -1 {
		// App exists → merge
		existingApp := profile.ApplicationData[appIndex]
		finalSet := bson.M{}

		for key, incomingVal := range update {
			traitName := key
			if strings.HasPrefix(key, "application_data.") {
				traitName = strings.TrimPrefix(key, "application_data.")
			}
			existingVal := existingApp.AppSpecificData[traitName]
			fieldPath := fmt.Sprintf("application_data.%d.app_specific_data.%s", appIndex, traitName)
			finalSet[fieldPath] = enrichFieldValues(existingVal, incomingVal)
		}

		_, err := repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileId}, bson.M{"$set": finalSet})
		if err != nil {
			return fmt.Errorf("failed to update existing application_data entry: %w", err)
		}
		return nil
	}

	// App does not exist → insert
	appSpecific := bson.M{}
	for key, incomingVal := range update {
		key = strings.TrimPrefix(key, "application_data.app_specific_data.")
		appSpecific[key] = incomingVal
	}

	newApp := models.ApplicationData{
		AppId:           appId,
		AppSpecificData: appSpecific,
	}

	_, err = repo.Collection.UpdateOne(ctx, bson.M{"profile_id": profileId}, bson.M{
		"$push": bson.M{"application_data": newApp},
	})
	if err != nil {
		return fmt.Errorf("failed to insert new application_data entry: %w", err)
	}
	return nil
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
		return incoming // fallback
	}
}

func toStringSlice(val interface{}) []string {
	switch v := val.(type) {
	case []interface{}:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case primitive.A:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

func toIntSlice(val interface{}) []int {
	switch v := val.(type) {
	case []interface{}:
		var result []int
		for _, item := range v {
			if i, ok := toInt(item); ok {
				result = append(result, i)
			}
		}
		return result
	case primitive.A:
		var result []int
		for _, item := range v {
			if i, ok := toInt(item); ok {
				result = append(result, i)
			}
		}
		return result
	case []int:
		return v
	default:
		return nil
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
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}
