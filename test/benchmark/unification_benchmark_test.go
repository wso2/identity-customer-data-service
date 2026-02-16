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
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

// setupUnificationTestSchema sets up schema for unification benchmarks
func setupUnificationTestSchema(b *testing.B, orgHandle string) {
	b.Helper()

	profileSchemaSvc := schemaService.GetProfileSchemaService()
	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	b.Cleanup(restore)

	schemaAttributes := []profileSchema.ProfileSchemaAttribute{
		{
			OrgId:         orgHandle,
			AttributeName: "identity_attributes.email",
			AttributeId:   uuid.New().String(),
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		},
		{
			OrgId:         orgHandle,
			AttributeName: "identity_attributes.phone",
			AttributeId:   uuid.New().String(),
			ValueType:     constants.StringDataType,
			MergeStrategy: "combine",
			Mutability:    constants.MutabilityReadWrite,
		},
	}

	_ = profileSchemaSvc.AddProfileSchemaAttributesForScope(schemaAttributes, constants.IdentityAttributes, orgHandle)
}

// Benchmark_AddUnificationRule benchmarks adding unification rules
func Benchmark_AddUnificationRule(b *testing.B) {
	orgHandle := fmt.Sprintf("unification-org-%d", time.Now().UnixNano())
	unificationSvc := service.GetUnificationRuleService()

	setupUnificationTestSchema(b, orgHandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rule := model.UnificationRule{
			RuleName:     fmt.Sprintf("Rule-%d", i),
			RuleId:       uuid.New().String(),
			OrgHandle:    orgHandle,
			PropertyName: "identity_attributes.email",
			Priority:     i + 1,
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		b.StartTimer()

		err := unificationSvc.AddUnificationRule(rule, orgHandle)
		if err != nil {
			b.Fatalf("Failed to add unification rule: %v", err)
		}
	}
}

// Benchmark_GetUnificationRules benchmarks retrieving unification rules
func Benchmark_GetUnificationRules(b *testing.B) {
	orgHandle := fmt.Sprintf("unification-org-%d", time.Now().UnixNano())
	unificationSvc := service.GetUnificationRuleService()

	setupUnificationTestSchema(b, orgHandle)

	// Create some rules for retrieval
	for i := 0; i < 5; i++ {
		rule := model.UnificationRule{
			RuleName:     fmt.Sprintf("Rule-%d", i),
			RuleId:       uuid.New().String(),
			OrgHandle:    orgHandle,
			PropertyName: "identity_attributes.email",
			Priority:     i + 1,
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		_ = unificationSvc.AddUnificationRule(rule, orgHandle)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := unificationSvc.GetUnificationRules(orgHandle)
		if err != nil {
			b.Fatalf("Failed to get unification rules: %v", err)
		}
	}
}

// Benchmark_UpdateUnificationRule benchmarks updating unification rules
func Benchmark_UpdateUnificationRule(b *testing.B) {
	orgHandle := fmt.Sprintf("unification-org-%d", time.Now().UnixNano())
	unificationSvc := service.GetUnificationRuleService()

	setupUnificationTestSchema(b, orgHandle)

	// Create a rule for updating
	ruleId := uuid.New().String()
	rule := model.UnificationRule{
		RuleName:     "Update Rule",
		RuleId:       ruleId,
		OrgHandle:    orgHandle,
		PropertyName: "identity_attributes.email",
		Priority:     1,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = unificationSvc.AddUnificationRule(rule, orgHandle)

	updatedRule := model.UnificationRule{
		RuleName:     "Updated Rule",
		RuleId:       ruleId,
		OrgHandle:    orgHandle,
		PropertyName: "identity_attributes.phone",
		Priority:     2,
		IsActive:     false,
		CreatedAt:    rule.CreatedAt,
		UpdatedAt:    time.Now().UTC(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := unificationSvc.UpdateUnificationRule(updatedRule, orgHandle)
		if err != nil {
			b.Fatalf("Failed to update unification rule: %v", err)
		}
	}
}

// Benchmark_DeleteUnificationRule benchmarks deleting unification rules
func Benchmark_DeleteUnificationRule(b *testing.B) {
	orgHandle := fmt.Sprintf("unification-org-%d", time.Now().UnixNano())
	unificationSvc := service.GetUnificationRuleService()

	setupUnificationTestSchema(b, orgHandle)

	// Create rules for deletion
	ruleIds := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		ruleId := uuid.New().String()
		rule := model.UnificationRule{
			RuleName:     fmt.Sprintf("Delete Rule-%d", i),
			RuleId:       ruleId,
			OrgHandle:    orgHandle,
			PropertyName: "identity_attributes.email",
			Priority:     i + 1,
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		_ = unificationSvc.AddUnificationRule(rule, orgHandle)
		ruleIds[i] = ruleId
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := unificationSvc.DeleteUnificationRule(ruleIds[i], orgHandle)
		if err != nil {
			b.Fatalf("Failed to delete unification rule: %v", err)
		}
	}
}

// Benchmark_GetUnificationRuleById benchmarks getting a specific unification rule
func Benchmark_GetUnificationRuleById(b *testing.B) {
	orgHandle := fmt.Sprintf("unification-org-%d", time.Now().UnixNano())
	unificationSvc := service.GetUnificationRuleService()

	setupUnificationTestSchema(b, orgHandle)

	// Create a rule for retrieval
	ruleId := uuid.New().String()
	rule := model.UnificationRule{
		RuleName:     "Get Rule",
		RuleId:       ruleId,
		OrgHandle:    orgHandle,
		PropertyName: "identity_attributes.email",
		Priority:     1,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = unificationSvc.AddUnificationRule(rule, orgHandle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := unificationSvc.GetUnificationRule(ruleId, orgHandle)
		if err != nil {
			b.Fatalf("Failed to get unification rule by id: %v", err)
		}
	}
}
