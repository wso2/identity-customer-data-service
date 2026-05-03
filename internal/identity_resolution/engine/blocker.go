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

package engine

import (
	"fmt"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine/algorithms"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine/normalization"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	urModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

type CandidateLookupFunc func(orgHandle, attributeName string, keyValues []string, excludeProfileID string, maxResults int) ([]string, error)

func FindCandidatesByIndex(
	keys []model.BlockingKey,
	orgHandle string,
	excludeProfileID string,
	candidateLookup CandidateLookupFunc,
) []string {
	logger := log.GetLogger()

	grouped := make(map[string][]string)
	for _, k := range keys {
		grouped[k.AttributeName] = append(grouped[k.AttributeName], k.KeyValue)
	}

	seen := make(map[string]bool)
	var candidateIDs []string

	for attrName, keyValues := range grouped {
		ids, err := candidateLookup(orgHandle, attrName, keyValues, excludeProfileID, constants.MaxCandidatesPerRule)
		if err != nil {
			logger.Error(fmt.Sprintf("Blocker: query failed for attribute '%s'", attrName), log.Error(err))
			continue
		}
		if ids == nil {
			continue
		}

		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				candidateIDs = append(candidateIDs, id)
			}
		}

		logger.Info(fmt.Sprintf("Blocker: attribute '%s' found %d candidates", attrName, len(ids)))
	}

	logger.Info(fmt.Sprintf("Blocker: total unique candidates: %d", len(candidateIDs)))
	return candidateIDs
}

func GenerateBlockingKeys(attrType string, attrName string, value string) []model.BlockingKey {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var keys []model.BlockingKey
	isFuzzyCapable := constants.FuzzyCapableAttributeTypes[attrType]

	if isFuzzyCapable {
		// Fuzzy-capable types: normalize for blocking (matches normalization in fuzzy matching).
		norm := normalization.NormalizeForType(value, attrType)
		if norm == "" {
			return nil
		}

		switch attrType {
		case constants.AttributeTypeEmail:
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			for _, lshKey := range algorithms.LSHBandHashes(norm) {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
			}

		case constants.AttributeTypePhone:
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			// Grab the end of the phone number so we can match users even if they typed a different country code.
			// We limit the length to prevent too many unrelated people from landing in the same search bucket.
			if len(norm) >= constants.PhoneSuffixBlockingLength {
				suffix := norm[len(norm)-constants.PhoneSuffixBlockingLength:]
				if suffix != norm {
					keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: suffix})
				}
			}

		case constants.AttributeTypeName:
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			priPhonetic, altPhonetic := algorithms.DoubleMetaphonePhrase(value)
			if priPhonetic != "" && priPhonetic != norm {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: priPhonetic})
			}
			if altPhonetic != "" && altPhonetic != priPhonetic && altPhonetic != norm {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: altPhonetic})
			}
			for _, lshKey := range algorithms.LSHBandHashes(norm) {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
			}

		case constants.AttributeTypeLocation:
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			for _, lshKey := range algorithms.LSHBandHashes(norm) {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
			}

		case constants.AttributeTypeFuzzyString:
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			for _, lshKey := range algorithms.LSHBandHashes(norm) {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
			}
		}
	} else {
		// Exact-only types: use raw value as blocking key (no normalization).
		keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: value})
	}

	return keys
}

// GenerateBlockingKeysFromRules generates blocking keys only for attributes that have active unification rules.
// For multi-value attributes (arrays) it generates one set of blocking keys per element so that each
// individual value is independently searchable during candidate search.
func GenerateBlockingKeysFromRules(flatAttrs map[string]interface{}, rules []urModel.UnificationRule) []model.BlockingKey {
	var allKeys []model.BlockingKey
	for _, rule := range rules {
		for _, strVal := range getStringValues(flatAttrs, rule.PropertyName) {
			keys := GenerateBlockingKeys(rule.AttributeType, rule.PropertyName, strVal)
			allKeys = append(allKeys, keys...)
		}
	}
	return allKeys
}

func getStringValues(attrs map[string]interface{}, key string) []string {
	if attrs == nil {
		return nil
	}
	v, ok := attrs[key]
	if !ok || v == nil {
		return nil
	}
	switch typed := v.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []interface{}:
		var result []string
		for _, elem := range typed {
			if s, ok := elem.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	case []string:
		var result []string
		for _, s := range typed {
			if s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		s := fmt.Sprintf("%v", v)
		if s == "" {
			return nil
		}
		return []string{s}
	}
}
