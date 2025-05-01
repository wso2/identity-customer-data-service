package service

import (
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/repository"
)

// GiveConsentToCollect grants consent to an app to collect data
func GiveConsentToCollect(permaID, appID string) error {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.GiveConsent(permaID, appID, "consented_to_collect")
}

// GiveConsentToShare grants consent to an app to collect data
func GiveConsentToShare(permaID, appID string) error {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.GiveConsent(permaID, appID, "consented_to_share")
}

// GetConsentedApps fetches all consented apps for a user
func GetConsentedApps(permaID string) ([]models.Consent, error) {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.GetConsentedApps(permaID)
}

// GetConsentedAppsToCollect fetches all apps user has consented to collect data for
func GetConsentedAppsToCollect(permaID string) ([]string, error) {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.GetConsentedAppsToCollect(permaID)
}

// GetConsentedAppsToShare fetches all apps user has consented to collect data for
func GetConsentedAppsToShare(permaID string) ([]string, error) {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.GetConsentedAppsToShare(permaID)
}

// RevokeConsentToCollect revokes a user's consent to collect data
func RevokeConsentToCollect(permaID, appID string) error {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.RevokeConsent(permaID, appID, "consented_to_collect")
}

// RevokeConsentToShare revokes a user's consent to collect data
func RevokeConsentToShare(permaID, appID string) error {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.RevokeConsent(permaID, appID, "consented_to_collect")
}

// RevokeAllConsents revokes all given consents for a user
func RevokeAllConsents(permaID string) error {
	mongoDB := locks.GetMongoDBInstance()
	consentRepo := repositories.NewConsentRepository(mongoDB.Database, "consents")
	return consentRepo.RevokeAllConsents(permaID)
}
