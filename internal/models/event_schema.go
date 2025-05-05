package models

type EventSchema struct {
	EventSchemaId string          `json:"event_schema_id" bson:"event_schema_id" bind:"required"`
	EventName     string          `json:"event_name" bson:"event_name"`
	EventType     string          `json:"event_type" bson:"event_type"`
	Properties    []EventProperty `json:"properties,omitempty" bson:"properties,omitempty"`
}

type EventProperty struct {
	PropertyName string `json:"property_name" bson:"property_name" bind:"required"`
	PropertyType string `json:"property_type" bson:"property_type" bind:"required"`
}
