package locks

import (
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type DistributedLock interface {
	Acquire(key string, ttl time.Duration) (bool, error)
	Release(key string) error
}

var mongoLock DistributedLock

func InitLocks(db *mongo.Database) {
	mongoLock = NewMongoLock(db)
}

func GetDistributedLock() DistributedLock {
	return mongoLock
}
