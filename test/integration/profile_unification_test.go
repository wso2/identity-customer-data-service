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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	unificationService "github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

// Helper for unmarshalling JSON into ProfileRequest
func mustUnmarshalProfile(jsonStr string) profileModel.ProfileRequest {
	var p profileModel.ProfileRequest
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		panic(err)
	}
	return p
}

func Test_Profile_Unification_Scenarios(t *testing.T) {

	PhoneBased := "phone_based"
	EmailBased := "email_based"
	AppId := "test-app-001"
	SuperTenantOrg := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())

	// Initialize Profile Schema Attributes
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	identityAttr := []schemaModel.ProfileSchemaAttribute{
		{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(), AttributeName: "identity_attributes.email", ValueType: constants.StringDataType, MergeStrategy: "combine",
			Mutability: constants.MutabilityReadWrite, MultiValued: true},
		{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(), AttributeName: "identity_attributes.phone_number", ValueType: constants.StringDataType,
			MergeStrategy: "combine", Mutability: constants.MutabilityReadWrite, MultiValued: true},
		{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(), AttributeName: "identity_attributes.user_id", ValueType: constants.StringDataType, MergeStrategy: "combine",
			Mutability: constants.MutabilityReadWrite},
	}

	traits := []schemaModel.ProfileSchemaAttribute{
		{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(), AttributeName: "traits.interests", ValueType: constants.StringDataType, MergeStrategy: "combine",
			Mutability: constants.MutabilityReadWrite, MultiValued: true},
	}

	appData := []schemaModel.ProfileSchemaAttribute{
		{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(), AttributeName: "application_data.device_id", ValueType: constants.StringDataType, MergeStrategy: "combine",
			Mutability: constants.MutabilityReadWrite, MultiValued: true, ApplicationIdentifier: AppId},
	}

	err := profileSchemaSvc.AddProfileSchemaAttributesForScope(identityAttr, constants.IdentityAttributes, SuperTenantOrg)
	err1 := profileSchemaSvc.AddProfileSchemaAttributesForScope(traits, constants.Traits, SuperTenantOrg)
	err2 := profileSchemaSvc.AddProfileSchemaAttributesForScope(appData, constants.ApplicationData, SuperTenantOrg)
	require.NoError(t, err)
	require.NoError(t, err1)
	require.NoError(t, err2)

	// Setup Unification Rules
	profileSvc := profileService.GetProfilesService()
	unificationSvc := unificationService.GetUnificationRuleService()

	emailRuleId := uuid.New().String()
	jsonData := []byte(`{
        "rule_name": "` + EmailBased + `",
		"rule_id": "` + emailRuleId + `",
		"tenant_id": "` + SuperTenantOrg + `",
        "property_name": "identity_attributes.email",
        "priority": 1,
        "is_active": true
    }`)

	var emailBasedRule model.UnificationRule
	unificationErr := json.Unmarshal(jsonData, &emailBasedRule)
	require.NoError(t, unificationErr, "Failed to unmarshal rule JSON")

	emailBasedRule.CreatedAt = time.Now().Unix()
	emailBasedRule.UpdatedAt = time.Now().Unix()

	_ = unificationSvc.AddUnificationRule(emailBasedRule, SuperTenantOrg)

	phoneRuleId := uuid.New().String()
	jsonData = []byte(`{
        "rule_name": "` + PhoneBased + `",
		"rule_id": "` + phoneRuleId + `",
		"tenant_id": "` + SuperTenantOrg + `",
        "property_name": "identity_attributes.phone_number",
        "priority": 2,
        "is_active": true
    }`)

	var phoneBasedRule model.UnificationRule
	unificationErr = json.Unmarshal(jsonData, &phoneBasedRule)
	require.NoError(t, unificationErr, "Failed to unmarshal rule JSON")

	phoneBasedRule.CreatedAt = time.Now().Unix()
	phoneBasedRule.UpdatedAt = time.Now().Unix()

	_ = unificationSvc.AddUnificationRule(phoneBasedRule, SuperTenantOrg)

	t.Run("Scenario1_TempProfiles_Email_Then_Phone_Unify", func(t *testing.T) {
		p1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["a@wso2.com"]},"traits":{"interests":["music"]}}`)
		p2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["a@wso2.com"],"phone_number":["0771234567"]},"traits":{"interests":["sports"]}}`)
		p3 := mustUnmarshalProfile(`{"identity_attributes":{"phone_number":["0771234567"]},"traits":{"interests":["art"]}}`)

		prof1, _ := profileSvc.CreateProfile(p1, SuperTenantOrg)
		prof2, _ := profileSvc.CreateProfile(p2, SuperTenantOrg)
		prof3, _ := profileSvc.CreateProfile(p3, SuperTenantOrg)

		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(prof1.ProfileId)
		merged2, _ := profileSvc.GetProfile(prof2.ProfileId)
		merged3, _ := profileSvc.GetProfile(prof3.ProfileId)

		require.Equal(t, merged1.MergedTo.ProfileId, merged2.MergedTo.ProfileId)
		require.Equal(t, EmailBased, merged1.MergedTo.Reason)
		require.Equal(t, merged2.MergedTo.ProfileId, merged3.MergedTo.ProfileId)
		require.Equal(t, PhoneBased, merged3.MergedTo.Reason)

		require.Contains(t, merged3.IdentityAttributes["email"].([]interface{}), "a@wso2.com")
		require.Contains(t, merged1.IdentityAttributes["phone_number"].([]interface{}), "0771234567")
		require.ElementsMatch(t, merged2.Traits["interests"].([]interface{}), []interface{}{"music", "sports", "art"})

		cleanProfiles(profileSvc, SuperTenantOrg)
	})

	t.Run("Scenario2_TempAndPerm_Merge_UserId_Inherit", func(t *testing.T) {
		temp := mustUnmarshalProfile(`{"identity_attributes":{"email":["b@wso2.com"],"phone_number":["0774567890"]},"traits":{"interests":["music"]}}`)
		perm := mustUnmarshalProfile(`{"user_id":"user-123","identity_attributes":{"user_id": "user-123", "email":["b@wso2.com","b2@wso2.com"]},"traits":{"interests":["sports"]}}`)

		p1, _ := profileSvc.CreateProfile(temp, SuperTenantOrg)
		p2, _ := profileSvc.CreateProfile(perm, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(p1.ProfileId)
		merged2, _ := profileSvc.GetProfile(p2.ProfileId)

		require.Equal(t, merged1.MergedTo.ProfileId, merged2.ProfileId)
		require.Equal(t, EmailBased, merged1.MergedTo.Reason)

		require.Contains(t, merged1.IdentityAttributes["email"].([]interface{}), "b2@wso2.com")
		require.ElementsMatch(t, []interface{}{"music", "sports"}, merged2.Traits["interests"].([]interface{}))
		require.Contains(t, merged2.IdentityAttributes["phone_number"].([]interface{}), "0774567890")

		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario3_TempTempThenPerm_Merge", func(t *testing.T) {
		temp1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["c@wso2.com"],"phone_number":["0771111111"]},"traits":{"interests":["music"]}}`)
		temp2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["c@wso2.com"]},"traits":{"interests":["art"]}}`)
		perm := mustUnmarshalProfile(`{"user_id":"perm-789","identity_attributes":{"user_id": "perm-789","phone_number":["0771111111"]},"traits":{"interests":["sports"]}}`)

		p1, _ := profileSvc.CreateProfile(temp1, SuperTenantOrg)
		p2, _ := profileSvc.CreateProfile(temp2, SuperTenantOrg)
		time.Sleep(2 * time.Second)
		p3, _ := profileSvc.CreateProfile(perm, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(p1.ProfileId)
		merged2, _ := profileSvc.GetProfile(p2.ProfileId)
		merged3, _ := profileSvc.GetProfile(p3.ProfileId)

		require.Equal(t, merged1.MergedTo.ProfileId, merged2.MergedTo.ProfileId)
		require.Equal(t, EmailBased, merged1.MergedTo.Reason)

		require.Equal(t, merged2.MergedTo.ProfileId, merged3.ProfileId)
		// todo: Verify if this is correct. Should it be PHONE_BASED since we merge perm to merged profiles of Temp using phone number?
		require.Equal(t, EmailBased, merged2.MergedTo.Reason)

		require.Equal(t, "perm-789", merged1.IdentityAttributes["user_id"])
		require.ElementsMatch(t, []interface{}{"music", "art", "sports"}, merged3.Traits["interests"].([]interface{}))
		require.Contains(t, merged2.IdentityAttributes["phone_number"].([]interface{}), "0771111111")
		require.Contains(t, merged3.IdentityAttributes["email"].([]interface{}), "c@wso2.com")

		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario4_TempPermThenAnotherTemp", func(t *testing.T) {
		perm := mustUnmarshalProfile(`{"user_id":"perm-001","identity_attributes":{"user_id":"perm-001","email":["d@wso2.com"]},"traits":{"interests":["music"]}}`)
		temp := mustUnmarshalProfile(`{"identity_attributes":{"email":["d@wso2.com"],"phone_number":["0775554444"]},"traits":{"interests":["art"]}}`)
		temp2 := mustUnmarshalProfile(`{"identity_attributes":{"phone_number":["0775554444"]},"traits":{"interests":["sports"]}}`)

		p1, _ := profileSvc.CreateProfile(perm, SuperTenantOrg)
		p2, _ := profileSvc.CreateProfile(temp, SuperTenantOrg)
		p3, _ := profileSvc.CreateProfile(temp2, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(p1.ProfileId)
		merged2, _ := profileSvc.GetProfile(p2.ProfileId)
		merged3, _ := profileSvc.GetProfile(p3.ProfileId)

		require.Equal(t, merged1.ProfileId, merged2.MergedTo.ProfileId)
		require.Equal(t, EmailBased, merged2.MergedTo.Reason)
		require.Equal(t, merged1.ProfileId, merged3.MergedTo.ProfileId)
		require.Equal(t, PhoneBased, merged3.MergedTo.Reason)

		require.ElementsMatch(t, []interface{}{"music", "art", "sports"}, merged3.Traits["interests"].([]interface{}))
		require.Contains(t, merged1.IdentityAttributes["phone_number"].([]interface{}), "0775554444")
		require.Contains(t, merged3.IdentityAttributes["email"].([]interface{}), "d@wso2.com")

		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario5_TwoPermanent_SameEmail_NoMerge", func(t *testing.T) {
		perm1 := mustUnmarshalProfile(`{"user_id":"perm-A","identity_attributes":{"email":["e@wso2.com"]}}`)
		perm2 := mustUnmarshalProfile(`{"user_id":"perm-B","identity_attributes":{"email":["e@wso2.com"]}}`)

		p1, _ := profileSvc.CreateProfile(perm1, SuperTenantOrg)
		p2, _ := profileSvc.CreateProfile(perm2, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		require.Empty(t, p1.MergedTo)
		require.Empty(t, p2.MergedTo)

		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario6_RulePriority_HigherPriorityWins", func(t *testing.T) {
		temp1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["f@wso2.com"],"phone_number":["0777777777"]}}`)
		temp2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["f@wso2.com"],"phone_number":["0777777777"]}}`)

		p1, _ := profileSvc.CreateProfile(temp1, SuperTenantOrg)
		p2, _ := profileSvc.CreateProfile(temp2, SuperTenantOrg)

		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(p1.ProfileId)
		merged2, _ := profileSvc.GetProfile(p2.ProfileId)

		require.Equal(t, merged1.MergedTo.ProfileId, merged2.MergedTo.ProfileId)
		require.Equal(t, EmailBased, merged2.MergedTo.Reason)

		cleanProfiles(profileSvc, SuperTenantOrg)
	})

	t.Run("Scenario7_InactiveRule_ShouldPreventUnification", func(t *testing.T) {

		patchData := disableUnificationRule()
		err = unificationSvc.PatchUnificationRule(emailRuleId, SuperTenantOrg, patchData)
		rule, _ := unificationSvc.GetUnificationRule(emailRuleId)
		require.Equal(t, false, rule.IsActive)

		p1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["g@wso2.com"]}}`)
		p2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["g@wso2.com"]}}`)

		prof1, _ := profileSvc.CreateProfile(p1, SuperTenantOrg)
		prof2, _ := profileSvc.CreateProfile(p2, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		mergedProfile1, _ := profileSvc.GetProfile(prof1.ProfileId)
		mergedProfile2, _ := profileSvc.GetProfile(prof2.ProfileId)

		require.Empty(t, mergedProfile1.MergedTo)
		require.Empty(t, mergedProfile2.MergedTo)

		patchData = enableUnificationRule()
		err = unificationSvc.PatchUnificationRule(emailRuleId, SuperTenantOrg, patchData)
		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario8_RuleChange_ShouldNotSplitExisting", func(t *testing.T) {
		p1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["j@wso2.com"]}}`)
		p2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["j@wso2.com"]}}`)

		prof1, _ := profileSvc.CreateProfile(p1, SuperTenantOrg)
		prof2, _ := profileSvc.CreateProfile(p2, SuperTenantOrg)
		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(prof1.ProfileId)
		merged2, _ := profileSvc.GetProfile(prof2.ProfileId)
		require.Equal(t, merged1.MergedTo.ProfileId, merged2.MergedTo.ProfileId)

		_ = unificationSvc.DeleteUnificationRule(emailRuleId)

		after1, _ := profileSvc.GetProfile(prof1.ProfileId)
		after2, _ := profileSvc.GetProfile(prof2.ProfileId)

		merged1, _ = profileSvc.GetProfile(after1.ProfileId)
		merged2, _ = profileSvc.GetProfile(after2.ProfileId)
		require.Equal(t, merged1.MergedTo.ProfileId, merged2.MergedTo.ProfileId)
		cleanProfiles(profileSvc, SuperTenantOrg)

	})

	t.Run("Scenario9_CrossTenantProfiles_ShouldNotUnify", func(t *testing.T) {

		OtherTenant := fmt.Sprintf("other.org-%d", time.Now().UnixNano())
		identityAttr := []schemaModel.ProfileSchemaAttribute{
			{OrgId: OtherTenant, AttributeId: uuid.New().String(), AttributeName: "identity_attributes.email", ValueType: constants.StringDataType, MergeStrategy: "combine",
				Mutability: constants.MutabilityReadWrite, MultiValued: true},
		}
		err = profileSchemaSvc.AddProfileSchemaAttributesForScope(identityAttr, constants.IdentityAttributes, OtherTenant)

		emailRuleId := uuid.New().String()
		jsonData := []byte(`{
	   "rule_name": "` + EmailBased + `",
		"rule_id": "` + emailRuleId + `",
		"tenant_id": "` + OtherTenant + `",
	   "property_name": "identity_attributes.email",
	   "priority": 1,
	   "is_active": true
		}`)

		var emailBasedRule model.UnificationRule
		unificationErr := json.Unmarshal(jsonData, &emailBasedRule)
		require.NoError(t, unificationErr, "Failed to unmarshal rule JSON")

		emailBasedRule.CreatedAt = time.Now().Unix()
		emailBasedRule.UpdatedAt = time.Now().Unix()

		_ = unificationSvc.AddUnificationRule(emailBasedRule, SuperTenantOrg)

		p1 := mustUnmarshalProfile(`{"identity_attributes":{"email":["k@wso2.com"]}}`)
		p2 := mustUnmarshalProfile(`{"identity_attributes":{"email":["k@wso2.com"]}}`)

		prof1, _ := profileSvc.CreateProfile(p1, SuperTenantOrg)
		prof2, _ := profileSvc.CreateProfile(p2, OtherTenant)
		time.Sleep(5 * time.Second)

		merged1, _ := profileSvc.GetProfile(prof1.ProfileId)
		merged2, _ := profileSvc.GetProfile(prof2.ProfileId)

		require.Empty(t, merged1.MergedTo)
		require.Empty(t, merged2.MergedTo)
		cleanProfiles(profileSvc, SuperTenantOrg)
		cleanProfiles(profileSvc, OtherTenant)
	})

	t.Run("Scenario10_CrossApplicationAttributeMatch", func(t *testing.T) {
		// TEST SCENARIO:
		// Profile 1: Has app1 (theme="light") AND app2 (theme="dark")
		// Profile 2: Has app2 (theme="light")
		// Unification Rule: Based on application_data.theme attribute

		App1Id := "app-theme-001"
		App2Id := "app-theme-002"

		themeSchema := []schemaModel.ProfileSchemaAttribute{
			{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(),
				AttributeName: "application_data.theme",
				ValueType:     constants.StringDataType, MergeStrategy: "combine",
				Mutability: constants.MutabilityReadWrite, MultiValued: false,
				ApplicationIdentifier: App1Id},
			{OrgId: SuperTenantOrg, AttributeId: uuid.New().String(),
				AttributeName: "application_data.theme",
				ValueType:     constants.StringDataType, MergeStrategy: "combine",
				Mutability: constants.MutabilityReadWrite, MultiValued: false,
				ApplicationIdentifier: App2Id},
		}
		err := profileSchemaSvc.AddProfileSchemaAttributesForScope(themeSchema, constants.ApplicationData, SuperTenantOrg)
		require.NoError(t, err, "Failed to add theme schema")

		themeRuleId := uuid.New().String()
		themeRuleData := []byte(`{
			"rule_name": "theme_based",
			"rule_id": "` + themeRuleId + `",
			"tenant_id": "` + SuperTenantOrg + `",
			"property_name": "application_data.theme",
			"priority": 3,
			"is_active": true
		}`)
		var themeRule model.UnificationRule
		_ = json.Unmarshal(themeRuleData, &themeRule)
		themeRule.CreatedAt = time.Now().Unix()
		themeRule.UpdatedAt = time.Now().Unix()
		err = unificationSvc.AddUnificationRule(themeRule, SuperTenantOrg)
		require.NoError(t, err, "Failed to add theme-based unification rule")

		p1 := mustUnmarshalProfile(`{
			"identity_attributes":{"email":["theme-test-p1@wso2.com"]},
			"traits":{"interests":["design"]},
			"application_data":{
				"` + App1Id + `":{"theme":"light"},
				"` + App2Id + `":{"theme":"dark"}
			}
		}`)

		p2 := mustUnmarshalProfile(`{
			"identity_attributes":{"email":["theme-test-p2@wso2.com"]},
			"traits":{"interests":["coding"]},
			"application_data":{
				"` + App2Id + `":{"theme":"light"}
			}
		}`)

		prof1, err := profileSvc.CreateProfile(p1, SuperTenantOrg)
		require.NoError(t, err, "Failed to create profile 1")

		prof2, err := profileSvc.CreateProfile(p2, SuperTenantOrg)
		require.NoError(t, err, "Failed to create profile 2")

		time.Sleep(2 * time.Second)

		merged1, _ := profileSvc.GetProfile(prof1.ProfileId)
		merged2, _ := profileSvc.GetProfile(prof2.ProfileId)

		didMerge := merged1.MergedTo.ProfileId != "" || merged2.MergedTo.ProfileId != ""

		require.False(t, didMerge, "Profiles should NOT merge based on cross-application attribute match")

		_ = unificationSvc.DeleteUnificationRule(themeRuleId)
		cleanProfiles(profileSvc, SuperTenantOrg)
	})

	// Cleanup
	t.Cleanup(func() {
		rules, _ := unificationSvc.GetUnificationRules(SuperTenantOrg)
		for _, r := range rules {
			_ = unificationSvc.DeleteUnificationRule(r.RuleId)
		}
		cleanProfiles(profileSvc, SuperTenantOrg)
		_ = profileSchemaSvc.DeleteProfileSchema(SuperTenantOrg)
		_ = profileSchemaSvc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
	})
}

func cleanProfiles(profileSvc profileService.ProfilesServiceInterface, org string) {

	profiles, _ := profileSvc.GetAllProfiles(org)
	for _, p := range profiles {
		_ = profileSvc.DeleteProfile(p.ProfileId)
	}
}

func disableUnificationRule() map[string]interface{} {
	jsonData := []byte(`{
        "is_active": false
    	}`)
	var patchData map[string]interface{}
	_ = json.Unmarshal(jsonData, &patchData)
	return patchData
}

func enableUnificationRule() map[string]interface{} {
	jsonData := []byte(`{
        "is_active": true
    	}`)
	var EnableData map[string]interface{}
	_ = json.Unmarshal(jsonData, &EnableData)
	return EnableData
}
