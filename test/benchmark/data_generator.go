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
	"math/rand"

	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileSchema "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// DataTier defines dataset scale for benchmarks.
type DataTier struct {
	Name                   string
	ProfileCount           int
	IdentityAttributeCount int
	TraitCount             int
	AppDataEntriesPerProfile int
	UnificationRuleCount   int
}

// Predefined tiers matching the specification.
var (
	TierSmall = DataTier{
		Name:                     "small",
		ProfileCount:             10000,
		IdentityAttributeCount:   100,
		TraitCount:               100,
		AppDataEntriesPerProfile: 100,
		UnificationRuleCount:     3,
	}
	TierMedium = DataTier{
		Name:                     "medium",
		ProfileCount:             100000,
		IdentityAttributeCount:   150,
		TraitCount:               250,
		AppDataEntriesPerProfile: 250,
		UnificationRuleCount:     5,
	}
	TierLarge = DataTier{
		Name:                     "large",
		ProfileCount:             1000000,
		IdentityAttributeCount:   200,
		TraitCount:               500,
		AppDataEntriesPerProfile: 500,
		UnificationRuleCount:     10,
	}
)

// OverlapConfig controls how much data overlaps between profiles to trigger unification.
type OverlapConfig struct {
	EmailOverlapPct    float64 // fraction of profiles sharing email
	DeviceOverlapPct   float64 // fraction of profiles sharing device_id
	CustomerOverlapPct float64 // fraction of profiles sharing customer_id
}

// DefaultOverlap returns the standard overlap configuration (10-20%).
func DefaultOverlap() OverlapConfig {
	return OverlapConfig{
		EmailOverlapPct:    0.15,
		DeviceOverlapPct:   0.10,
		CustomerOverlapPct: 0.10,
	}
}

// SchemaConfig holds the schema attributes to register for benchmarks.
type SchemaConfig struct {
	IdentityAttributes []profileSchema.ProfileSchemaAttribute
	Traits             []profileSchema.ProfileSchemaAttribute
	AppData            []profileSchema.ProfileSchemaAttribute
}

// GenerateSchemaConfig builds a rich schema matching the specification.
func GenerateSchemaConfig(orgHandle string) SchemaConfig {
	return SchemaConfig{
		IdentityAttributes: []profileSchema.ProfileSchemaAttribute{
			newSchemaAttr(orgHandle, "identity_attributes.email", constants.StringDataType, true, ""),
			newSchemaAttr(orgHandle, "identity_attributes.phone", constants.StringDataType, true, ""),
			newSchemaAttr(orgHandle, "identity_attributes.username", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "identity_attributes.customer_id", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "identity_attributes.device_id", constants.StringDataType, true, ""),
		},
		Traits: []profileSchema.ProfileSchemaAttribute{
			newSchemaAttr(orgHandle, "traits.loyalty_tier", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "traits.engagement_score", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "traits.preferred_category", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "traits.spending_score", constants.StringDataType, false, ""),
			newSchemaAttr(orgHandle, "traits.campaign_affinity", constants.StringDataType, false, ""),
		},
		AppData: []profileSchema.ProfileSchemaAttribute{
			newSchemaAttr(orgHandle, "application_data.device_type", constants.StringDataType, false, "app1"),
			newSchemaAttr(orgHandle, "application_data.session_id", constants.StringDataType, false, "app1"),
			newSchemaAttr(orgHandle, "application_data.login_method", constants.StringDataType, false, "app1"),
			newSchemaAttr(orgHandle, "application_data.app_preference", constants.StringDataType, false, "app1"),
			newSchemaAttr(orgHandle, "application_data.last_purchase", constants.StringDataType, false, "app1"),
		},
	}
}

func newSchemaAttr(orgHandle, name, valueType string, multiValued bool, appIdentifier string) profileSchema.ProfileSchemaAttribute {
	return profileSchema.ProfileSchemaAttribute{
		OrgId:                 orgHandle,
		AttributeId:           uuid.New().String(),
		AttributeName:         name,
		ValueType:             valueType,
		MergeStrategy:         "combine",
		Mutability:            constants.MutabilityReadWrite,
		MultiValued:           multiValued,
		ApplicationIdentifier: appIdentifier,
	}
}

// Profile data pools for diverse generation.
var (
	loyaltyTiers       = []string{"bronze", "silver", "gold", "platinum", "diamond"}
	engagementScores   = []string{"low", "medium", "high", "very_high"}
	categories         = []string{"electronics", "fashion", "home", "sports", "books", "food", "travel", "auto"}
	spendingScores     = []string{"frugal", "moderate", "generous", "premium"}
	campaignAffinities = []string{"email_promo", "social_ads", "push_notify", "sms_deals", "loyalty_program"}
	deviceTypes        = []string{"mobile_ios", "mobile_android", "desktop_chrome", "desktop_safari", "tablet"}
	loginMethods       = []string{"password", "google_sso", "facebook_sso", "biometric", "magic_link"}
	appPreferences     = []string{"dark_mode", "light_mode", "compact_view", "notifications_on", "auto_play"}
)

