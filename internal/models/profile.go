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
