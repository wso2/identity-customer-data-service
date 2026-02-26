/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

type SearchRequest struct {
	UserID             string                            `json:"user_id,omitempty"`
	IdentityAttributes map[string]interface{}            `json:"identity_attributes,omitempty"`
	Traits             map[string]interface{}            `json:"traits,omitempty"`
	ApplicationData    map[string]map[string]interface{} `json:"application_data,omitempty"`
	MaxResults         int                               `json:"max_results,omitempty"`
	Threshold          float64                           `json:"threshold,omitempty"`
}

func (r *SearchRequest) FlatAttributes() map[string]interface{} {
	merged := make(map[string]interface{})
	FlattenMap("traits", r.Traits, merged)
	FlattenMap("identity_attributes", r.IdentityAttributes, merged)
	if r.UserID != "" {
		merged["user_id"] = r.UserID
	}
	return merged
}

func FlattenMap(prefix string, m map[string]interface{}, out map[string]interface{}) {
	for k, v := range m {
		fullKey := prefix + "." + k
		if nested, ok := v.(map[string]interface{}); ok {
			FlattenMap(fullKey, nested, out)
		} else {
			out[fullKey] = v
		}
	}
}

func (r *SearchRequest) GetMaxResults() int {
	if r.MaxResults <= 0 {
		return 50
	}
	if r.MaxResults > 100 {
		return 100
	}
	return r.MaxResults
}

func (r *SearchRequest) GetThreshold(defaults Thresholds) float64 {
	if r.Threshold <= 0 {
		return defaults.ManualReview
	}
	if r.Threshold > 1.0 {
		return 1.0
	}
	return r.Threshold
}

type SearchResponse struct {
	Matches         []MatchResult `json:"matches"`
	TotalCandidates int           `json:"total_candidates"`
	ProcessingTime  int64         `json:"processing_time_ms"`
}

type MatchResult struct {
	CandidateID    string                 `json:"candidate_id"`
	UserID         string                 `json:"user_id,omitempty"`
	FinalScore     float64                `json:"final_score"`
	ScoreBreakdown map[string]float64     `json:"score_breakdown"`
	Attributes     map[string]interface{} `json:"attributes"`
}
