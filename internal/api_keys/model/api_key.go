package model

type APIKey struct {
	APIKey    string `db:"api_key"`
	OrgID     string `db:"org_id"`
	AppID     string `db:"app_id"`
	State     string `db:"state"`
	ExpiresAt int64  `db:"expires_at"`
	CreatedAt int64  `db:"created_at"`
}
