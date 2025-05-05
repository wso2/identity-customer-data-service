package locks

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoLock struct {
	Collection *mongo.Collection
}

func NewMongoLock(db *mongo.Database) DistributedLock {
	return &MongoLock{
		Collection: db.Collection("locks"),
	}
}

func (l *MongoLock) Acquire(key string, ttl time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	lock := bson.M{
		"_id":        key,
		"created_at": time.Now(),
		"expires_at": time.Now().Add(ttl),
	}

	_, err := l.Collection.InsertOne(ctx, lock)
	if err != nil {
		// Duplicate key => lock already held
		return false, nil
	}

	return true, nil
}

func (l *MongoLock) Release(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := l.Collection.DeleteOne(ctx, bson.M{"_id": key})
	return err
}
