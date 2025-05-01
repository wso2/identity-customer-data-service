package repositories

import (
	"context"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ConsentRepository handles MongoDB operations for consents
type ConsentRepository struct {
	Collection *mongo.Collection
}

// NewConsentRepository initializes a repository for `consents` collection
func NewConsentRepository(db *mongo.Database, collectionName string) *ConsentRepository {
	return &ConsentRepository{
		Collection: db.Collection(collectionName),
	}
}

// GetConsentedApps fetches all apps with consent status
func (repo *ConsentRepository) GetConsentedApps(permaID string) ([]models.Consent, error) {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"perma_id": permaID}
	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to fetch consents: "+err.Error())
		return nil, err
	}
	defer cursor.Close(ctx)

	var consents []models.Consent
	if err := cursor.All(ctx, &consents); err != nil {
		//logger.LogMessage("ERROR", "Failed to decode consent results: "+err.Error())
		return nil, err
	}

	//logger.LogMessage("INFO", "Fetched consents for user: "+permaID)
	return consents, nil
}

// GetConsentedAppsToCollect fetches only apps where `consented_to_collect=true`
func (repo *ConsentRepository) GetConsentedAppsToCollect(permaID string) ([]string, error) {
	return repo.getConsentedAppsByType(permaID, "consented_to_collect")
}

// GetConsentedAppsToShare fetches only apps where `consented_to_share=true`
func (repo *ConsentRepository) GetConsentedAppsToShare(permaID string) ([]string, error) {
	return repo.getConsentedAppsByType(permaID, "consented_to_share")
}

// Internal function to get list of apps based on consent type
func (repo *ConsentRepository) getConsentedAppsByType(permaID, consentType string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"perma_id": permaID, consentType: true}
	cursor, err := repo.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var apps []string
	for cursor.Next(ctx) {
		var consent models.Consent
		if err := cursor.Decode(&consent); err == nil {
			apps = append(apps, consent.AppID)
		}
	}
	return apps, nil
}

// GiveConsent updates or creates a consent record
func (repo *ConsentRepository) GiveConsent(permaID, appID string, consentType string) error {
	//logger := pkg.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{consentType: true}}
	filter := bson.M{"perma_id": permaID, "app_id": appID}

	_, err := repo.Collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		//logger.LogMessage("ERROR", "Failed to give consent: "+err.Error())
		return err
	}

	//logger.LogMessage("INFO", "Consent updated for user "+permaID+" on app "+appID)
	return nil
}

// RevokeConsent removes a user's consent for an app
func (repo *ConsentRepository) RevokeConsent(permaID, appID string, consentType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{consentType: false}}
	filter := bson.M{"perma_id": permaID, "app_id": appID}

	_, err := repo.Collection.UpdateOne(ctx, filter, update)
	return err
}

// RevokeAllConsents removes all consents for a user
func (repo *ConsentRepository) RevokeAllConsents(permaID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"perma_id": permaID}
	_, err := repo.Collection.DeleteMany(ctx, filter)
	return err
}
