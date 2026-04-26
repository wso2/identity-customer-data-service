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

package normalization

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

var multiSpace = regexp.MustCompile(`\s+`)

func NormalizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = multiSpace.ReplaceAllString(name, " ")
	// Remove non-letter, non-space characters
	var cleaned []rune
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			cleaned = append(cleaned, r)
		}
	}
	return strings.TrimSpace(string(cleaned))
}

func TokenSortName(name string) string {
	normalized := NormalizeName(name)
	tokens := strings.Fields(normalized)
	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func NormalizePhone(phone string) string {
	var digits []rune
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	return string(digits)
}

var dateFormats = []string{
	"2006-01-02",
	"01/02/2006",
	"02/01/2006",
	"2006/01/02",
	"Jan 2, 2006",
	"January 2, 2006",
	"02-Jan-2006",
	"2006-01-02T15:04:05Z07:00",
}

func NormalizeDate(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return ""
	}
	for _, format := range dateFormats {
		if t, err := time.Parse(format, date); err == nil {
			return t.Format("2006.01.02")
		}
	}
	return date
}

func NormalizeForType(value string, attrType string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	switch attrType {
	case constants.AttributeTypeName:
		return TokenSortName(value)
	case constants.AttributeTypeEmail:
		return NormalizeEmail(value)
	case constants.AttributeTypePhone:
		return NormalizePhone(value)
	case constants.AttributeTypeDate:
		return NormalizeDate(value)
	case constants.AttributeTypeLocation:
		return strings.TrimSpace(strings.ToLower(value))
	case constants.AttributeTypeFuzzyString:
		return strings.TrimSpace(strings.ToLower(value))
	case constants.AttributeTypeUniqueID:
		return strings.TrimSpace(strings.ToLower(value))
	case constants.AttributeTypePrimitiveExact:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}
