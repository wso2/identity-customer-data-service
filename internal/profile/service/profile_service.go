package service

import (
	"context"
	"fmt"
	repositories "github.com/wso2/identity-customer-data-service/internal/events/store"
	"log"
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

	ctx := context.Background()
	postgresDB := database.GetPostgresInstance()
	conn, err := postgresDB.DB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get DB connection: %w", err)
	}
	defer conn.Close() // ensures lock is released and session is closed

	// Create a lock tied to this connection
	lock := database.NewPostgresLock(conn)
	lockIdentifier := event.ProfileId

	//  Attempt to acquire the lock with retry
	var acquired bool
	for i := 0; i < constants.MaxRetryAttempts; i++ {
		acquired, err = lock.Acquire(lockIdentifier)
		if err != nil {
			return nil, fmt.Errorf("lock acquisition error for %s: %w", event.ProfileId, err)
		}
		if acquired {
			break
		}
		time.Sleep(constants.RetryDelay)
	}
	if !acquired {
		return nil, fmt.Errorf("could not acquire lock for profile %s after %d retries", event.ProfileId, constants.MaxRetryAttempts)
	}
	defer func() {
		_ = lock.Release(lockIdentifier) //  Always attempt to release
	}()

	//  Insert/update using standard DB (does not have to use same conn unless needed)
	profileRepo := store2.NewProfileRepository(postgresDB.DB)
	profileToUpsert := model2.Profile{
		ProfileId: event.ProfileId,
		ProfileHierarchy: &model2.ProfileHierarchy{
			IsParent:    true,
			ListProfile: true,
		},
		IdentityAttributes: make(map[string]interface{}),
		Traits:             make(map[string]interface{}),
		ApplicationData:    []model2.ApplicationData{},
	}

	if err := profileRepo.InsertProfile(profileToUpsert); err != nil {
		return nil, fmt.Errorf("failed to insert or update profile %s: %w", event.ProfileId, err)
	}

	profileFetched, errWait := WaitForProfile(event.ProfileId, constants.MaxRetryAttempts, constants.RetryDelay)
	if errWait != nil || profileFetched == nil {
		return nil, fmt.Errorf("profile %s not visible after insert/update: %w", event.ProfileId, errWait)
	}

	return profileFetched, nil
}

// GetProfile retrieves a profile
func GetProfile(ProfileId string) (*model2.Profile, error) {

	postgresDB := database.GetPostgresInstance()
	profileRepo := store2.NewProfileRepository(postgresDB.DB)

	profile, _ := profileRepo.GetProfile(ProfileId)
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
		masterProfile, err := profileRepo.GetProfile(profile.ProfileHierarchy.ParentProfileID)
		// todo: app context should be restricted for apps that is requesting these

		masterProfile.ApplicationData, _ = profileRepo.FetchApplicationData(masterProfile.ProfileId)

		// building the hierarchy
		masterProfile.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(masterProfile.ProfileId)
		masterProfile.ProfileHierarchy.ParentProfileID = masterProfile.ProfileId
		masterProfile.ProfileId = profile.ProfileId

		if err != nil {
			return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
		}
		if masterProfile == nil {
			return nil, nil
		}
		return masterProfile, nil
	}
}

