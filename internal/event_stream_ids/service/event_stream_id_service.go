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
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/model"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/store"
)

type EventStreamIdServiceInterface interface {
	CreateEventStreamId(orgID, appID string) (*model.EventStreamId, error)
	GetEventStreamIdPerApp(orgID, appID string) ([]*model.EventStreamId, error)
	GetEventStreamId(eventStreamId string) (*model.EventStreamId, error)
	RotateEventStreamId(eventStreamId string) (*model.EventStreamId, error)
	RevokeEventStreamId(eventStreamId string) error
}

// EventStreamIdService is the default implementation of EventStreamIdServiceInterface.
type EventStreamIdService struct{}

// GetEventStreamIdService returns a concrete service with store injected
func GetEventStreamIdService() EventStreamIdServiceInterface {
	return &EventStreamIdService{}
}

// CreateAPIKey generates and stores a new API key
func (s *EventStreamIdService) CreateEventStreamId(orgID, appID string) (*model.EventStreamId, error) {
	key := generateSecureToken()
	now := time.Now().Unix()
	exp := now + (60 * 60 * 24 * 365) // 1 year

	eventStreamId := &model.EventStreamId{
		EventStreamId: key,
		OrgID:         orgID,
		AppID:         appID,
		State:         "active",
		ExpiresAt:     exp,
		CreatedAt:     now,
	}

	if err := store.InsertEventStreamId(eventStreamId); err != nil {
		return nil, fmt.Errorf("failed to insert API key: %w", err)
	}
	return eventStreamId, nil
}

// GetEventStreamIdPerApp returns an API key for a specific org and app
func (s *EventStreamIdService) GetEventStreamIdPerApp(orgID, appID string) ([]*model.EventStreamId, error) {
	eventStreamId, err := store.GetEventStreamIdsPerApp(orgID, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Event stream id: %w", err)
	}
	return eventStreamId, nil
}

// GetEventStreamId retrieves an API key by its value
func (s *EventStreamIdService) GetEventStreamId(eventStreamId string) (*model.EventStreamId, error) {
	key, err := store.GetEventStreamId(eventStreamId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve eventStreamId: %w", err)
	}
	return key, nil
}

// RotateEventStreamId revokes the old API key and creates a new one for the same org/app
func (s *EventStreamIdService) RotateEventStreamId(eventStreamId string) (*model.EventStreamId, error) {

	oldEventStreamId, _ := store.GetEventStreamId(eventStreamId)
	if err := store.UpdateState(oldEventStreamId.EventStreamId, "revoked"); err != nil {
		return nil, fmt.Errorf("failed to revoke old API eventStreamId: %w", err)
	}

	eventStreamId = generateSecureToken()
	now := time.Now().Unix()
	exp := now + (60 * 60 * 24 * 365)

	newEventStreamId := &model.EventStreamId{
		EventStreamId: eventStreamId,
		OrgID:         oldEventStreamId.OrgID,
		AppID:         oldEventStreamId.AppID,
		State:         "active",
		ExpiresAt:     exp,
		CreatedAt:     now,
	}

	if err := store.InsertEventStreamId(newEventStreamId); err != nil {
		return nil, fmt.Errorf("failed to insert new API eventStreamId: %w", err)
	}
	return newEventStreamId, nil
}

// RevokeEventStreamId sets the state of the key to 'revoked'
func (s *EventStreamIdService) RevokeEventStreamId(eventStreamId string) error {
	if err := store.UpdateState(eventStreamId, "revoked"); err != nil {
		return fmt.Errorf("failed to revoke EventStreamId: %w", err)
	}
	return nil
}

func generateSecureToken() string {
	return uuid.New().String()
}
