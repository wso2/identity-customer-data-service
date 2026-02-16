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

package benchmark

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// setupTestSchema sets up the profile schema for benchmarking
func setupTestSchema(b *testing.B, orgHandle string) {
	b.Helper()

	profileSchemaSvc := schemaService.GetProfileSchemaService()
	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	b.Cleanup(restore)

	identityAttributes := []profileSchema.ProfileSchemaAttribute{
		{
			OrgId:         orgHandle,
			AttributeId:   uuid.New().String(),
			AttributeName: "identity_attributes.email",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   true,
		},
		{
			OrgId:         orgHandle,
			AttributeId:   uuid.New().String(),
			AttributeName: "identity_attributes.phone",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   true,
		},
	}

	traits := []profileSchema.ProfileSchemaAttribute{
		{
			OrgId:         orgHandle,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.interests",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   true,
		},
		{
			OrgId:         orgHandle,
			AttributeId:   uuid.New().String(),
			AttributeName: "traits.preferences",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   true,
		},
	}

	appData := []profileSchema.ProfileSchemaAttribute{
		{
			OrgId:                 orgHandle,
			AttributeId:           uuid.New().String(),
			AttributeName:         "application_data.device_id",
			ValueType:             constants.StringDataType,
			MergeStrategy:         "combine",
			Mutability:            constants.MutabilityReadWrite,
			ApplicationIdentifier: "app1",
			MultiValued:           true,
		},
	}

	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(identityAttributes, constants.IdentityAttributes, orgHandle)
	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(traits, constants.Traits, orgHandle)
	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(appData, constants.ApplicationData, orgHandle)
}

// createTestProfile creates a profile for benchmarking
func createTestProfile(profileSvc profileService.ProfilesServiceInterface, orgHandle string, userId string) (*profileModel.ProfileResponse, error) {
	var profileRequest profileModel.ProfileRequest
	jsonData := []byte(fmt.Sprintf(`{
		"user_id": "%s",
		"identity_attributes": { "email": ["test%s@wso2.com"], "phone": ["+1234567890"] },
		"traits": { "interests": ["reading", "coding"], "preferences": ["dark_mode"] },
		"application_data": { "app1": { "device_id": ["device1"] } }
	}`, userId, userId))
	_ = json.Unmarshal(jsonData, &profileRequest)

	return profileSvc.CreateProfile(profileRequest, orgHandle)
}

// Benchmark_CreateProfile benchmarks profile creation
func Benchmark_CreateProfile(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userId := fmt.Sprintf("user-%d", i)
		_, err := createTestProfile(profileSvc, orgHandle, userId)
		if err != nil {
			b.Fatalf("Failed to create profile: %v", err)
		}
	}
}

// Benchmark_GetProfile benchmarks profile retrieval
func Benchmark_GetProfile(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create a profile for retrieval
	profile, err := createTestProfile(profileSvc, orgHandle, "benchmark-user")
	if err != nil {
		b.Fatalf("Failed to create profile for benchmark: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profileSvc.GetProfile(profile.ProfileId)
		if err != nil {
			b.Fatalf("Failed to get profile: %v", err)
		}
	}
}

// Benchmark_UpdateProfile benchmarks profile updates
func Benchmark_UpdateProfile(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create a profile for updating
	profile, err := createTestProfile(profileSvc, orgHandle, "update-user")
	if err != nil {
		b.Fatalf("Failed to create profile for benchmark: %v", err)
	}

	var updateRequest profileModel.ProfileRequest
	jsonData := []byte(`{
		"identity_attributes": { "email": ["updated@wso2.com"] },
		"traits": { "interests": ["reading", "travel", "photography"] }
	}`)
	_ = json.Unmarshal(jsonData, &updateRequest)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profileSvc.UpdateProfile(profile.ProfileId, orgHandle, updateRequest)
		if err != nil {
			b.Fatalf("Failed to update profile: %v", err)
		}
	}
}

// Benchmark_PatchProfile benchmarks profile patch operations
func Benchmark_PatchProfile(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create a profile for patching
	profile, err := createTestProfile(profileSvc, orgHandle, "patch-user")
	if err != nil {
		b.Fatalf("Failed to create profile for benchmark: %v", err)
	}

	patchData := map[string]interface{}{
		"traits": map[string]interface{}{
			"preferences": []string{"light_mode"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profileSvc.PatchProfile(profile.ProfileId, orgHandle, patchData)
		if err != nil {
			b.Fatalf("Failed to patch profile: %v", err)
		}
	}
}

// Benchmark_GetAllProfiles benchmarks listing profiles
func Benchmark_GetAllProfiles(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create multiple profiles for listing
	for i := 0; i < 10; i++ {
		userId := fmt.Sprintf("list-user-%d", i)
		_, _ = createTestProfile(profileSvc, orgHandle, userId)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := profileSvc.GetAllProfilesCursor(orgHandle, 10, nil)
		if err != nil {
			b.Fatalf("Failed to get all profiles: %v", err)
		}
	}
}

// Benchmark_GetAllProfilesWithFilter benchmarks listing profiles with filters
func Benchmark_GetAllProfilesWithFilter(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create multiple profiles for filtered listing
	for i := 0; i < 10; i++ {
		userId := fmt.Sprintf("filter-user-%d", i)
		_, _ = createTestProfile(profileSvc, orgHandle, userId)
	}

	filters := []string{"identity_attributes.email co 'test'"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := profileSvc.GetAllProfilesWithFilterCursor(orgHandle, filters, 10, nil)
		if err != nil {
			b.Fatalf("Failed to get filtered profiles: %v", err)
		}
	}
}

// Benchmark_DeleteProfile benchmarks profile deletion
func Benchmark_DeleteProfile(b *testing.B) {
	orgHandle := fmt.Sprintf("benchmark-org-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()

	setupTestSchema(b, orgHandle)

	// Create profiles for deletion
	profileIds := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		userId := fmt.Sprintf("delete-user-%d", i)
		profile, err := createTestProfile(profileSvc, orgHandle, userId)
		if err != nil {
			b.Fatalf("Failed to create profile for benchmark: %v", err)
		}
		profileIds[i] = profile.ProfileId
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := profileSvc.DeleteProfile(profileIds[i])
		if err != nil {
			b.Fatalf("Failed to delete profile: %v", err)
		}
	}
}
