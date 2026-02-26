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
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine/algorithms"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/normalization"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func MatchAttribute(val1, val2 string, attrType string, mode string, valueType string) float64 {
	logger := log.GetLogger()

	if val1 == "" || val2 == "" {
		return 0.0
	}

	var score float64

	switch attrType {
	case constants.AttributeTypeName:
		score = matchName(val1, val2, mode)
	case constants.AttributeTypeEmail:
		score = matchEmail(val1, val2, mode)
	case constants.AttributeTypePhone:
		score = matchPhone(val1, val2, mode)
	case constants.AttributeTypeDate:
		score = matchDate(val1, val2)
	case constants.AttributeTypeGender:
		score = matchGender(val1, val2)
	case constants.AttributeTypeID:
		score = matchID(val1, val2)
	case constants.AttributeTypeLocation:
		score = matchLocation(val1, val2, mode)
	default:
		if valueType == constants.StringDataType {
			score = matchFuzzyString(val1, val2, mode)
		} else {
			score = matchExact(val1, val2)
		}
	}

	logger.Debug("Matched attribute",
		log.String("type", attrType),
		log.String("mode", mode),
		log.String("val1", truncate(val1, 30)),
		log.String("val2", truncate(val2, 30)),
		log.Any("score", score))

	return score
}

func matchName(val1, val2 string, mode string) float64 {
	sorted1 := normalization.TokenSortName(val1)
	sorted2 := normalization.TokenSortName(val2)

	if sorted1 == sorted2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}
	jwScore := algorithms.JaroWinkler(sorted1, sorted2)
	phoneticScore := algorithms.PhoneticSimilarity(sorted1, sorted2)

	if phoneticScore >= 1.0 {
		if jwScore < 0.92 {
			return 0.92
		}
		return jwScore
	}

	if phoneticScore >= 0.9 && jwScore >= 0.70 {
		boosted := jwScore + (1.0-jwScore)*0.15
		if boosted > jwScore {
			return boosted
		}
	}

	return jwScore
}

func matchEmail(val1, val2 string, mode string) float64 {
	n1 := normalization.NormalizeEmail(val1)
	n2 := normalization.NormalizeEmail(val2)

	if n1 == n2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}
	similarity := algorithms.LevenshteinSimilarity(n1, n2)
	if similarity < 0.8 {
		return 0.0
	}
	return similarity
}

func matchPhone(val1, val2 string, mode string) float64 {
	n1 := normalization.NormalizePhone(val1)
	n2 := normalization.NormalizePhone(val2)

	if n1 == n2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}
	if len(n1) >= 7 && len(n2) >= 7 {
		suffix1 := n1[len(n1)-7:]
		suffix2 := n2[len(n2)-7:]
		if suffix1 == suffix2 {
			return 0.9
		}
	}
	return 0.0
}

func matchDate(val1, val2 string) float64 {
	n1 := normalization.NormalizeDOB(val1)
	n2 := normalization.NormalizeDOB(val2)
	if n1 == n2 {
		return 1.0
	}
	return 0.0
}

func matchGender(val1, val2 string) float64 {
	n1 := normalization.NormalizeGender(val1)
	n2 := normalization.NormalizeGender(val2)
	if n1 == n2 {
		return 1.0
	}
	return 0.0
}

func matchID(val1, val2 string) float64 {
	n1 := strings.TrimSpace(strings.ToLower(val1))
	n2 := strings.TrimSpace(strings.ToLower(val2))
	if n1 == n2 {
		return 1.0
	}
	return 0.0
}

func matchLocation(val1, val2 string, mode string) float64 {
	n1 := strings.TrimSpace(strings.ToLower(val1))
	n2 := strings.TrimSpace(strings.ToLower(val2))

	if n1 == n2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}
	expanded1 := algorithms.ExpandAddressAbbreviations(n1)
	expanded2 := algorithms.ExpandAddressAbbreviations(n2)
	return algorithms.JaccardSimilarity(expanded1, expanded2)
}

func matchFuzzyString(val1, val2 string, mode string) float64 {
	n1 := strings.TrimSpace(strings.ToLower(val1))
	n2 := strings.TrimSpace(strings.ToLower(val2))

	if n1 == n2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}
	jwScore := algorithms.JaroWinkler(n1, n2)
	if jwScore < constants.FuzzyStringThreshold {
		return 0.0
	}
	return jwScore
}

func matchExact(val1, val2 string) float64 {
	if strings.EqualFold(strings.TrimSpace(val1), strings.TrimSpace(val2)) {
		return 1.0
	}
	return 0.0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
