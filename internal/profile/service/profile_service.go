package service

import (
	"fmt"
	repositories "github.com/wso2/identity-customer-data-service/internal/events/store"
	"net/http"
	"strconv"
	"strings"
	"time"

	enrichmentModel "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	eventModel "github.com/wso2/identity-customer-data-service/internal/events/model"
	//repositories "github.com/wso2/identity-customer-data-service/internal/events/store"
	model2 "github.com/wso2/identity-customer-data-service/internal/profile/model"
	store2 "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"

	"github.com/wso2/identity-customer-data-service/internal/database"
)

// Define an interface for ProfileRepository to decouple the dependency
// This interface will be implemented by the actual repository in `store2`
type ProfileRepository interface {
	FindProfileByID(profileId string) (*model2.Profile, error)
	InsertProfile(profile model2.Profile) error
	DeleteProfile(profileId string) error
	GetAllProfiles() ([]model2.Profile, error)
	GetAllProfilesWithFilter(filters []string) ([]model2.Profile, error)
	DetachChildFromParent(parentID, childID string) error
}

// Update the interface to use the correct type
// Define an interface for ProfileSchemaRepository to decouple the dependency
// This interface will be implemented by the actual repository in `store`
type ProfileSchemaRepository interface {
	GetProfileEnrichmentRules() ([]enrichmentModel.ProfileEnrichmentRule, error)
}

func CreateOrUpdateProfile(event eventModel.Event) (*model2.Profile, error) {
	lock := database.GetDistributedLock()
	lockKey := "lock:profile:" + event.ProfileId

	// 🔁 Retry logic for acquiring the lock
	var acquired bool
	var err error
	for i := 0; i < constants.MaxRetryAttempts; i++ {
		acquired, err = lock.Acquire(lockKey, 1*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %v", err)
		}
		if acquired {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !acquired {
		return nil, fmt.Errorf("could not acquire lock for profile %s after retries", event.ProfileId)
	}
	defer lock.Release(lockKey)

	// Safe insert if not exists (upsert)
	profile := model2.Profile{
		ProfileId: event.ProfileId,
		ProfileHierarchy: &model2.ProfileHierarchy{
			IsParent:    true,
			ListProfile: true,
		},
	}

	mongoDB := database.GetMongoDBInstance()
	profileRepo := store2.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)
	if err := profileRepo.InsertProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to insert or ensure profile: %v", err)
	}

	profileFetched, err := WaitForProfile(event.ProfileId, constants.MaxRetryAttempts, constants.RetryDelay)
	if err != nil || profileFetched == nil {
		return nil, fmt.Errorf("profile not visible after insert: %v", err)
	}

	return profileFetched, nil
}

// GetProfile retrieves a profile
func GetProfile(ProfileId string) (*model2.Profile, error) {

	mongoDB := database.GetMongoDBInstance()
	profileRepo := store2.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	profile, _ := profileRepo.FindProfileByID(ProfileId)
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
		masterProfile, err := profileRepo.FindProfileByID(profile.ProfileHierarchy.ParentProfileID)

		// todo: app context should be restricted for apps that is requesting these
		// setting the current profile hierarchy to the master profile
		masterProfile.ProfileHierarchy = buildProfileHierarchy(profile, masterProfile)
		masterProfile.ProfileId = profile.ProfileId
		if err != nil {
			return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
		}
		if masterProfile == nil {
			logger.Debug("Master profile is unfortunately empty")
			return nil, nil
		}
		return masterProfile, nil
	}
}

func buildProfileHierarchy(profile *model2.Profile, masterProfile *model2.Profile) *model2.ProfileHierarchy {

	profileHierarchy := &model2.ProfileHierarchy{
		IsParent:        false,
		ListProfile:     true,
		ParentProfileID: masterProfile.ProfileId,
		ChildProfiles:   []model2.ChildProfile{},
	}
	if len(masterProfile.ProfileHierarchy.ChildProfiles) > 0 {
		profileHierarchy.ChildProfiles = masterProfile.ProfileHierarchy.ChildProfiles
	}
	return profileHierarchy
}

