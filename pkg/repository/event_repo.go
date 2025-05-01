package repositories

import (
	"context"
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// EventRepository handles MongoDB operations for user events
type EventRepository struct {
	Collection *mongo.Collection
}

// NewEventRepository initializes a repository for `events` collection
func NewEventRepository(db *mongo.Database, collectionName string) *EventRepository {
	return &EventRepository{
		Collection: db.Collection(collectionName),
	}
}

// AddEvent inserts a single event into MongoDB
func (repo *EventRepository) AddEvent(event models.Event) error {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := repo.Collection.InsertOne(ctx, event)
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to insert event: "+err.Error())
		return err
	}

	//logger.LogMessage("INFO", "Event inserted successfully for user "+event.PermaId)
	return nil
}

// AddEvents inserts multiple events in bulk
func (repo *EventRepository) AddEvents(events []models.Event) error {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var docs []interface{}
	for _, event := range events {
		docs = append(docs, event)
	}

	_, err := repo.Collection.InsertMany(ctx, docs)
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to insert multiple events: "+err.Error())
		return err
	}

	//logger.LogMessage("INFO", "Batch events inserted successfully")
	return nil
}

// FindEvents fetches all events for a user
func (repo *EventRepository) FindEvents(filters []string, timeFilter bson.M) ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, value := parts[0], strings.ToLower(parts[1]), parts[2]

		switch operator {
		case "eq":
			filter[field] = value
		case "sw":
			filter[field] = bson.M{"$regex": fmt.Sprintf("^%s", value)}
		case "co":
			filter[field] = bson.M{"$regex": value}
		}
	}

	// Add time filter if provided
	for k, v := range timeFilter {
		filter[k] = v
	}

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (repo *EventRepository) FindEvent(eventId string) (*models.Event, error) {
	filter := bson.M{"event_id": eventId}
	var event models.Event
	err := repo.Collection.FindOne(context.TODO(), filter).Decode(&event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (repo *EventRepository) DeleteEvent(eventId string) error {
	filter := bson.M{"event_id": eventId}
	_, err := repo.Collection.DeleteOne(context.TODO(), filter)
	return err
}

func (repo *EventRepository) DeleteEventsByProfileId(permaID string) error {
	_, err := repo.Collection.DeleteMany(context.TODO(), bson.M{"perma_id": permaID})
	return err
}

func (repo *EventRepository) DeleteEventsByAppID(permaID, appID string) error {
	filter := bson.M{"profile_id": permaID, "application_id": appID}
	_, err := repo.Collection.DeleteMany(context.TODO(), filter)
	return err
}

func (repo *EventRepository) FindEventsWithFilter(filter bson.M) ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.Event
	err = cursor.All(ctx, &events)
	return events, err
}
