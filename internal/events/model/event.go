/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package model

type Event struct {
	ProfileId      string                 `json:"profile_id" bson:"profile_id"`
	EventType      string                 `json:"event_type" bson:"event_type"`
	EventName      string                 `json:"event_name" bson:"event_name"`
	EventId        string                 `json:"event_id" bson:"event_id"`
	AppId          string                 `json:"application_id" bson:"application_id"`
	OrgId          string                 `json:"org_id" bson:"org_id"`
	EventTimestamp int                    `json:"event_timestamp" bson:"event_timestamp"`
	Properties     map[string]interface{} `json:"properties,omitempty" bson:"properties,omitempty"`
	Context        map[string]interface{} `json:"context,omitempty" bson:"context,omitempty"`
}