// DeleteProfile removes a profile from MongoDB by `perma_id`
func DeleteProfile(ProfileId string) error {
	mongoDB := database.GetMongoDBInstance()
	profileRepo := store2.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)
	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories.NewEventRepository(postgresDB.DB)

	// Fetch the existing profile before deletion
	profile, err := profileRepo.FindProfileByID(ProfileId)
	if profile == nil {
		logger.Info(fmt.Sprintf("Profile with profile_id: %s that is requested for deletion is not found",
			ProfileId))
		return nil
	}
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}

	//  Delete related events
	if err := eventRepo.DeleteEventsByProfileId(ProfileId); err != nil {
		return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) == 0 {
		// Delete the master with no children
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
		}
		return nil
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) > 0 {
		//get all child profiles and delete
		for _, childProfile := range profile.ProfileHierarchy.ChildProfiles {
			profile, err := profileRepo.FindProfileByID(childProfile.ChildProfileId)
			if profile == nil {
				logger.Debug("Child profile with profile_id: %s that is being deleted is not found",
					childProfile.ChildProfileId)
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
			if err != nil {
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
			err = profileRepo.DeleteProfile(childProfile.ChildProfileId)
			if err != nil {
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
		}
		// now delete master
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
		}
		return nil
	}

	// If it is a child profile, delete it
	if !(profile.ProfileHierarchy.IsParent) {
		err = profileRepo.DetachChildFromParent(profile.ProfileHierarchy.ParentProfileID, ProfileId)
		if err != nil {
			return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
		}
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
		}
	}

	return nil
}

func WaitForProfile(ProfileId string, maxRetries int, delay time.Duration) (*model2.Profile, error) {
	profileRepo := store2.NewProfileRepository(database.GetMongoDBInstance().Database, constants.ProfileCollection)

	for i := 0; i < maxRetries; i++ {
		profile, err := profileRepo.FindProfileByID(ProfileId)
		if err != nil {
			return nil, err
		}
		if profile != nil {
			return profile, nil
		}
		time.Sleep(delay)
	}
	return nil, nil
}

// GetAllProfiles retrieves all profiles
func GetAllProfiles() ([]model2.Profile, error) {
	mongoDB := database.GetMongoDBInstance()
	profileRepo := store2.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	existingProfiles, err := profileRepo.GetAllProfiles()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		return []model2.Profile{}, nil
	}

	var result []model2.Profile
	for _, profile := range existingProfiles {
		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and assign current profile ID
			master, err := profileRepo.FindProfileByID(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}
			master.ProfileHierarchy = buildProfileHierarchy(&profile, master)
			master.ProfileId = profile.ProfileId
			result = append(result, *master)
		}
	}
	return result, nil
}

// GetAllProfilesWithFilter handles fetching all profiles with filter
func GetAllProfilesWithFilter(filters []string) ([]model2.Profile, error) {

	mongoDB := database.GetMongoDBInstance()
	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	profileRepo := store2.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	rules, err := schemaRepo.GetProfileEnrichmentRules()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfileEnrichmentRules, err)
	}

	// Step 2: Build trait → valueType mapping
	propertyTypeMap := make(map[string]string)
	for _, rule := range rules {
		propertyTypeMap[rule.PropertyName] = rule.ValueType
	}

	// Step 3: Rewrite filters with correct parsed types
	var updatedFilters []string
	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, rawValue := parts[0], parts[1], parts[2]
		valueType := propertyTypeMap[field]
		parsed := parseTypedValueForFilters(valueType, rawValue)

		// Prepare updated filter string
		var valueStr string
		switch v := parsed.(type) {
		case string:
			valueStr = v
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		updatedFilters = append(updatedFilters, fmt.Sprintf("%s %s %s", field, operator, valueStr))
	}

	// Step 4: Pass updated filters to repo
	existingProfiles, err := profileRepo.GetAllProfilesWithFilter(updatedFilters)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		existingProfiles = []model2.Profile{}
	}

	var result []model2.Profile
	for _, profile := range existingProfiles {
		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and assign current profile ID
			master, err := profileRepo.FindProfileByID(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}
			master.ProfileHierarchy = buildProfileHierarchy(&profile, master)
			master.ProfileId = profile.ProfileId
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
