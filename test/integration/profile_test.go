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
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	unificationService "github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

func Test_Profile(t *testing.T) {

	SuperTenantOrg := fmt.Sprintf("carbon.super-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	profileSchemaSvc := schemaService.GetProfileSchemaService()
	unificationSvc := unificationService.GetUnificationRuleService()

	t.Run("PreRequisite_AddProfileSchemaAttributes", func(t *testing.T) {

		identityAttributes := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "identity_attributes.email",
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				MultiValued:   true,
			},
		}

		traits := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         SuperTenantOrg,
				AttributeId:   uuid.New().String(),
				AttributeName: "traits.interests",
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				MultiValued:   true,
			},
		}

		appData := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:                 SuperTenantOrg,
				AttributeId:           uuid.New().String(),
				AttributeName:         "application_data.device_id",
				ValueType:             constants.StringDataType,
				MergeStrategy:         "combine",
				Mutability:            constants.MutabilityReadWrite,
				ApplicationIdentifier: "app1",
				MultiValued:           true,
			},
		}

		err := profileSchemaSvc.AddProfileSchemaAttributesForScope(identityAttributes, constants.IdentityAttributes, SuperTenantOrg)
		require.NoError(t, err, "Failed to add identity schema attributes")

		err = profileSchemaSvc.AddProfileSchemaAttributesForScope(traits, constants.Traits, SuperTenantOrg)
		require.NoError(t, err, "Failed to add traits schema attributes")

		err = profileSchemaSvc.AddProfileSchemaAttributesForScope(appData, constants.ApplicationData, SuperTenantOrg)
		require.NoError(t, err, "Failed to add app data schema attributes")
	})

	email := "test@wso2.com"
	var profileRequest profileModel.ProfileRequest
	jsonData := []byte(`{
		  "user_id": "user-001",
		  "identity_attributes": { "email": ["test@wso2.com"] },
		  "traits": { "interests": ["reading"] },
		  "application_data": { "app1": { "device_id": ["device1"] } }
		}`)
	_ = json.Unmarshal(jsonData, &profileRequest)

	t.Run("Create_Profile_Success", func(t *testing.T) {
		profile, err := profileSvc.CreateProfile(profileRequest, SuperTenantOrg)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Equal(t, email, profile.IdentityAttributes["email"].([]interface{})[0])
		require.Contains(t, profile.Traits["interests"], "reading")
	})

	t.Run("Get_Profile_Success", func(t *testing.T) {
		profiles, err := profileSvc.GetAllProfiles(SuperTenantOrg)
		require.NoError(t, err)
		require.NotEmpty(t, profiles)
		profile, err := profileSvc.GetProfile(profiles[0].ProfileId)
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Contains(t, profile.IdentityAttributes["email"], email)
	})

	t.Run("Update_Profile_Success", func(t *testing.T) {
		_, err := profileSvc.CreateProfile(profileRequest, SuperTenantOrg)
		require.NoError(t, err)

		var updatedRequest profileModel.ProfileRequest
		jsonData := []byte(`{
		"identity_attributes": {
			"email": ["updated@wso2.com"]
		},
		"traits": {
			"interests": ["reading", "travel"]
		}
	}`)
		_ = json.Unmarshal(jsonData, &updatedRequest)

		profiles, _ := profileSvc.GetAllProfiles(SuperTenantOrg)
		p := profiles[0]

		updated, err := profileSvc.UpdateProfile(p.ProfileId, SuperTenantOrg, updatedRequest)
		require.NoError(t, err)
		require.Contains(t, updated.Traits["interests"], "travel")
		require.Equal(t, "updated@wso2.com", updated.IdentityAttributes["email"].([]interface{})[0])
	})

	t.Run("Delete_Profile_Success", func(t *testing.T) {
		profiles, _ := profileSvc.GetAllProfiles(SuperTenantOrg)
		p := profiles[0]

		err := profileSvc.DeleteProfile(p.ProfileId)
		require.NoError(t, err)

		_, err = profileSvc.GetProfile(p.ProfileId)
		require.Error(t, err)
	})

	t.Cleanup(func() {
		rules, _ := unificationSvc.GetUnificationRules(SuperTenantOrg)
		for _, r := range rules {
			_ = unificationSvc.DeleteUnificationRule(r.RuleId)
		}
		profiles, _ := profileSvc.GetAllProfiles(SuperTenantOrg)
		for _, p := range profiles {
			_ = profileSvc.DeleteProfile(p.ProfileId)
		}
		_ = profileSchemaSvc.DeleteProfileSchema(SuperTenantOrg)
		_ = profileSchemaSvc.DeleteProfileSchemaAttributesByScope(SuperTenantOrg, constants.IdentityAttributes)
	})
}
