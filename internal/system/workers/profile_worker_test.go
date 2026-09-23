/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package workers

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	_ "modernc.org/sqlite"
)

func TestUnificationEventIDIsStableForOlderMessages(t *testing.T) {
	profile := profileModel.Profile{
		ProfileId: "profile-1",
		UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 123, time.UTC),
	}
	first := unificationEventID(profile)
	require.NotEmpty(t, first)
	require.Equal(t, first, unificationEventID(profile))
	profile.UpdatedAt = profile.UpdatedAt.Add(time.Nanosecond)
	require.NotEqual(t, first, unificationEventID(profile))
	profile.UnificationEventID = "explicit-event"
	require.Equal(t, "explicit-event", unificationEventID(profile))
}

func TestStartProfileWorkerRequiresUnificationEventTable(t *testing.T) {
	db, err := sql.Open(database.DriverSQLite, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	provider.SetTestDB(db, database.TypeSQLite)
	t.Cleanup(func() { provider.SetTestDB(nil, "") })
	err = StartProfileWorker()
	require.ErrorContains(t, err, "profile unification event storage is unavailable")
}
