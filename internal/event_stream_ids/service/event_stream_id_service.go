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
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"net/http"
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
type EventStreamIdService struct {
	store store.EventStreamIdStoreInterface
}

// GetEventStreamIdService returns a concrete service with store injected
func GetEventStreamIdService() EventStreamIdServiceInterface {
	return &EventStreamIdService{
		store: &store.EventStreamIdStore{}, // ✅ real store implementation
	}
}

// CreateEventStreamId generates and stores a new API key
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

	if err := s.store.InsertEventStreamId(eventStreamId); err != nil {
		return nil, err
	}
	return eventStreamId, nil
}

// GetEventStreamIdPerApp returns an API key for a specific org and app
func (s *EventStreamIdService) GetEventStreamIdPerApp(orgID, appID string) ([]*model.EventStreamId, error) {

	eventStreamId, err := s.store.GetEventStreamIdsPerApp(orgID, appID)
	if err != nil {
		return nil, err
	}
	return eventStreamId, nil
}

// GetEventStreamId retrieves an API key by its value
func (s *EventStreamIdService) GetEventStreamId(eventStreamId string) (*model.EventStreamId, error) {

	key, err := s.store.GetEventStreamId(eventStreamId)
	if err != nil {
		return nil, err
	}
	if key == nil {
		errorMsg := fmt.Sprintf("No meta data found for event stream id: %s", eventStreamId)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, http.StatusNotFound)
		return nil, clientError
	}
	return key, nil
}

// RotateEventStreamId revokes the old API key and creates a new one for the same org/app
func (s *EventStreamIdService) RotateEventStreamId(eventStreamId string) (*model.EventStreamId, error) {

	oldEventStreamId, _ := s.store.GetEventStreamId(eventStreamId)
	if oldEventStreamId == nil {
		errorMsg := fmt.Sprintf("No meta data found for event stream id: %s", eventStreamId)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.GET_EVENT_STREAM_ID.Code,
			Message:     errors2.GET_EVENT_STREAM_ID.Message,
			Description: errorMsg,
		}, http.StatusNotFound)
		return nil, clientError
	}
	if err := s.store.UpdateState(oldEventStreamId.EventStreamId, "revoked"); err != nil {
		return nil, err
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

	if err := s.store.InsertEventStreamId(newEventStreamId); err != nil {
		return nil, err
	}
	return newEventStreamId, nil
}

// RevokeEventStreamId sets the state of the key to 'revoked'
func (s *EventStreamIdService) RevokeEventStreamId(eventStreamId string) error {

	if err := s.store.UpdateState(eventStreamId, "revoked"); err != nil {
		return err
	}
	return nil
}

// generateSecureToken generates a secure token using UUID
func generateSecureToken() string {
	return uuid.New().String()
}
