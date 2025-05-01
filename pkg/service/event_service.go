package service

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	repositories "github.com/wso2/identity-customer-data-service/pkg/repository"
	"go.mongodb.org/mongo-driver/bson"
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
	mongoDB := locks.GetMongoDBInstance()
	eventRepo := repositories.NewEventRepository(mongoDB.Database, constants.EventCollection)
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
func GetEvents(filters []string, timeFilter bson.M) ([]models.Event, error) {
	mongoDB := locks.GetMongoDBInstance()
	eventRepo := repositories.NewEventRepository(mongoDB.Database, constants.EventCollection)
	return eventRepo.FindEvents(filters, timeFilter)
}

func GetEvent(eventId string) (*models.Event, error) {
	mongoDB := locks.GetMongoDBInstance()
	eventRepo := repositories.NewEventRepository(mongoDB.Database, constants.EventCollection)
	return eventRepo.FindEvent(eventId)
}

// CountEventsMatchingRule retrieves count of events that has occured in a timerange
func CountEventsMatchingRule(profileId string, trigger models.RuleTrigger, timeRange string) (int, error) {

	eventRepo := repositories.NewEventRepository(locks.GetMongoDBInstance().Database, constants.EventCollection)
	durationInSec, err := strconv.Atoi(timeRange) // parse string to int
	if err != nil {
		log.Printf("Invalid time range format: %v", err)
		//return
	}

	currentTime := time.Now().UTC().Unix()          // current time in seconds
	startTime := currentTime - int64(durationInSec) // assuming value is in minutes
	log.Println("efef", trigger.EventType)
	log.Println("efef", trigger.EventName)

	filter := bson.M{
		"profile_id": profileId,
		"event_type": strings.ToLower(trigger.EventType),
		"event_name": strings.ToLower(trigger.EventName),
		"event_timestamp": bson.M{
			"$gte": startTime,
		},
	}

	// Fetch matching events
	events, err := eventRepo.FindEventsWithFilter(filter)
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

func EvaluateConditions(event models.Event, triggerConditions []models.RuleCondition) bool {
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
