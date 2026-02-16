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
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// Benchmark_GetProfileSchema benchmarks fetching profile schema
func Benchmark_GetProfileSchema(b *testing.B) {
	orgHandle := fmt.Sprintf("schema-org-%d", time.Now().UnixNano())
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	// Setup schema attributes
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
	}

	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(identityAttributes, constants.IdentityAttributes, orgHandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profileSchemaSvc.GetProfileSchema(orgHandle)
		if err != nil {
			b.Fatalf("Failed to get profile schema: %v", err)
		}
	}
}

// Benchmark_AddSchemaAttribute benchmarks adding schema attributes
func Benchmark_AddSchemaAttribute(b *testing.B) {
	orgHandle := fmt.Sprintf("schema-org-%d", time.Now().UnixNano())
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		attributes := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         orgHandle,
				AttributeId:   uuid.New().String(),
				AttributeName: fmt.Sprintf("identity_attributes.field_%d", i),
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				MultiValued:   false,
			},
		}
		b.StartTimer()

		err := profileSchemaSvc.AddProfileSchemaAttributesForScope(attributes, constants.IdentityAttributes, orgHandle)
		if err != nil {
			b.Fatalf("Failed to add schema attribute: %v", err)
		}
	}
}

// Benchmark_GetSchemaAttributesByScope benchmarks fetching schema attributes by scope
func Benchmark_GetSchemaAttributesByScope(b *testing.B) {
	orgHandle := fmt.Sprintf("schema-org-%d", time.Now().UnixNano())
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	// Setup schema attributes
	attributes := []profileSchema.ProfileSchemaAttribute{
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

	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(attributes, constants.IdentityAttributes, orgHandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profileSchemaSvc.GetSchemaAttributesByScope(orgHandle, constants.IdentityAttributes)
		if err != nil {
			b.Fatalf("Failed to get schema attributes by scope: %v", err)
		}
	}
}

// Benchmark_UpdateSchemaAttribute benchmarks updating schema attributes
func Benchmark_UpdateSchemaAttribute(b *testing.B) {
	orgHandle := fmt.Sprintf("schema-org-%d", time.Now().UnixNano())
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	// Setup initial schema attribute
	attributeId := uuid.New().String()
	attributes := []profileSchema.ProfileSchemaAttribute{
		{
			OrgId:         orgHandle,
			AttributeId:   attributeId,
			AttributeName: "identity_attributes.email",
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
			MultiValued:   true,
		},
	}

	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(attributes, constants.IdentityAttributes, orgHandle)

	// Update request
	updateAttr := profileSchema.ProfileSchemaAttribute{
		OrgId:         orgHandle,
		AttributeId:   attributeId,
		AttributeName: "identity_attributes.email",
		ValueType:     constants.StringDataType,
		MergeStrategy: "override",
		Mutability:    constants.MutabilityReadWrite,
		MultiValued:   true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := profileSchemaSvc.UpdateSchemaAttribute(updateAttr, orgHandle)
		if err != nil {
			b.Fatalf("Failed to update schema attribute: %v", err)
		}
	}
}

// Benchmark_DeleteSchemaAttribute benchmarks deleting schema attributes
func Benchmark_DeleteSchemaAttribute(b *testing.B) {
	orgHandle := fmt.Sprintf("schema-org-%d", time.Now().UnixNano())
	profileSchemaSvc := schemaService.GetProfileSchemaService()

	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	defer restore()

	// Create attributes for deletion
	attributeIds := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		attributeId := uuid.New().String()
		attributes := []profileSchema.ProfileSchemaAttribute{
			{
				OrgId:         orgHandle,
				AttributeId:   attributeId,
				AttributeName: fmt.Sprintf("identity_attributes.field_%d", i),
				ValueType:     constants.StringDataType,
				MergeStrategy: "combine",
				Mutability:    constants.MutabilityReadWrite,
				MultiValued:   false,
			},
		}
		_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(attributes, constants.IdentityAttributes, orgHandle)
		attributeIds[i] = attributeId
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := profileSchemaSvc.DeleteSchemaAttribute(attributeIds[i], orgHandle)
		if err != nil {
			b.Fatalf("Failed to delete schema attribute: %v", err)
		}
	}
}
