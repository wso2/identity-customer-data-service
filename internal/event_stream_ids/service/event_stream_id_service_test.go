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

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/model"
)

// MockEventStreamIdStore implements store.EventStreamIdStoreInterface for testing
type MockEventStreamIdStore struct {
	mock.Mock
}

func (m *MockEventStreamIdStore) InsertEventStreamId(e *model.EventStreamId) error {
	args := m.Called(e)
	return args.Error(0)
}

func (m *MockEventStreamIdStore) GetEventStreamId(eventStreamId string) (*model.EventStreamId, error) {
	args := m.Called(eventStreamId)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*model.EventStreamId), args.Error(1)
}

func (m *MockEventStreamIdStore) GetEventStreamIdsPerApp(orgID, appID string) ([]*model.EventStreamId, error) {
	args := m.Called(orgID, appID)
	return args.Get(0).([]*model.EventStreamId), args.Error(1)
}

func (m *MockEventStreamIdStore) UpdateState(eventStreamId string, state string) error {
	args := m.Called(eventStreamId, state)
	return args.Error(0)
}

func TestCreateEventStreamId(t *testing.T) {

	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	// Expect InsertEventStreamId to be called with any *EventStreamId
	mockStore.
		On("InsertEventStreamId", mock.MatchedBy(func(e *model.EventStreamId) bool {
			return e.OrgID == "org1" && e.AppID == "app1"
		})).
		Return(nil)

	result, err := svc.CreateEventStreamId("org1", "app1")

	assert.NoError(t, err)
	assert.Equal(t, "org1", result.OrgID)
	assert.Equal(t, "app1", result.AppID)
	assert.Equal(t, "active", result.State)
	assert.True(t, result.ExpiresAt > time.Now().Unix())

	mockStore.AssertExpectations(t)
}

func TestGetEventStreamIdPerApp(t *testing.T) {

	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	expected := []*model.EventStreamId{
		{EventStreamId: "id1", OrgID: "org1", AppID: "app1"},
	}

	mockStore.On("GetEventStreamIdsPerApp", "org1", "app1").Return(expected, nil)

	result, err := svc.GetEventStreamIdPerApp("org1", "app1")

	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockStore.AssertExpectations(t)
}

func TestGetEventStreamId_Found(t *testing.T) {
	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	expected := &model.EventStreamId{EventStreamId: "id1", OrgID: "org1", AppID: "app1"}

	mockStore.On("GetEventStreamId", "id1").Return(expected, nil)

	result, err := svc.GetEventStreamId("id1")

	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockStore.AssertExpectations(t)
}

func TestGetEventStreamId_NotFound(t *testing.T) {
	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	mockStore.On("GetEventStreamId", "missing").Return(nil, nil)

	result, err := svc.GetEventStreamId("missing")

	assert.Nil(t, result)
	assert.Error(t, err)

	mockStore.AssertExpectations(t)
}

func TestRotateEventStreamId(t *testing.T) {
	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	old := &model.EventStreamId{EventStreamId: "old_id", OrgID: "org1", AppID: "app1", State: "active"}

	mockStore.On("GetEventStreamId", "old_id").Return(old, nil)
	mockStore.On("UpdateState", "old_id", "revoked").Return(nil)
	mockStore.On("InsertEventStreamId", mock.MatchedBy(func(e *model.EventStreamId) bool {
		return e.OrgID == "org1" && e.AppID == "app1" && e.State == "active"
	})).Return(nil)

	result, err := svc.RotateEventStreamId("old_id")

	assert.NoError(t, err)
	assert.Equal(t, "org1", result.OrgID)
	assert.Equal(t, "app1", result.AppID)
	assert.Equal(t, "active", result.State)

	mockStore.AssertExpectations(t)
}

func TestRevokeEventStreamId(t *testing.T) {
	mockStore := new(MockEventStreamIdStore)
	svc := EventStreamIdService{store: mockStore}

	mockStore.On("UpdateState", "id1", "revoked").Return(nil)

	err := svc.RevokeEventStreamId("id1")

	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
}
