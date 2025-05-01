package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	errors "github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/logger"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/repository"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func CreateOrUpdateProfile(event models.Event) (*models.Profile, error) {

	mongoDB := locks.GetMongoDBInstance()
	profileRepo := repositories.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	lock := locks.GetDistributedLock()
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
	profile := models.Profile{
		ProfileId: event.ProfileId,
		ProfileHierarchy: &models.ProfileHierarchy{
			IsParent:    true,
			ListProfile: true,
		},
	}

	if err := profileRepo.InsertProfile(profile); err != nil {
		return nil, fmt.Errorf("failed to insert or ensure profile: %v", err)
	}

	profileFetched, err := waitForProfile(event.ProfileId, constants.MaxRetryAttempts, constants.RetryDelay)
	if err != nil || profileFetched == nil {
		return nil, fmt.Errorf("profile not visible after insert: %v", err)
	}

	return profileFetched, nil
}

// GetProfile retrieves a profile
func GetProfile(ProfileId string) (*models.Profile, error) {

	mongoDB := locks.GetMongoDBInstance()
	profileRepo := repositories.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	profile, _ := profileRepo.FindProfileByID(ProfileId)
	if profile == nil {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrProfileNotFound.Code,
			Message:     errors.ErrProfileNotFound.Message,
			Description: errors.ErrProfileNotFound.Description,
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
			return nil, errors.NewServerError(errors.ErrWhileFetchingProfile, err)
		}
		if masterProfile == nil {
			logger.Debug("Master profile is unfortunately empty")
			return nil, nil
		}
		return masterProfile, nil
	}
}

func buildProfileHierarchy(profile *models.Profile, masterProfile *models.Profile) *models.ProfileHierarchy {

	profileHierarchy := &models.ProfileHierarchy{
		IsParent:        false,
		ListProfile:     true,
		ParentProfileID: masterProfile.ProfileId,
		ChildProfiles:   []models.ChildProfile{},
	}
	if len(masterProfile.ProfileHierarchy.ChildProfiles) > 0 {
		profileHierarchy.ChildProfiles = masterProfile.ProfileHierarchy.ChildProfiles
	}
	return profileHierarchy
}

// DeleteProfile removes a profile from MongoDB by `perma_id`
func DeleteProfile(ProfileId string) error {
	mongoDB := locks.GetMongoDBInstance()
	profileRepo := repositories.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)
	eventRepo := repositories.NewEventRepository(mongoDB.Database, constants.EventCollection)

	// Fetch the existing profile before deletion
	profile, err := profileRepo.FindProfileByID(ProfileId)
	if profile == nil {
		logger.Info(fmt.Sprintf("Profile with profile_id: %s that is requested for deletion is not found",
			ProfileId))
		return nil
	}
	if err != nil {
		return errors.NewServerError(errors.ErrWhileFetchingProfile, err)
	}

	//  Delete related events
	if err := eventRepo.DeleteEventsByProfileId(ProfileId); err != nil {
		return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) == 0 {
		// Delete the master with no children
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
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
				return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
			}
			if err != nil {
				return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
			}
			err = profileRepo.DeleteProfile(childProfile.ChildProfileId)
			if err != nil {
				return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
			}
		}
		// now delete master
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
		}
		return nil
	}

	// If it is a child profile, delete it
	if !(profile.ProfileHierarchy.IsParent) {
		err = profileRepo.DetachChildFromParent(profile.ProfileHierarchy.ParentProfileID, ProfileId)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
		}
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors.NewServerError(errors.ErrWhileDeletingProfile, err)
		}
	}

	return nil
}

func waitForProfile(ProfileId string, maxRetries int, delay time.Duration) (*models.Profile, error) {
	profileRepo := repositories.NewProfileRepository(locks.GetMongoDBInstance().Database, constants.ProfileCollection)

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
func GetAllProfiles() ([]models.Profile, error) {
	mongoDB := locks.GetMongoDBInstance()
	profileRepo := repositories.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)

	existingProfiles, err := profileRepo.GetAllProfiles()
	if err != nil {
		return nil, errors.NewServerError(errors.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		return []models.Profile{}, nil
	}

	var result []models.Profile
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
func GetAllProfilesWithFilter(filters []string) ([]models.Profile, error) {

	mongoDB := locks.GetMongoDBInstance()
	profileRepo := repositories.NewProfileRepository(mongoDB.Database, constants.ProfileCollection)
	schemaRepo := repositories.NewProfileSchemaRepository(mongoDB.Database, constants.ProfileSchemaCollection)
	rules, err := schemaRepo.GetProfileEnrichmentRules()
	if err != nil {
		return nil, errors.NewServerError(errors.ErrWhileFetchingProfileEnrichmentRules, err)
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
		return nil, errors.NewServerError(errors.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		existingProfiles = []models.Profile{}
	}

	var result []models.Profile
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