// GenerateProfileRequest creates a profile request with realistic data.
// index controls deterministic yet varied data; overlap specifies shared attribute ratios.
func GenerateProfileRequest(index int, totalProfiles int, overlap OverlapConfig) profileModel.ProfileRequest {
	rng := rand.New(rand.NewSource(int64(index)))

	// Identity attributes with controlled overlap
	email := generateEmail(index, totalProfiles, overlap.EmailOverlapPct, rng)
	phone := fmt.Sprintf("+1%010d", rng.Intn(10000000000))
	username := fmt.Sprintf("user_%d", index)
	customerID := generateCustomerID(index, totalProfiles, overlap.CustomerOverlapPct, rng)
	deviceID := generateDeviceID(index, totalProfiles, overlap.DeviceOverlapPct, rng)

	identity := map[string]interface{}{
		"email":       []interface{}{email},
		"phone":       []interface{}{phone},
		"username":    username,
		"customer_id": customerID,
		"device_id":   []interface{}{deviceID},
	}

	// Traits with diverse values
	traits := map[string]interface{}{
		"loyalty_tier":       loyaltyTiers[rng.Intn(len(loyaltyTiers))],
		"engagement_score":   engagementScores[rng.Intn(len(engagementScores))],
		"preferred_category": categories[rng.Intn(len(categories))],
		"spending_score":     spendingScores[rng.Intn(len(spendingScores))],
		"campaign_affinity":  campaignAffinities[rng.Intn(len(campaignAffinities))],
	}

	// Application data
	appData := map[string]map[string]interface{}{
		"app1": {
			"device_type":    deviceTypes[rng.Intn(len(deviceTypes))],
			"session_id":     fmt.Sprintf("sess_%s", uuid.New().String()[:8]),
			"login_method":   loginMethods[rng.Intn(len(loginMethods))],
			"app_preference": appPreferences[rng.Intn(len(appPreferences))],
			"last_purchase":  fmt.Sprintf("order_%d", rng.Intn(100000)),
		},
	}

	return profileModel.ProfileRequest{
		UserId:             fmt.Sprintf("user-%d", index),
		IdentityAttributes: identity,
		Traits:             traits,
		ApplicationData:    appData,
	}
}

// generateEmail produces emails with controlled overlap.
func generateEmail(index, totalProfiles int, overlapPct float64, rng *rand.Rand) string {
	overlapPool := int(float64(totalProfiles) * overlapPct)
	if overlapPool < 1 {
		overlapPool = 1
	}
	if rng.Float64() < overlapPct && overlapPool > 0 {
		sharedIdx := rng.Intn(overlapPool)
		return fmt.Sprintf("shared_%d@example.com", sharedIdx)
	}
	return fmt.Sprintf("user_%d@example.com", index)
}

// generateCustomerID produces customer IDs with controlled overlap.
func generateCustomerID(index, totalProfiles int, overlapPct float64, rng *rand.Rand) string {
	overlapPool := int(float64(totalProfiles) * overlapPct)
	if overlapPool < 1 {
		overlapPool = 1
	}
	if rng.Float64() < overlapPct {
		return fmt.Sprintf("CUST-SHARED-%d", rng.Intn(overlapPool))
	}
	return fmt.Sprintf("CUST-%d", index)
}

// generateDeviceID produces device IDs with controlled overlap.
func generateDeviceID(index, totalProfiles int, overlapPct float64, rng *rand.Rand) string {
	overlapPool := int(float64(totalProfiles) * overlapPct)
	if overlapPool < 1 {
		overlapPool = 1
	}
	if rng.Float64() < overlapPct {
		return fmt.Sprintf("device-shared-%d", rng.Intn(overlapPool))
	}
	return fmt.Sprintf("device-%d", index)
}

// GenerateFilterQueries returns a set of realistic filter queries for benchmarking.
func GenerateFilterQueries() []struct {
	Name   string
	Filter []string
} {
	return []struct {
		Name   string
		Filter []string
	}{
		{Name: "email_eq", Filter: []string{"identity_attributes.email eq 'shared_0@example.com'"}},
		{Name: "loyalty_eq", Filter: []string{"traits.loyalty_tier eq 'gold'"}},
		{Name: "device_co", Filter: []string{"application_data.device_type co 'mobile'"}},
		{Name: "username_sw", Filter: []string{"identity_attributes.username sw 'user_1'"}},
		{Name: "category_eq", Filter: []string{"traits.preferred_category eq 'electronics'"}},
	}
}
