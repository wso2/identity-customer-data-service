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
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/normalization"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

type KeyLookupFunc func(orgHandle, attributeName string, keyValues []string, excludeProfileID string, maxResults int) ([]string, error)

func FindCandidatesByIndex(
	keys []model.BlockingKey,
	orgHandle string,
	excludeProfileID string,
	lookupFn KeyLookupFunc,
) []string {
	logger := log.GetLogger()

	grouped := make(map[string][]string)
	for _, k := range keys {
		grouped[k.AttributeName] = append(grouped[k.AttributeName], k.KeyValue)
	}

	seen := make(map[string]bool)
	var candidateIDs []string

	for attrName, keyValues := range grouped {
		ids, err := lookupFn(orgHandle, attrName, keyValues, excludeProfileID, constants.MaxCandidatesPerRule)
		if err != nil {
			logger.Error(fmt.Sprintf("Blocker: query failed for attribute '%s'", attrName), log.Error(err))
			continue
		}
		if ids == nil {
			logger.Info(fmt.Sprintf("Blocker: skipping attribute '%s' — too many candidates (>%d)",
				attrName, constants.MaxCandidatesPerRule))
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

func GenerateBlockingKeys(attrType string, attrName string, value string, valueType string) []model.BlockingKey {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var keys []model.BlockingKey

	switch attrType {
	case constants.AttributeTypeEmail:
		norm := normalization.NormalizeEmail(value)
		if norm != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
		}

	case constants.AttributeTypePhone:
		digits := normalization.NormalizePhone(value)
		if digits != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: digits})
			// Add last-7 suffix for country-code-agnostic matching
			if len(digits) >= 7 {
				suffix := digits[len(digits)-7:]
				if suffix != digits {
					keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: suffix})
				}
			}
		}

	case constants.AttributeTypeName:
		tokenSorted := normalization.TokenSortName(value)
		if tokenSorted != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: tokenSorted})
		}
		priPhonetic, altPhonetic := algorithms.DoubleMetaphonePhrase(value)
		if priPhonetic != "" && priPhonetic != tokenSorted {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: priPhonetic})
		}
		if altPhonetic != "" && altPhonetic != priPhonetic && altPhonetic != tokenSorted {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: altPhonetic})
		}
		for _, lshKey := range algorithms.LSHBandHashes(tokenSorted) {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
		}

	case constants.AttributeTypeDate:
		norm := normalization.NormalizeDOB(value)
		if norm != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
		}

	case constants.AttributeTypeGender:
		norm := normalization.NormalizeGender(value)
		if norm != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
		}

	case constants.AttributeTypeLocation:
		norm := strings.TrimSpace(strings.ToLower(value))
		if norm != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			for _, lshKey := range algorithms.LSHBandHashes(norm) {
				keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
			}
		}

	default:
		norm := strings.TrimSpace(strings.ToLower(value))
		if norm != "" {
			keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: norm})
			if valueType == constants.StringDataType {
				for _, lshKey := range algorithms.LSHBandHashes(norm) {
					keys = append(keys, model.BlockingKey{AttributeName: attrName, KeyValue: lshKey})
				}
			}
		}
	}

	return keys
}

func GenerateProfileBlockingKeys(rules []*model.EnrichedRule, flatAttrs map[string]interface{}) []model.BlockingKey {
	var allKeys []model.BlockingKey
	for _, rule := range rules {
		val := getStringValue(flatAttrs, rule.PropertyName)
		if val == "" {
			continue
		}
		keys := GenerateBlockingKeys(rule.AttrType, rule.PropertyName, val, rule.ValueType)
		allKeys = append(allKeys, keys...)
	}
	return allKeys
}

func getStringValue(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}
