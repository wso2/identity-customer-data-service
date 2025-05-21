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
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/service"
	"testing"
)

func Test_EventStreamId(t *testing.T) {
	svc := service.GetEventStreamIdService()

	orgID := "test-org"
	appID := "test-app"

	t.Run("Create_event_stream_Id", func(t *testing.T) {
		key, err := svc.CreateEventStreamId(orgID, appID)
		require.NoError(t, err, "Failed to create event stream ID")
		require.Equal(t, orgID, key.OrgID)
		require.Equal(t, appID, key.AppID)
		require.Equal(t, "active", key.State)
		require.NotEmpty(t, key.EventStreamId)
	})

	var streamID string
	t.Run("Fetch_event_stream_Id_by_app", func(t *testing.T) {
		keys, err := svc.GetEventStreamIdPerApp(orgID, appID)
		require.NoError(t, err, "Failed to fetch event stream ID by app")
		require.NotEmpty(t, keys)
		streamID = keys[0].EventStreamId
	})

	t.Run("Get_event_stream_Id_by_Id", func(t *testing.T) {
		key, err := svc.GetEventStreamId(streamID)
		require.NoError(t, err, "Failed to get event stream ID")
		require.Equal(t, streamID, key.EventStreamId)
	})

	t.Run("Rotate_event_stream_Id", func(t *testing.T) {
		newKey, err := svc.RotateEventStreamId(streamID)
		require.NoError(t, err, "Failed to rotate event stream ID")
		require.NotEqual(t, streamID, newKey.EventStreamId)
		require.Equal(t, "active", newKey.State)

		// old key should now be revoked
		oldKey, err := svc.GetEventStreamId(streamID)
		require.NoError(t, err)
		require.Equal(t, "revoked", oldKey.State)
	})

	t.Run("Revoke_event_stream_Id", func(t *testing.T) {
		keys, _ := svc.GetEventStreamIdPerApp(orgID, appID)
		require.NotEmpty(t, keys)

		latestKey := keys[len(keys)-1].EventStreamId
		err := svc.RevokeEventStreamId(latestKey)
		require.NoError(t, err, "Failed to revoke event stream ID")

		key, _ := svc.GetEventStreamId(latestKey)
		require.Equal(t, "revoked", key.State)
	})
}
