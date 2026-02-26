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

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	urModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

func DetectAttributeType(propertyName string) string {
	full := strings.ToLower(strings.TrimSpace(propertyName))

	leaf := full
	if idx := strings.LastIndex(full, "."); idx >= 0 {
		leaf = full[idx+1:]
	}

	switch {
	case leaf == "name" || leaf == "full_name" || leaf == "first_name" ||
		leaf == "last_name" || leaf == "fullname" || leaf == "display_name" ||
		leaf == "displayname" || leaf == "givenname" || leaf == "lastname" ||
		leaf == "middlename" || leaf == "firstname" || leaf == "nickname" || leaf == "familyname":
		return constants.AttributeTypeName
	case leaf == "email" || leaf == "emailaddress" || leaf == "emailaddresses" ||
		strings.HasSuffix(leaf, "_email"):
		return constants.AttributeTypeEmail
	case leaf == "phone" || leaf == "mobile" || leaf == "phone_number" ||
		leaf == "phonenumber" || leaf == "telephone" || leaf == "mobilenumber" ||
		leaf == "mobilenumbers":
		return constants.AttributeTypePhone
	case leaf == "dob" || leaf == "date_of_birth" || leaf == "dateofbirth" || leaf == "birthday":
		return constants.AttributeTypeDate
	case leaf == "gender" || leaf == "sex":
		return constants.AttributeTypeGender
	case leaf == "national_id" || leaf == "ssn" || leaf == "nic" ||
		leaf == "passport" || leaf == "nationalid":
		return constants.AttributeTypeID
	case leaf == "location" || leaf == "address" || leaf == "city" || leaf == "country" ||
		leaf == "region" || leaf == "locality" || leaf == "postalcode" || leaf == "postal_code" ||
		leaf == "streetaddress":
		return constants.AttributeTypeLocation
	default:
		return constants.AttributeTypeUnknown
	}
}

