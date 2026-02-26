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

var dobFormats = []string{
	"2006-01-02",
	"01/02/2006",
	"02/01/2006",
	"2006/01/02",
	"Jan 2, 2006",
	"January 2, 2006",
	"02-Jan-2006",
	"2006-01-02T15:04:05Z07:00",
}

func NormalizeDOB(dob string) string {
	dob = strings.TrimSpace(dob)
	if dob == "" {
		return ""
	}
	for _, format := range dobFormats {
		if t, err := time.Parse(format, dob); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return dob
}

func NormalizeGender(gender string) string {
	g := strings.TrimSpace(strings.ToLower(gender))
	switch g {
	case "m", "male", "man":
		return "M"
	case "f", "female", "woman":
		return "F"
	case "o", "other", "non-binary", "nonbinary":
		return "O"
	default:
		return g
	}
}
