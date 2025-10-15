package integration

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	"testing"
)

func Test_Profiles(t *testing.T) {

	profileSvc := profileService.GetProfilesService()

	// Define profile creation data
	profileID1 := uuid.New().String()
	email := "test-cds@wso2.com"
	profileRequest := profileModel.ProfileRequest{
		UserId:             "4e04c4f1-c0e6-43aa-aeb0-19b5c883a420",
		IdentityAttributes: map[string]interface{}{"email": []string{email}},
		Traits:             map[string]interface{}{"role": []string{"admin"}},
		ApplicationData:    map[string]map[string]interface{}{"app1": {"device_id": "device1", "browser": "Chrome"}},
	}

	t.Run("Create_Profile", func(t *testing.T) {
		// Create profile
		profile, err := profileSvc.CreateProfile(profileRequest, "carbon.super")
		require.NoError(t, err, "Failed to create profile")
		require.NotNil(t, profile)
		require.Equal(t, profileID1, profile.ProfileId)
		require.Equal(t, email, profile.IdentityAttributes["email"].([]interface{})[0])
	})

	// Fetch the profile after creation
	t.Run("Fetch_Created_Profile", func(t *testing.T) {
		// Fetch created profile by ID
		profile, err := profileSvc.GetProfile(profileID1)
		require.NoError(t, err, "Failed to fetch created profile")
		require.NotNil(t, profile)
		require.Equal(t, profileID1, profile.ProfileId)
		require.Equal(t, email, profile.IdentityAttributes["email"].([]interface{})[0])
	})

	// Update the profile
	t.Run("Update_Profile", func(t *testing.T) {
		updatedProfileRequest := profileModel.ProfileRequest{
			IdentityAttributes: map[string]interface{}{"email": []string{"updated-email@wso2.com"}},
			Traits:             map[string]interface{}{"role": []string{"admin", "user"}},
		}
		profile, err := profileSvc.UpdateProfile(profileID1, updatedProfileRequest)
		require.NoError(t, err, "Failed to update profile")
		require.Equal(t, "updated-email@wso2.com", profile.IdentityAttributes["email"].([]interface{})[0])
	})

	// Delete the profile
	t.Run("Delete_Profile", func(t *testing.T) {
		err := profileSvc.DeleteProfile(profileID1)
		require.NoError(t, err, "Failed to delete profile")

		// Try fetching the deleted profile
		profile, err := profileSvc.GetProfile(profileID1)
		require.Error(t, err, "Expected error after deleting profile")
		require.Nil(t, profile)
	})
}