var (
	// Email: user@domain.tld — very high specificity.
	reEmail = regexp.MustCompile(
		`(?i)^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// Date — ISO 8601: 2024-01-15 or 2024-01-15T10:30:00
	reDateISO = regexp.MustCompile(
		`^\d{4}[-/]\d{1,2}[-/]\d{1,2}([ T]\d{2}:\d{2}(:\d{2})?)?$`)

	// Date — US/EU slash/dash/dot: 01/15/2024, 15-01-2024, 15.01.24
	reDateSlash = regexp.MustCompile(
		`^\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}$`)

	// SSN: 123-45-6789 or 123456789
	reSSN = regexp.MustCompile(
		`^\d{3}-?\d{2}-?\d{4}$`)

	// Passport: 1-2 letters followed by 6-9 digits (covers most countries).
	rePassport = regexp.MustCompile(
		`(?i)^[a-z]{1,2}\d{6,9}$`)

	// NIC (e.g., Sri Lanka): 9 digits + V/X or 12 digits
	reNIC = regexp.MustCompile(
		`(?i)^(\d{9}[vx]|\d{12})$`)

	// US Zip: 12345 or 12345-6789
	reZipUS = regexp.MustCompile(
		`^\d{5}(-\d{4})?$`)

	// UK Postcode: A9 9AA, A99 9AA, A9A 9AA, etc.
	reZipUK = regexp.MustCompile(
		`(?i)^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$`)

	// Canada Postal Code: A1A 1A1
	reZipCA = regexp.MustCompile(
		`(?i)^[A-Z]\d[A-Z]\s?\d[A-Z]\d$`)
)

type contentMatcher struct {
	attrType string
	match    func(string) bool
}

var contentMatchers = []contentMatcher{
	{constants.AttributeTypeEmail, func(v string) bool { return reEmail.MatchString(v) }},
	{constants.AttributeTypeDate, func(v string) bool { return reDateISO.MatchString(v) || reDateSlash.MatchString(v) }},
	{constants.AttributeTypeID, func(v string) bool { return reSSN.MatchString(v) || rePassport.MatchString(v) || reNIC.MatchString(v) }},
	{constants.AttributeTypeLocation, func(v string) bool { return reZipUS.MatchString(v) || reZipUK.MatchString(v) || reZipCA.MatchString(v) }},
}

func InferTypeFromValues(values []string) string {
	samples := make([]string, 0, constants.MaxSampleSize)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			samples = append(samples, v)
			if len(samples) >= constants.MaxSampleSize {
				break
			}
		}
	}
	if len(samples) == 0 {
		return constants.AttributeTypeUnknown
	}

	total := float64(len(samples))

	for _, cm := range contentMatchers {
		hits := 0
		for _, v := range samples {
			if cm.match(v) {
				hits++
			}
		}
		ratio := float64(hits) / total
		if ratio >= constants.MinContentMatchRatio {
			return cm.attrType
		}
	}

	return constants.AttributeTypeUnknown
}

type ValueSampler func(orgHandle, propertyName string, limit int) ([]string, error)

type EnrichedRule struct {
	RuleID       string
	RuleName     string
	PropertyName string
	PropertyID   string
	Priority     int
	IsActive     bool
	AttrType     string
	ValueType    string
	MinScore     float64
}

func DefaultMinScore(attrType string) float64 {
	switch attrType {
	case constants.AttributeTypeEmail:
		return 0.90
	case constants.AttributeTypePhone:
		return 0.80
	case constants.AttributeTypeID:
		return 0.95
	case constants.AttributeTypeName:
		return 0.70
	case constants.AttributeTypeLocation:
		return 0.55
	case constants.AttributeTypeDate:
		return 0.95
	case constants.AttributeTypeGender:
		return 0.95
	default:
		return 0.50
	}
}

func EnrichRules(rules []urModel.UnificationRule) []*EnrichedRule {
	enriched := make([]*EnrichedRule, 0, len(rules))
	for _, r := range rules {
		if !r.IsActive {
			continue
		}
		attrType := DetectAttributeType(r.PropertyName)
		enriched = append(enriched, &EnrichedRule{
			RuleID:       r.RuleId,
			RuleName:     r.RuleName,
			PropertyName: r.PropertyName,
			PropertyID:   r.PropertyId,
			Priority:     r.Priority,
			IsActive:     true,
			AttrType:     attrType,
			MinScore:     DefaultMinScore(attrType),
		})
	}
	sort.Slice(enriched, func(i, j int) bool {
		return enriched[i].Priority < enriched[j].Priority
	})
	return enriched
}

type SchemaLookup func(orgHandle string) (map[string]string, error)

func EnrichRulesWithSampling(rules []urModel.UnificationRule, orgHandle string, sampler ValueSampler, schemaLookup SchemaLookup) []*EnrichedRule {
	logger := log.GetLogger()

	enriched := EnrichRules(rules)

	if sampler != nil {

		for _, r := range enriched {
			if r.AttrType != constants.AttributeTypeUnknown {
				continue
			}

			values, err := sampler(orgHandle, r.PropertyName, constants.MaxSampleSize)
			if err != nil {
				logger.Warn(fmt.Sprintf("EnrichRulesWithSampling: sampling failed for '%s': %v", r.PropertyName, err))
				continue
			}
			if len(values) == 0 {
				continue
			}

			inferred := InferTypeFromValues(values)
			if inferred != constants.AttributeTypeUnknown {
				logger.Info(fmt.Sprintf("EnrichRulesWithSampling: Layer 2 inferred '%s' → %s (from %d samples)",
					r.PropertyName, inferred, len(values)))
				r.AttrType = inferred
				r.MinScore = DefaultMinScore(inferred)
			}
		}
	}

	if schemaLookup != nil {
		needsLayer3 := false
		for _, r := range enriched {
			if r.AttrType == constants.AttributeTypeUnknown && r.PropertyID != "" {
				needsLayer3 = true
				break
			}
		}

		if needsLayer3 {
			schemaMap, err := schemaLookup(orgHandle)
			if err != nil {
				logger.Warn(fmt.Sprintf("EnrichRulesWithSampling: schema lookup failed: %v", err))
			} else {
				for _, r := range enriched {
					if r.AttrType != constants.AttributeTypeUnknown || r.PropertyID == "" {
						continue
					}

					valueType, ok := schemaMap[r.PropertyID]
					if !ok {
						continue
					}

					r.ValueType = valueType

					switch valueType {
					case constants.DateDataType, constants.DateTimeDataType:
						r.AttrType = constants.AttributeTypeDate
						r.MinScore = DefaultMinScore(constants.AttributeTypeDate)
						logger.Info(fmt.Sprintf("EnrichRulesWithSampling: Layer 3 reclassified '%s' → DATE (value_type=%s)",
							r.PropertyName, valueType))
					case constants.StringDataType:
						logger.Info(fmt.Sprintf("EnrichRulesWithSampling: Layer 3 set '%s' → fuzzy string (value_type=string)",
							r.PropertyName))
					case constants.BooleanDataType, constants.IntegerDataType, constants.EpochDataType:
						logger.Info(fmt.Sprintf("EnrichRulesWithSampling: Layer 3 confirmed '%s' → exact match (value_type=%s)",
							r.PropertyName, valueType))
					case constants.ComplexDataType:
						logger.Info(fmt.Sprintf("EnrichRulesWithSampling: Layer 3 skipping complex '%s'", r.PropertyName))
					default:
						logger.Debug(fmt.Sprintf("EnrichRulesWithSampling: Layer 3 unrecognized value_type '%s' for '%s'",
							valueType, r.PropertyName))
					}
				}
			}
		}
	}

	return enriched
}
