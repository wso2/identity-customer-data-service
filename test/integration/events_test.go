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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/events/service"
	profservice "github.com/wso2/identity-customer-data-service/internal/profile/service"
	profileworker "github.com/wso2/identity-customer-data-service/internal/system/workers"
	"testing"
	"time"
)

func Test_Events(t *testing.T) {
	eventSvc := service.GetEventsService()
	queue := &profileworker.ProfileWorkerQueue{}

	event := model.Event{
		EventId:   uuid.New().String(),
		ProfileId: uuid.New().String(),
		EventType: "track",
		EventName: "item_purchased",
		Properties: map[string]interface{}{
			"amount":     120.5,
			"item_name":  "Wireless Mouse",
			"category":   "electronics",
			"successful": true,
		},
		EventTimestamp: int(time.Now().UTC().Unix()),
	}

	t.Run("Add_event", func(t *testing.T) {
		err := eventSvc.AddEvents(event, queue)
		require.NoError(t, err, "Failed to add event")
	})

	t.Run("Get_Profile", func(t *testing.T) {
		profileId := event.ProfileId
		profileService := profservice.GetProfilesService()
		profile, err := profileService.GetProfile(profileId)
		require.NoError(t, err, "Failed to get profile")
		require.Equal(t, profileId, profile.ProfileId, "Profile ID mismatch")
	})

	t.Run("Get_event_by_ID", func(t *testing.T) {
		result, err := eventSvc.GetEvent(event.EventId)
		require.NoError(t, err, "Failed to get event")
		require.Equal(t, event.EventId, result.EventId, "Event ID mismatch")
		require.Equal(t, "item_purchased", result.EventName)
	})

	t.Run("Get_events_with_filters", func(t *testing.T) {
		filters := []string{
			"event_type:track",
			"event_name:item_purchased",
			"profile_id:" + event.ProfileId,
		}
		timeFilter := map[string]int{
			"event_timestamp_gte": int(time.Now().UTC().Add(-5 * time.Minute).Unix()),
		}
		events, err := eventSvc.GetEvents(filters, timeFilter)
		require.NoError(t, err, "Failed to fetch events with filters")
		require.GreaterOrEqual(t, len(events), 1, "Expected at least one event")
	})
}
