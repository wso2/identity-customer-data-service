package repositories

import (
	"context"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type EventSchemaRepository struct {
	Collection *mongo.Collection
}

func NewEventSchemaRepository(db *mongo.Database, collection string) *EventSchemaRepository {
	return &EventSchemaRepository{Collection: db.Collection(collection)}
}

func (r *EventSchemaRepository) AddEventSchema(schema models.EventSchema) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Collection.InsertOne(ctx, schema)
	return err
}

func (r *EventSchemaRepository) GetAllEventSchemas() ([]models.EventSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var schemas []models.EventSchema
	err = cursor.All(ctx, &schemas)
	return schemas, err
}

func (r *EventSchemaRepository) GetById(id string) (*models.EventSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var schema models.EventSchema
	err := r.Collection.FindOne(ctx, bson.M{"event_schema_id": id}).Decode(&schema)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &schema, err
}

func (r *EventSchemaRepository) Patch(id string, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": updates}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"event_schema_id": id}, update)
	return err
}

func (r *EventSchemaRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Collection.DeleteOne(ctx, bson.M{"event_schema_id": id})
	return err
}
