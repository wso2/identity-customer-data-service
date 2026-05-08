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
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine/normalization"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

func MatchAttribute(val1, val2 string, attrType string, mode string) float64 {

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
	case constants.AttributeTypeUniqueID:
		score = matchID(val1, val2)
	case constants.AttributeTypeLocation:
		score = matchLocation(val1, val2, mode)
	case constants.AttributeTypeFuzzyString:
		score = matchFuzzyString(val1, val2, mode)
	case constants.AttributeTypePrimitiveExact:
		score = matchExact(val1, val2)
	default:
		score = matchExact(val1, val2)
	}

	return score
}

func matchName(val1, val2 string, mode string) float64 {
	// Raw exact check first (no normalization).
	if val1 == val2 {
		return 1.0
	}
	// If values are not exactly the same, strict mode returns 0 immediately without further processing.
	if mode == constants.UnificationModeStrict {
		return 0.0
	}

	// Fuzzy mode: normalize then apply algorithms.
	sorted1 := normalization.TokenSortName(val1)
	sorted2 := normalization.TokenSortName(val2)
	if sorted1 == sorted2 {
		return 1.0
	}

	jwScore := algorithms.JaroWinkler(sorted1, sorted2)
	phoneticScore := algorithms.PhoneticSimilarity(sorted1, sorted2)

	if phoneticScore >= 1.0 {
		if jwScore < constants.NamePhoneticExactJWMin {
			return constants.NamePhoneticExactJWMin
		}
		return jwScore
	}

	return jwScore
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
	if len(n1) >= constants.PhoneSuffixBlockingLength && len(n2) >= constants.PhoneSuffixBlockingLength {
		suffix1 := n1[len(n1)-constants.PhoneSuffixBlockingLength:]
		suffix2 := n2[len(n2)-constants.PhoneSuffixBlockingLength:]
		if suffix1 == suffix2 {
			return constants.PhoneSuffixMatchScore
		}
	}
	return 0.0
}

func matchDate(val1, val2 string) float64 {
	n1 := normalization.NormalizeDate(val1)
	n2 := normalization.NormalizeDate(val2)

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

func matchEmail(val1, val2 string, mode string) float64 {
	// Raw exact check first (no normalization).
	if val1 == val2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}

	// Fuzzy mode: normalize then compare.
	n1 := normalization.NormalizeEmail(val1)
	n2 := normalization.NormalizeEmail(val2)
	if n1 == n2 {
		return 1.0
	}

	local1, domain1, ok1 := strings.Cut(n1, "@")
	local2, domain2, ok2 := strings.Cut(n2, "@")
	if !ok1 || !ok2 {
		// Malformed email (no '@')
		return 0.0
	}

	localSim := algorithms.LevenshteinSimilarity(local1, local2)

	domainSim := 1.0
	if domain1 != domain2 {
		domainSim = algorithms.JaroWinkler(domain1, domain2)
	}

	if localSim < domainSim {
		return localSim
	}
	return domainSim
}

func matchExact(val1, val2 string) float64 {
	if val1 == val2 {
		return 1.0
	}
	return 0.0
}

func matchLocation(val1, val2 string, mode string) float64 {
	// Raw exact check first (no normalization).
	if val1 == val2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}

	// Fuzzy mode: normalize then compare.
	n1 := normalization.NormalizeForType(val1, constants.AttributeTypeLocation)
	n2 := normalization.NormalizeForType(val2, constants.AttributeTypeLocation)
	if n1 == n2 {
		return 1.0
	}
	expanded1 := algorithms.ExpandAddressAbbreviations(n1)
	expanded2 := algorithms.ExpandAddressAbbreviations(n2)
	return algorithms.JaccardSimilarity(expanded1, expanded2)
}

func matchFuzzyString(val1, val2 string, mode string) float64 {
	// Raw exact check first (no normalization).
	if val1 == val2 {
		return 1.0
	}
	if mode == constants.UnificationModeStrict {
		return 0.0
	}

	// Fuzzy mode: normalize then compare.
	n1 := normalization.NormalizeForType(val1, constants.AttributeTypeFuzzyString)
	n2 := normalization.NormalizeForType(val2, constants.AttributeTypeFuzzyString)
	if n1 == n2 {
		return 1.0
	}
	jwScore := algorithms.JaroWinkler(n1, n2)
	return jwScore
}
