package locks

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB struct holds the client and database name
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

var (
	mongoInstance *MongoDB
	once          sync.Once
)

// ConnectMongoDB initializes a global MongoDB connection
func ConnectMongoDB(uri, dbName string) *MongoDB {
	once.Do(func() {
		//logger := GetLogger()

		// Set connection options
		clientOptions := options.Client().ApplyURI(uri)
		client, err := mongo.NewClient(clientOptions)
		if err != nil {
			//logger.LogMessage("ERROR", "MongoDB client creation failed: "+err.Error())
			log.Fatal(err)
		}

		// Connect to MongoDB
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = client.Connect(ctx)
		if err != nil {
			//logger.LogMessage("ERROR", "MongoDB connection failed: "+err.Error())
			log.Fatal(err)
		}

		// Ping to ensure connection is live
		err = client.Ping(ctx, nil)
		if err != nil {
			//logger.LogMessage("ERROR", "MongoDB ping failed: "+err.Error())
			log.Fatal(err)
		}

		//logger.LogMessage("INFO", "✅ Connected to MongoDB")

		// Assign global instance
		mongoInstance = &MongoDB{
			Client:   client,
			Database: client.Database(dbName),
		}
	})

	return mongoInstance
}

// GetMongoDBInstance returns the MongoDB instance
func GetMongoDBInstance() *MongoDB {
	return mongoInstance
}
