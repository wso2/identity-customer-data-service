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
	"log"

	//"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	enrichmentModel "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	enrichmentService "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
	eventModel "github.com/wso2/identity-customer-data-service/internal/events/model"
	eventService "github.com/wso2/identity-customer-data-service/internal/events/service"
	"github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	unificationModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	unificationService "github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
	"testing"
	"time"
)

func Test_Profile_Enrichment_And_Unification(t *testing.T) {
	//ctx := context.Background()
	workers.StartProfileWorker()
	enrichmentSvc := enrichmentService.GetEnrichmentRuleService()
	unificationSvc := unificationService.GetUnificationRuleService()
	profileSvc := service.GetProfilesService()
	rules := []enrichmentModel.ProfileEnrichmentRule{
		{
			PropertyName:      "identity_attributes.email",
			ValueType:         "arrayOfString",
			ComputationMethod: "extract",
			SourceField:       "email",
			MergeStrategy:     "combine",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "identify",
				EventName: "user_logged_in",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "identity_attributes.email",
			ValueType:         "arrayOfString",
			ComputationMethod: "extract",
			SourceField:       "email",
			MergeStrategy:     "combine",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "track",
				EventName: "newsletter_subscribed",
				Conditions: []enrichmentModel.RuleCondition{{
					Field:    "newsletter_subscribed",
					Operator: "equals",
					Value:    "true",
				}},
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "identity_attributes.phone_number",
			ValueType:         "arrayOfString",
			ComputationMethod: "extract",
			SourceField:       "mobile_number",
			MergeStrategy:     "combine",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "track",
				EventName: "purchase_initiated",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "identity_attributes.phone_number",
			ValueType:         "arrayOfString",
			ComputationMethod: "extract",
			SourceField:       "mobile_number",
			MergeStrategy:     "combine",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "identify",
				EventName: "user_logged_in",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "identity_attributes.user_name",
			ValueType:         "string",
			ComputationMethod: "extract",
			SourceField:       "user_name",
			MergeStrategy:     "overwrite",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "identify",
				EventName: "user_logged_in",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "identity_attributes.user_id",
			ValueType:         "string",
			ComputationMethod: "extract",
			SourceField:       "user_id",
			MergeStrategy:     "overwrite",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "identify",
				EventName: "user_logged_in",
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			PropertyName:      "traits.interests",
			ValueType:         "arrayOfString",
			ComputationMethod: "extract",
			SourceField:       "objectname",
			MergeStrategy:     "combine",
			Trigger: enrichmentModel.RuleTrigger{
				EventType: "track",
				EventName: "category_searched",
				Conditions: []enrichmentModel.RuleCondition{{
					Field:    "action",
					Operator: "equals",
					Value:    "select_category",
				}},
			},
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	for _, rule := range rules {
		_ = enrichmentSvc.AddEnrichmentRule(rule)
	}

	rulesU := []unificationModel.UnificationRule{
		{
			RuleId:    uuid.New().String(),
			RuleName:  "user id based",
			Property:  "identity_attributes.user_id",
			Priority:  1,
			IsActive:  true,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			RuleId:    uuid.New().String(),
			RuleName:  "email based",
			Property:  "identity_attributes.email",
			Priority:  2,
			IsActive:  true,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			RuleId:    uuid.New().String(),
			RuleName:  "phone based",
			Property:  "identity_attributes.phone_number",
			Priority:  3,
			IsActive:  true,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}
	for _, rule := range rulesU {
		_ = unificationSvc.AddUnificationRule(rule)
	}

	profile1ID := "profile_1"
	profile2ID := "profile_2"
	profile3ID := "profile_3"
	profile4ID := "profile_4"
	device1Id := "device_1"
	device2Id := "device_2"
	device3Id := "device_3"
	device4Id := "device_4"
	app1Id := "app_1"
	app2Id := "app_2"

	eventsSvc := eventService.GetEventsService()
	eventQueue := &workers.ProfileWorkerQueue{}

	events := []eventModel.Event{
		{
			ProfileId:      profile1ID,
			EventId:        uuid.New().String(),
			EventType:      "page",
			EventName:      "page_visited",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Context: map[string]interface{}{
				"device_id": device1Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
			Properties: map[string]interface{}{
				"url": "http://localhost:3000/",
			},
		},
		{
			ProfileId:      profile1ID,
			EventId:        uuid.New().String(),
			EventType:      "identify",
			EventName:      "guest_user_session",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"device_id": device1Id,
				"username":  "anon-happy-turtle",
			},
			Context: map[string]interface{}{
				"device_id": device1Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},
		{
			ProfileId:      profile1ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "category_searched",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"action":     "select_category",
				"objecttype": "category",
				"objectname": "A",
			},
			Context: map[string]interface{}{
				"device_id": device1Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},
		{
			ProfileId:      profile1ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "newsletter_subscribed",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"email":                 "email1@gmail.com",
				"username":              "anon-happy-turtle",
				"newsletter_subscribed": true,
			},
			Context: map[string]interface{}{
				"device_id": device1Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},
		{
			ProfileId:      profile1ID,
			EventId:        uuid.New().String(),
			EventType:      "identify",
			EventName:      "user_logged_in",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"email":         "email2@gmail.com",
				"mobile_number": "07x1234567",
				"user_name":     "user1",
				"user_id":       "user-1-id",
			},
			Context: map[string]interface{}{
				"device_id": device1Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},

		// Second profile events - same app , different device : prof1/prof2- mobile based unify
		{
			ProfileId:      profile2ID,
			EventId:        uuid.New().String(),
			EventType:      "page",
			EventName:      "page_visited",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Context: map[string]interface{}{
				"browser":   "Chrome",
				"os":        "macOS",
				"device_id": device2Id,
			},
			Properties: map[string]interface{}{
				"url": "http://localhost:3000/",
			},
		},
		{
			ProfileId:      profile2ID,
			EventId:        uuid.New().String(),
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			EventType:      "track",
			EventName:      "purchase_initiated",
			Context: map[string]interface{}{
				"browser":   "Chrome",
				"os":        "macOS",
				"device_id": device2Id,
			},
			Properties: map[string]interface{}{
				"offer_value":    50,
				"item_name":      "Toy 5",
				"original_price": "24.46",
				"final_price":    "18.34",
				"mobile_number":  "07x1234567",
			},
		},
		{
			ProfileId:      profile2ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "category_searched",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"action":     "select_category",
				"objecttype": "category",
				"objectname": "B",
			},
			Context: map[string]interface{}{
				"device_id": device2Id,
				"os":        "macOS",
				"browser":   "FireFix",
			},
		},

		// third profile events - same app , different device : prof1/prof3- email based unify
		{
			ProfileId:      profile3ID,
			EventId:        uuid.New().String(),
			EventType:      "page",
			EventName:      "page_visited",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Context: map[string]interface{}{
				"device_id": device3Id,
				"os":        "Linux",
				"browser":   "Chromium",
			},
			Properties: map[string]interface{}{
				"url": "http://localhost:3000/",
			},
		},
		{
			ProfileId:      profile3ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "category_searched",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"action":     "select_category",
				"objecttype": "category",
				"objectname": "C",
			},
			Context: map[string]interface{}{
				"device_id": device3Id,
				"os":        "Linux",
				"browser":   "Chromium",
			},
		},
		{
			ProfileId:      profile3ID,
			EventId:        uuid.New().String(),
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			EventType:      "track",
			EventName:      "purchase_initiated",
			Context: map[string]interface{}{
				"device_id": device3Id,
				"os":        "Linux",
				"browser":   "Chromium",
			},
			Properties: map[string]interface{}{
				"offer_value":    50,
				"item_name":      "Toy 5",
				"original_price": "24.46",
				"final_price":    "18.34",
				"mobile_number":  "07x9876543",
			},
		},
		{
			ProfileId:      profile3ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "newsletter_subscribed",
			AppId:          app1Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"email":                 "email1@gmail.com",
				"username":              "anon-happy-turtle",
				"newsletter_subscribed": true,
			},
			Context: map[string]interface{}{
				"device_id": device3Id,
				"os":        "Linux",
				"browser":   "Chromium",
			},
		},

		// Fourth profile events - different app , different device - user id based unify
		{
			ProfileId:      profile4ID,
			EventId:        uuid.New().String(),
			EventType:      "page",
			EventName:      "page_visited",
			AppId:          app2Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Context: map[string]interface{}{
				"device_id": device4Id,
				"os":        "macOS",
				"browser":   "FireFix",
			},
			Properties: map[string]interface{}{
				"url": "http://localhost:3000/",
			},
		},
		{
			ProfileId:      profile4ID,
			EventId:        uuid.New().String(),
			EventType:      "track",
			EventName:      "category_searched",
			AppId:          app2Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"action":     "select_category",
				"objecttype": "category",
				"objectname": "D",
			},
			Context: map[string]interface{}{
				"device_id": device4Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},
		{
			ProfileId:      profile4ID,
			EventId:        uuid.New().String(),
			EventType:      "identify",
			EventName:      "user_logged_in",
			AppId:          app2Id,
			OrgId:          "carbon.super",
			EventTimestamp: int(time.Now().Unix()),
			Properties: map[string]interface{}{
				"user_name": "user1",
				"user_id":   "user-1-id",
			},
			Context: map[string]interface{}{
				"device_id": device4Id,
				"os":        "macOS",
				"browser":   "Chrome",
			},
		},
	}

	for _, e := range events {
		time.Sleep(2 * time.Second) // to ensure the event is processed
		err := eventsSvc.AddEvents(e, eventQueue)
		require.NoError(t, err)
	}

	t.Run("Profile_Enrichment_And_Unification", func(t *testing.T) {
		time.Sleep(10 * time.Second) // to ensure the event is processed

		// Fetch all profiles
		allProfiles, err := profileSvc.GetAllProfiles()
		require.NoError(t, err)
		require.Len(t, allProfiles, 4, "Expected 4 profiles")

		// Ensure all non-parent profiles point to the same master profile
		var ParentProfileID string
		for _, profile := range allProfiles {
			if ParentProfileID == "" {
				ParentProfileID = profile.ProfileHierarchy.ParentProfileID
			} else {
				require.Equal(t, ParentProfileID, profile.ProfileHierarchy.ParentProfileID, "Profile %s does not share the same master", profile.ProfileId)
			}
		}

		// Now fetch the master profile
		profile1, err := profileSvc.GetProfile(profile1ID)
		require.NoError(t, err)
		require.NotNil(t, profile1)

		// Ensure master profile has correct child profiles (profile1, 2, 3, 4)
		childIDs := map[string]bool{}
		for _, child := range profile1.ProfileHierarchy.ChildProfiles {
			childIDs[child.ChildProfileId] = true
		}
		require.Contains(t, childIDs, profile1ID)
		require.Contains(t, childIDs, profile2ID)
		require.Contains(t, childIDs, profile3ID)
		require.Contains(t, childIDs, profile4ID)

		// Expected rule mappings
		expectedRules := map[string]string{
			profile1ID: "phone based", // profile1 unified via phone with profile2
			profile2ID: "phone based",
			profile3ID: "email based",   // profile3 unified via email
			profile4ID: "user id based", // profile4 unified via user ID
		}

		for _, child := range profile1.ProfileHierarchy.ChildProfiles {
			expectedRule, ok := expectedRules[child.ChildProfileId]
			require.True(t, ok, "Unexpected child profile found: %s", child.ChildProfileId)
			require.Equal(t, expectedRule, child.RuleName, "Incorrect rule used for child profile: %s", child.ChildProfileId)
		}

		// Check enrichment: email and phone
		emails, ok := profile1.IdentityAttributes["email"].([]interface{})
		require.True(t, ok, "email not enriched correctly")
		require.Contains(t, emails, "email1@gmail.com")
		require.Contains(t, emails, "email2@gmail.com")

		phones, ok := profile1.IdentityAttributes["phone_number"].([]interface{})
		require.True(t, ok, "phone_number not enriched correctly")
		require.Contains(t, phones, "07x1234567")
		require.Contains(t, phones, "07x9876543")

		// Check interest trait
		interests, ok := profile1.Traits["interests"].([]interface{})
		require.True(t, ok, "interests not enriched")
		require.ElementsMatch(t, interests, []interface{}{"A", "B", "C", "D"})

		// Check user_name and user_id
		require.Equal(t, "user1", profile1.IdentityAttributes["user_name"])
		require.Equal(t, "user-1-id", profile1.IdentityAttributes["user_id"])

		//  Check merged application data
		for _, appData := range profile1.ApplicationData {
			log.Print("AppId: ", appData.AppId)
		}

		for _, device := range profile1.ApplicationData[0].Devices {
			log.Print("device id:", device.DeviceId)
		}

		require.Len(t, profile1.ApplicationData, 2, "Expected 2 application entries in master profile")

		deviceSet := map[string]bool{}
		appSet := map[string]bool{}

		for _, appData := range profile1.ApplicationData {
			appSet[appData.AppId] = true
			for _, device := range appData.Devices {
				deviceSet[device.DeviceId] = true
			}
		}

		require.Contains(t, appSet, app1Id, "App1 not found in application data")
		require.Contains(t, appSet, app2Id, "App2 not found in application data")

		require.Contains(t, deviceSet, device1Id, "Device1 not merged")
		require.Contains(t, deviceSet, device2Id, "Device2 not merged")
		require.Contains(t, deviceSet, device3Id, "Device3 not merged")
		require.Contains(t, deviceSet, device4Id, "Device4 not merged")

		require.Len(t, deviceSet, 4, "Expected 4 unique devices in merged application data")
	})

}
