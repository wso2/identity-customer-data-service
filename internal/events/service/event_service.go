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
	erm "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/events/store"
	provider "github.com/wso2/identity-customer-data-service/internal/profile/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"

	"strconv"
	"strings"
	"time"
)

type EventsServiceInterface interface {
	AddEvents(event model.Event, queue EventQueue) error
	GetEvents(filters []string, timeFilter map[string]int) ([]model.Event, error)
	GetEvent(eventId string) (*model.Event, error)
}

// EventsService is the default implementation of the EventsServiceInterface.
type EventsService struct{}

// GetEventsService creates a new instance of EventsService.
func GetEventsService() EventsServiceInterface {

	return &EventsService{}
}

// EventQueue Define an interface for enqueuing events
// This interface will be implemented by the actual worker in `workers`
type EventQueue interface {
	Enqueue(event model.Event)
}

// AddEvents stores a single event in MongoDB
func (es *EventsService) AddEvents(event model.Event, queue EventQueue) error {

	// Step 1: Ensure profile exists (with lock protection)
	profilesProvider := provider.NewProfilesProvider()
	profileService := profilesProvider.GetProfilesService()
	err := profileService.CreateOrUpdateProfile(event)
	logger := log.GetLogger()
	if err != nil {
		logger.Debug(fmt.Sprintf("failed to create or fetch profile with id: %s", event.ProfileId),
			log.Error(err))
		return err
		// todo: should we throw an error here - stems from CreateOrUpdateProfile - becz
	}

	isValid, err := es.validateEvent(event)
	if err != nil || !isValid {
		logger.Debug(fmt.Sprintf("failed to validate event with id: %s", event.EventId), log.Error(err))
		return err
	}

	// Step 2: Store the event
	event.EventType = strings.ToLower(event.EventType)
	event.EventName = strings.ToLower(event.EventName)
	if err := store.AddEvent(event); err != nil {
		logger.Debug(fmt.Sprintf("failed to persist event with id: %s", event.EventId), log.Error(err))
		return err
	}

	// Step 3: Enqueue the event for enrichment/unification (async)
	queue.Enqueue(event)

	return nil
}

// validateEvent validates the event before storing it
func (es *EventsService) validateEvent(event model.Event) (bool, error) {

	if event.EventId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: "Event id is required",
		}, http.StatusBadRequest)
		return false, clientError
	}

	if event.EventName == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: "Event name is required.",
		}, http.StatusBadRequest)
		return false, clientError
	}

	if event.ProfileId == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: "Profile id is required.",
		}, http.StatusBadRequest)
		return false, clientError
	}

	if event.EventType == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: "Event type is required.",
		}, http.StatusBadRequest)
		return false, clientError
	}

	if !constants.AllowedEventTypes[strings.ToLower(event.EventType)] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: fmt.Sprintf("'%s' is not an expected event type.", event.EventType),
		}, http.StatusBadRequest)
		return false, clientError
	}

	if int64(event.EventTimestamp) > time.Now().UTC().Unix() {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_EVENT.Code,
			Message:     errors2.INVALID_EVENT.Message,
			Description: "Event can not happen in the future. We only accept timestamps in UTC.",
		}, http.StatusBadRequest)
		return false, clientError
	}

	return true, nil
}

// GetEvents retrieves all events
func (es *EventsService) GetEvents(filters []string, timeFilter map[string]int) ([]model.Event, error) {

	return store.FindEvents(filters, timeFilter)
}

func (es *EventsService) GetEvent(eventId string) (*model.Event, error) {

	event, err := store.FindEvent(eventId)
	if err != nil {
		logger := log.GetLogger()
		logger.Debug(fmt.Sprintf("Failed to fetch event with id: %s", eventId), log.Error(err))
		return nil, err
	}
	if event == nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.EVENT_NOT_FOUND.Code,
			Message:     errors2.EVENT_NOT_FOUND.Message,
			Description: fmt.Sprintf("Event with ID %s not found", eventId),
		}, http.StatusNotFound)
		return nil, clientError
	}
	return event, nil
}

// CountEventsMatchingRule retrieves count of events that has occurred in a time range
func CountEventsMatchingRule(profileId string, trigger erm.RuleTrigger, timeRange int64) (int, error) {

	currentTime := time.Now().UTC().Unix() // current time in seconds
	startTime := currentTime - timeRange   // assuming value is in minutes

	timeFilter := map[string]int{
		"event_timestamp_gte": int(startTime), // Use the key expected by Postgres FindEvents
	}
	rawFilters := []string{
		fmt.Sprintf("profile_id:%s", profileId),
		fmt.Sprintf("event_type:%s", strings.ToLower(trigger.EventType)),
		fmt.Sprintf("event_name:%s", strings.ToLower(trigger.EventName)),
	}

	events, err := store.FindEvents(rawFilters, timeFilter)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch events for counting: %v", err)
	}
	count := 0
	for _, event := range events {
		if EvaluateConditions(event, trigger.Conditions) {
			count++
		}
	}
	return count, nil
}

// EvaluateConditions evaluates the conditions of a rule against an event
func EvaluateConditions(event model.Event, triggerConditions []erm.RuleCondition) bool {
	for _, cond := range triggerConditions {
		fieldVal := GetFieldFromEvent(event, cond.Field)
		if !EvaluateCondition(fieldVal, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}

// EvaluateCondition evaluates a single condition against an actual value
func EvaluateCondition(actual interface{}, operator string, expected string) bool {
	switch strings.ToLower(operator) {
	case "equals":
		return fmt.Sprintf("%v", actual) == expected

	case "not_equals":
		return fmt.Sprintf("%v", actual) != expected

	case "exists":
		return actual != nil && fmt.Sprintf("%v", actual) != ""

	case "not_exists":
		return actual == nil || fmt.Sprintf("%v", actual) == ""

	case "contains":
		if str, ok := actual.(string); ok {
			return strings.Contains(str, expected)
		}
		return false

	case "not_contains":
		if str, ok := actual.(string); ok {
			return !strings.Contains(str, expected)
		}
		return false

	case "greater_than":
		return compareNumeric(actual, expected, ">")

	case "greater_than_equals":
		return compareNumeric(actual, expected, ">=")

	case "less_than":
		return compareNumeric(actual, expected, "<")

	case "less_than_equals":
		return compareNumeric(actual, expected, "<=")

	default:
		return false
	}
}

// compareNumeric compares a numeric value with a string representation of a number
func compareNumeric(actual interface{}, expected string, op string) bool {
	actualFloat, err1 := toFloat(actual)
	expectedFloat, err2 := strconv.ParseFloat(expected, 64)
	if err1 != nil || err2 != nil {
		return false
	}

	switch op {
	case ">":
		return actualFloat > expectedFloat
	case ">=":
		return actualFloat >= expectedFloat
	case "<":
		return actualFloat < expectedFloat
	case "<=":
		return actualFloat <= expectedFloat
	default:
		return false
	}
}

// GetFieldFromEvent retrieves a field from the event properties
func GetFieldFromEvent(event model.Event, field string) interface{} {
	if event.Properties == nil {
		return nil
	}

	if val, ok := event.Properties[field]; ok {
		return val
	}
	return nil
}

// toFloat converts various types to float64
func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.INVALID_TYPE.Code,
			Message:     errors2.INVALID_TYPE.Message,
			Description: fmt.Sprintf("Invalid type for conversion to float: %T", v),
		}, nil)
		return 0, serverError
	}
}
