package models

// Consent represents a user's consent record for an app
type Consent struct {
	PermaID            string `json:"perma_id" bson:"perma_id" binding:"required"`
	AppID              string `json:"app_id" bson:"app_id" binding:"required"`
	ConsentedToCollect bool   `json:"consented_to_collect" bson:"consented_to_collect"`
	ConsentedToShare   bool   `json:"consented_to_share" bson:"consented_to_share"`
}