// DeleteProfile removes a profile from MongoDB by `perma_id`
func DeleteProfile(ProfileId string) error {

	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories.NewEventRepository(postgresDB.DB)
	profileRepo := store2.NewProfileRepository(postgresDB.DB)

	// Fetch the existing profile before deletion
	profile, err := profileRepo.GetProfile(ProfileId)
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

	if profile.ProfileHierarchy.IsParent {
		// fetching the child if its parent
		profile.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(profile.ProfileId)
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) == 0 {
		// Delete the parent with no children
		err = profileRepo.DeleteProfile(ProfileId)
		if err != nil {
			return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
		}
		return nil
	}

	if profile.ProfileHierarchy.IsParent && len(profile.ProfileHierarchy.ChildProfiles) > 0 {
		//get all child profiles and delete
		for _, childProfile := range profile.ProfileHierarchy.ChildProfiles {
			profile, err := profileRepo.GetProfile(childProfile.ChildProfileId)
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
		parentProfile, err := profileRepo.GetProfile(profile.ProfileHierarchy.ParentProfileID)
		parentProfile.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(parentProfile.ProfileId)

		if len(parentProfile.ProfileHierarchy.ChildProfiles) == 1 {
			// delete the parent as this is the only child
			err = profileRepo.DeleteProfile(profile.ProfileHierarchy.ParentProfileID)
			err = profileRepo.DeleteProfile(ProfileId)
			if err != nil {
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
		} else {
			err = profileRepo.DetachChildProfileFromParent(profile.ProfileHierarchy.ParentProfileID, ProfileId)
			if err != nil {
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
			err = profileRepo.DeleteProfile(ProfileId)
			if err != nil {
				return errors2.NewServerError(errors2.ErrWhileDeletingProfile, err)
			}
		}

	}

	return nil
}

func WaitForProfile(profileID string, maxRetries int, retryDelay time.Duration) (*model2.Profile, error) {

	var profile *model2.Profile
	var lastErr error
	postgresDB := database.GetPostgresInstance()
	profileRepo := store2.NewProfileRepository(postgresDB.DB)

	for i := 0; i < maxRetries; i++ {
		if i > 0 { // Only sleep on subsequent retries
			time.Sleep(retryDelay)
		}
		profile, lastErr = profileRepo.GetProfile(profileID) // Assuming GetProfile is a method on profileRepo
		if profile != nil {
			return profile, nil
		}
		if lastErr != nil {
			log.Print("waitForProfile: Error during fetch attempt", "profileId", profileID, "attempt", i+1, "error", lastErr)
			// Continue to retry, lastErr will be reported if all retries fail
		}
	}

	// logger.Error("waitForProfile: Profile not visible after all retries", "profileId", profileID, "attempts", maxRetries)
	if lastErr != nil {
		return nil, fmt.Errorf("profile %s not visible after %d retries, last error: %w", profileID, maxRetries, lastErr)
	}
	return nil, fmt.Errorf("profile %s not visible after %d retries", profileID, maxRetries)
}

// GetAllProfiles retrieves all profiles
func GetAllProfiles() ([]model2.Profile, error) {

	postgresDB := database.GetPostgresInstance() // Your method to get *sql.DB wrapped or raw
	profileRepo := store2.NewProfileRepository(postgresDB.DB)

	existingProfiles, err := profileRepo.GetAllProfiles()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if existingProfiles == nil {
		return []model2.Profile{}, nil
	}

	// todo: app context should be restricted for apps that is requesting these

	var result []model2.Profile
	for _, profile := range existingProfiles {
		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and assign current profile ID
			master, err := profileRepo.GetProfile(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}

			master.ApplicationData, _ = profileRepo.FetchApplicationData(master.ProfileId)

			// building the hierarchy
			master.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(master.ProfileId)
			master.ProfileId = profile.ProfileId
			master.ProfileHierarchy.ParentProfileID = master.ProfileId

			result = append(result, *master)
		}
	}
	return result, nil
}

// GetAllProfilesWithFilter handles fetching all profiles with filter
func GetAllProfilesWithFilter(filters []string) ([]model2.Profile, error) {

	postgresDB := database.GetPostgresInstance()
	schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
	profileRepo := store2.NewProfileRepository(postgresDB.DB)

	// Step 1: Fetch enrichment rules to extract value types
	rules, err := schemaRepo.GetProfileEnrichmentRules()
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
	filteredProfiles, err := profileRepo.GetAllProfilesWithFilter(rewrittenFilters)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileFetchingProfile, err)
	}
	if filteredProfiles == nil {
		filteredProfiles = []model2.Profile{}
	}

	var result []model2.Profile
	for _, profile := range filteredProfiles {
		if !profile.ProfileHierarchy.ListProfile {
			continue
		}

		if profile.ProfileHierarchy.IsParent {
			result = append(result, profile)
		} else {
			// Fetch master and attach current profile context
			master, err := profileRepo.GetProfile(profile.ProfileHierarchy.ParentProfileID)
			if err != nil || master == nil {
				continue
			}

			master.ApplicationData, _ = profileRepo.FetchApplicationData(master.ProfileId)
			master.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(master.ProfileId)

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
