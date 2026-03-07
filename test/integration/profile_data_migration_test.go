/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
)

// insertTestProfile inserts a minimal profile row with the given JSONB traits blob.
// Returns the profileId. Caller is responsible for cleanup.
func insertTestProfile(t *testing.T, orgHandle string, traits map[string]interface{}) string {
	t.Helper()

	db := provider.NewDBProvider().GetRawDB()
	profileId := uuid.New().String()

	traitsJSON, err := json.Marshal(traits)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO profiles (profile_id, org_handle, traits, created_at, updated_at, list_profile)
		VALUES ($1, $2, $3, $4, $4, true)
	`, profileId, orgHandle, string(traitsJSON), time.Now())
	require.NoError(t, err, "failed to insert test profile")

	return profileId
}

// readTraits fetches the raw traits JSONB for a profile row.
func readTraits(t *testing.T, profileId string) map[string]interface{} {
	t.Helper()

	db := provider.NewDBProvider().GetRawDB()
	var raw string
	err := db.QueryRow(`SELECT traits FROM profiles WHERE profile_id = $1`, profileId).Scan(&raw)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	return result
}

// deleteTestProfile removes a profile row inserted for a test.
func deleteTestProfile(t *testing.T, profileId string) {
	t.Helper()
	db := provider.NewDBProvider().GetRawDB()
	_, _ = db.Exec(`DELETE FROM profiles WHERE profile_id = $1`, profileId)
}

// getNestedString navigates a nested map using a dot-separated path and returns the leaf as a string.
func getNestedString(m map[string]interface{}, keys ...string) (string, bool) {
	var cur interface{} = m
	for _, k := range keys {
		mp, ok := cur.(map[string]interface{})
		if !ok {
			return "", false
		}
		cur, ok = mp[k]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

func Test_ProfileDataMigration(t *testing.T) {
	org := fmt.Sprintf("migration-test-%d", time.Now().UnixNano())

	t.Run("MigrateRemovedComplexSubAttribute_MovesValueToFlatKey", func(t *testing.T) {
		// Start: {"address": {"city": "NY", "zip": "10001"}}
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"address": map[string]interface{}{
				"city": "NY",
				"zip":  "10001",
			},
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.MigrateRemovedComplexSubAttribute(org, constants.Traits, "address", "city")
		require.NoError(t, err)

		traits := readTraits(t, profileId)

		// Flat key "address.city" must exist with preserved value.
		require.Equal(t, "NY", traits["address.city"], "flat key should hold the removed sub-attribute value")

		// Nested "address" object must still exist but without "city".
		addr, ok := traits["address"].(map[string]interface{})
		require.True(t, ok, "address key should still be a nested object")
		require.NotContains(t, addr, "city", "city should be removed from nested object")
		require.Equal(t, "10001", addr["zip"], "zip should remain in nested object")
	})

	t.Run("MigrateRemovedComplexSubAttribute_NoOpWhenSubKeyAbsent", func(t *testing.T) {
		// "city" is not present in the nested object — migration should not error.
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"address": map[string]interface{}{
				"zip": "10001",
			},
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.MigrateRemovedComplexSubAttribute(org, constants.Traits, "address", "city")
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		require.NotContains(t, traits, "address.city", "flat key should not appear when sub-key was absent")
	})

	t.Run("MigrateAddedComplexSubAttribute_FoldsFlatKeyIntoNested", func(t *testing.T) {
		// Start: flat key exists, nested object has zip only.
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"address":      map[string]interface{}{"zip": "10001"},
			"address.city": "NY",
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.MigrateAddedComplexSubAttribute(org, constants.Traits, "address", "city")
		require.NoError(t, err)

		traits := readTraits(t, profileId)

		// Flat key must be removed.
		require.NotContains(t, traits, "address.city", "flat key should be removed after folding")

		// Nested object must contain both keys.
		city, ok := getNestedString(traits, "address", "city")
		require.True(t, ok, "address.city should be present as nested key")
		require.Equal(t, "NY", city)

		zip, ok := getNestedString(traits, "address", "zip")
		require.True(t, ok, "address.zip should still be present")
		require.Equal(t, "10001", zip)
	})

	t.Run("MigrateAddedComplexSubAttribute_NoOpWhenFlatKeyAbsent", func(t *testing.T) {
		// No flat key — nothing to fold in, should not error.
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"address": map[string]interface{}{"zip": "10001"},
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.MigrateAddedComplexSubAttribute(org, constants.Traits, "address", "city")
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		addr, ok := traits["address"].(map[string]interface{})
		require.True(t, ok)
		require.NotContains(t, addr, "city", "city should not appear when there was no flat key to fold")
	})

	t.Run("CoerceProfileAttributeType_StringToInteger_ValidValue", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"age": "30",
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"age"}, "",
			constants.StringDataType, constants.IntegerDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		age, ok := traits["age"].(float64) // JSON numbers decode as float64
		require.True(t, ok, "age should be a number after coercion")
		require.EqualValues(t, 30, age)
	})

	t.Run("CoerceProfileAttributeType_StringToInteger_InvalidValue_Nullified", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"age": "twenty-five",
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"age"}, "",
			constants.StringDataType, constants.IntegerDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		// Key must exist but value must be null (decoded as nil in Go).
		val, exists := traits["age"]
		require.True(t, exists, "age key should still exist")
		require.Nil(t, val, "age value should be null after failed coercion")
	})

	t.Run("CoerceProfileAttributeType_BooleanToInteger", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"flag": true,
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"flag"}, "",
			constants.BooleanDataType, constants.IntegerDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		flag, ok := traits["flag"].(float64)
		require.True(t, ok, "flag should be a number after coercion")
		require.EqualValues(t, 1, flag)
	})

	t.Run("CoerceProfileAttributeType_IntegerToDecimal", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"score": 42,
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"score"}, "",
			constants.IntegerDataType, constants.DecimalDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		score, ok := traits["score"].(float64)
		require.True(t, ok)
		require.EqualValues(t, 42.0, score)
	})

	t.Run("CoerceProfileAttributeType_StringToBoolean_ValidValue", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"active": "true",
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"active"}, "",
			constants.StringDataType, constants.BooleanDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		active, ok := traits["active"].(bool)
		require.True(t, ok, "active should be a boolean after coercion")
		require.True(t, active)
	})

	t.Run("CoerceProfileAttributeType_IncompatibleTypes_Nullified", func(t *testing.T) {
		// boolean → date_time: no coercion path → values should be nullified.
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"flag": true,
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"flag"}, "",
			constants.BooleanDataType, constants.DateTimeDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		val, exists := traits["flag"]
		require.True(t, exists, "flag key should still exist")
		require.Nil(t, val, "flag should be null when no coercion path exists")
	})

	t.Run("CoerceProfileAttributeType_IntegerToString", func(t *testing.T) {
		profileId := insertTestProfile(t, org, map[string]interface{}{
			"count": 7,
		})
		defer deleteTestProfile(t, profileId)

		err := profileStore.CoerceProfileAttributeType(org, constants.Traits, []string{"count"}, "",
			constants.IntegerDataType, constants.StringDataType)
		require.NoError(t, err)

		traits := readTraits(t, profileId)
		count, ok := traits["count"].(string)
		require.True(t, ok, "count should be a string after coercion")
		require.Equal(t, "7", count)
	})
}

