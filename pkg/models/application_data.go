package models

// ApplicationData represents contextual data for an application
type ApplicationData struct {
	AppId           string                 `json:"application_id" bson:"application_id"`
	Devices         []Devices              `json:"devices,omitempty" bson:"devices,omitempty"`
	AppSpecificData map[string]interface{} `json:"app_specific_data,omitempty" bson:"app_specific_data,omitempty"`
}

// Devices represents user devices
type Devices struct {
	DeviceId       string `json:"device_id,omitempty" bson:"device_id,omitempty"`
	DeviceType     string `json:"device_type,omitempty" bson:"device_type,omitempty"`
	LastUsed       int    `json:"last_used,omitempty" bson:"last_used,omitempty"`
	Os             string `json:"os,omitempty" bson:"os,omitempty"`
	Browser        string `json:"browser,omitempty" bson:"browser,omitempty"`
	BrowserVersion string `json:"browser_version,omitempty" bson:"browser_version,omitempty"`
	Ip             string `json:"ip,omitempty" bson:"ip,omitempty"`
	Region         string `json:"region,omitempty" bson:"region,omitempty"`
}
