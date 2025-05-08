package models

import "encoding/json"

type ProfileHierarchy struct {
	ParentProfileID string         `json:"parent_profile_id,omitempty" bson:"parent_profile_id,omitempty"`
	IsParent        bool           `json:"is_parent,omitempty" bson:"is_parent,omitempty"`
	ListProfile     bool           `json:"list_profile,omitempty" bson:"list_profile,omitempty"`
	ChildProfiles   []ChildProfile `json:"child_profile_ids,omitempty" bson:"child_profile_ids,omitempty"`
}

type ChildProfile struct {
	ChildProfileId string `json:"child_profile_id,omitempty" bson:"child_profile_id,omitempty"`
	RuleName       string `json:"rule_name,omitempty" bson:"rule_name,omitempty"`
}

type Profile struct {
	ProfileId          string                 `json:"profile_id" bson:"profile_id"`
	OriginCountry      string                 `json:"origin_country" bson:"origin_country"`
	IdentityAttributes map[string]interface{} `json:"identity_attributes,omitempty" bson:"identity_attributes,omitempty"`
	Traits             map[string]interface{} `json:"traits,omitempty" bson:"traits,omitempty"`
	ApplicationData    []ApplicationData      `json:"application_data,omitempty" bson:"application_data,omitempty"`
	ProfileHierarchy   *ProfileHierarchy      `json:"profile_hierarchy,omitempty" bson:"profile_hierarchy,omitempty"`
}

type ProfileEnrichmentRule struct {
	RuleId            string      `json:"rule_id,omitempty" bson:"rule_id,omitempty"`
	PropertyName      string      `json:"property_name" bson:"property_name"`
	Description       string      `json:"description,omitempty" bson:"description,omitempty"`
	Value             interface{} `json:"value,omitempty" bson:"value,omitempty"` // required if computation == static
	ValueType         string      `json:"value_type,omitempty" bson:"value_type,omitempty"`
	ComputationMethod string      `json:"computation_method,omitempty" bson:"computation_method,omitempty"` // if trait_type == computed
	SourceField       string      `json:"source_field,omitempty" bson:"source_field,omitempty"`             // For concat
	TimeRange         int64       `json:"time_range,omitempty" bson:"time_range,omitempty"`                 // last x seconds - required for count, sum, avg
	MergeStrategy     string      `json:"merge_strategy" bson:"merge_strategy"`                             // overwrite, combine, ignore
	Trigger           RuleTrigger `json:"trigger" bson:"trigger"`
	CreatedAt         int64       `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt         int64       `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type RuleTrigger struct {
	EventType  string          `json:"event_type" bson:"event_type"`
	EventName  string          `json:"event_name" bson:"event_name"`
	Conditions []RuleCondition `json:"conditions" bson:"conditions"`
}

type RuleCondition struct {
	Field    string `json:"field" bson:"field"`
	Operator string `json:"operator" bson:"operator"`
	Value    string `json:"value" bson:"value"`
}

func (a ApplicationData) MarshalJSON() ([]byte, error) {
	base := map[string]interface{}{
		"application_id": a.AppId,
		"devices":        a.Devices,
	}
	for k, v := range a.AppSpecificData {
		base[k] = v
	}
	return json.Marshal(base)
}
