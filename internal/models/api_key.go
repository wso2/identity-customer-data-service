package models

type APIKey struct {
	KeyID     string `bson:"key_id"`
	OrgID     string `bson:"org_id"`
	AppID     string `bson:"app_id"`
	State     string `bson:"state"`
	ExpiresAt int    `bson:"expires_at"`
	CreatedAt int    `bson:"created_at"`
}
