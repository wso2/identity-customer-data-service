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

package integration

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	enrModel "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	enrService "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
	eventModel "github.com/wso2/identity-customer-data-service/internal/events/model"
	eventService "github.com/wso2/identity-customer-data-service/internal/events/service"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	profileworker "github.com/wso2/identity-customer-data-service/internal/system/workers"
	"testing"
	"time"
)

func Test_Profiles(t *testing.T) {

	logger := log.GetLogger()
	enrichmentSvc := enrService.GetEnrichmentRuleService()
	profileSvc := profileService.GetProfilesService()
	eventSvc := eventService.GetEventsService()
	queue := &profileworker.ProfileWorkerQueue{}

	emailRule := enrModel.ProfileEnrichmentRule{
		RuleId:            "email_extract_rule",
		PropertyName:      "identity_attributes.email",
		ValueType:         "arrayOfString",
		MergeStrategy:     "combine",
		ComputationMethod: "extract",
		SourceField:       "email",
		Trigger: enrModel.RuleTrigger{
			EventType: "identify",
			EventName: "user_logged_in",
		},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	err := enrichmentSvc.AddEnrichmentRule(emailRule)
	require.NoError(t, err, "Failed to add email enrichment rule")

	// Define first profile via event ingestion
	profileID1 := uuid.New().String()
	eventId := uuid.New().String()
	appId := uuid.New().String()
	logger.Debug("Creating profile with ID: " + profileID1)
	logger.Debug("Creating profile with ID: " + eventId)
	deviceID := uuid.New().String()
	event1 := eventModel.Event{
		ProfileId: profileID1,
		EventId:   eventId,
		EventType: "identify",
		EventName: "user_logged_in",
		OrgId:     "carbon.super",
		AppId:     appId,
		Context: map[string]interface{}{
			"browser":   "Chrome",
			"os":        "macOS",
			"device_id": deviceID,
		},
		Properties: map[string]interface{}{
			"email":      "test-cds@wso2.com",
			"first_name": "Test",
			"last_name":  "User",
			"user_id":    "4e04c4f1-c0e6-43aa-aeb0-19b5c883a420",
			"user_name":  "admin",
		},
	}

	t.Run("Create profile via event", func(t *testing.T) {
		err := eventSvc.AddEvents(event1, queue)
		require.NoError(t, err, "Failed to ingest event and create profile")
	})

	t.Run("Fetch created profile", func(t *testing.T) {
		time.Sleep(5000 * time.Millisecond) // to ensure the event is processed
		profile, err := profileSvc.GetProfile(profileID1)
		logger.Debug("Fetched profile: " + profile.ProfileId)
		require.NoError(t, err, "Failed to fetch profile")
		require.NotNil(t, profile)
		require.Equal(t, profileID1, profile.ProfileId)
		require.Equal(t, "test-cds@wso2.com", profile.IdentityAttributes["email"].([]interface{})[0])
		logger.Debug("Fetched profile email: " + profile.IdentityAttributes["email"].([]interface{})[0].(string))
		require.Equal(t, deviceID, profile.ApplicationData[0].Devices[0].DeviceId)
		require.Equal(t, "Chrome", profile.ApplicationData[0].Devices[0].Browser)
	})

	t.Run("Get all profiles", func(t *testing.T) {
		profiles, err := profileSvc.GetAllProfiles()
		require.NoError(t, err)
		require.NotEmpty(t, profiles)
	})

	t.Run("Filter profiles by email", func(t *testing.T) {
		filter := []string{"identity_attributes.email co test-cds@wso2.com"}
		filteredProfiles, err := profileSvc.GetAllProfilesWithFilter(filter)
		require.NoError(t, err)
		require.Len(t, filteredProfiles, 1)
		require.Equal(t, profileID1, filteredProfiles[0].ProfileId)
	})

	t.Run("Delete profile", func(t *testing.T) {
		err := profileSvc.DeleteProfile(profileID1)
		require.NoError(t, err)

		deleted, err := profileSvc.GetProfile(profileID1)
		require.Error(t, err)
		require.Nil(t, deleted)
	})
}
