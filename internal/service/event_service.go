package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/database"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/models"
	repositories "github.com/wso2/identity-customer-data-service/internal/repository"
	"log"
	"strconv"
	"strings"
	"time"
)

// AddEvents stores a single event in MongoDB
func AddEvents(event models.Event) error {

	// Step 1: Ensure profile exists (with lock protection)
	_, err := CreateOrUpdateProfile(event)
	if err != nil {
		return fmt.Errorf("failed to create or fetch profile: %v", err)
	}

	// Step 2: Store the event
	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories.NewEventRepository(postgresDB.DB)
	event.EventType = strings.ToLower(event.EventType)
	event.EventName = strings.ToLower(event.EventName)
	if err := eventRepo.AddEvent(event); err != nil {

		return fmt.Errorf("failed to store event: %v", err)
	}

	// Step 3: Enqueue the event for enrichment/unification (async)
	EnqueueEventForProcessing(event)

	return nil
}

// GetEvents retrieves all events
func GetEvents(filters []string, timeFilter map[string]int) ([]models.Event, error) {
	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories.NewEventRepository(postgresDB.DB)
	return eventRepo.FindEvents(filters, timeFilter)
}

func GetEvent(eventId string) (*models.Event, error) {
	postgresDB := database.GetPostgresInstance()
	eventRepo := repositories.NewEventRepository(postgresDB.DB)
	return eventRepo.FindEvent(eventId)
}

// CountEventsMatchingRule retrieves count of events that has occured in a timerange
func CountEventsMatchingRule(profileId string, trigger model.RuleTrigger, timeRange int64) (int, error) {

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
	events, err := GetEvents(rawFilters, timeFilter)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch events for counting: %v", err)
	}
	count := 0
	for _, event := range events {
		log.Printf("Evaluating event: %v", event)
		if EvaluateConditions(event, trigger.Conditions) {
			log.Printf("incrementing")
			count++
		}
	}
	return count, nil
}

func EvaluateConditions(event models.Event, triggerConditions []model.RuleCondition) bool {
	for _, cond := range triggerConditions {
		fieldVal := GetFieldFromEvent(event, cond.Field)
		if !EvaluateCondition(fieldVal, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}
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

func GetFieldFromEvent(event models.Event, field string) interface{} {
	if event.Properties == nil {
		return nil
	}

	if val, ok := event.Properties[field]; ok {
		return val
	}
	return nil
}

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
		return 0, fmt.Errorf("cannot convert to float")
	}
}
