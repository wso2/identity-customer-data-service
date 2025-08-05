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

// ProfileConsent represents the consent information associated with a profile
type ProfileConsent struct {
	Consents []ConsentRecord `json:"consents" bson:"consents"`
}

// ConsentRecord represents an individual consent record for a profile
type ConsentRecord struct {
	CategoryIdentifier string `json:"category_identifier" bson:"category_identifier"` // References the consent category
	IsConsented        bool   `json:"is_consented" bson:"is_consented"`               // Whether the user has given consent
	ConsentedAt        int64  `json:"consented_at" bson:"consented_at"`               // Timestamp when consent was given/updated
}

// ProfileConsentResponse is the response model for profile consents API
type ProfileConsentResponse struct {
	Consents []ConsentRecord `json:"consents"`
}
