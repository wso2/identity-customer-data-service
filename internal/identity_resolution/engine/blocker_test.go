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
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// TestFindCandidatesByIndexIsolatesFuzzyBucketSaturation guards the regression where a
// crowded fuzzy bucket took the exact-match key down with it. Both key kinds used to share
// one query, so an over-populated LSH band or phonetic code pushed the result past the cap
// and the whole attribute returned nothing — silently losing exact duplicates.
func TestFindCandidatesByIndexIsolatesFuzzyBucketSaturation(t *testing.T) {
	if err := log.Init("error"); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	keys := []model.BlockingKey{
		{AttributeName: "traits.name", KeyValue: "john smith"},
		{AttributeName: "traits.name", KeyValue: "lsh:deadbeef", IsFuzzy: true},
		{AttributeName: "traits.name", KeyValue: "JN SM0", IsFuzzy: true},
	}

	var exactQueries, fuzzyQueries int
	lookup := func(_, _ string, keyValues []string, _ string, maxResults int) ([]string, error) {
		// The exact key is queried on its own; the fuzzy keys share the other query.
		if len(keyValues) == 1 && keyValues[0] == "john smith" {
			exactQueries++
			return []string{"profile-exact"}, nil
		}
		fuzzyQueries++
		// Saturated bucket: the store returns nil to signal "too common to be evidence".
		return nil, nil
	}

	got := FindCandidatesByIndex(keys, "acme", "self", lookup)

	if exactQueries != 1 || fuzzyQueries != 1 {
		t.Fatalf("expected exact and fuzzy keys queried separately, got exact=%d fuzzy=%d",
			exactQueries, fuzzyQueries)
	}
	if len(got) != 1 || got[0] != "profile-exact" {
		t.Errorf("saturated fuzzy bucket suppressed the exact match: got %v", got)
	}
}

func TestFindCandidatesByIndexDeduplicatesAcrossGroups(t *testing.T) {
	if err := log.Init("error"); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	keys := []model.BlockingKey{
		{AttributeName: "traits.name", KeyValue: "john smith"},
		{AttributeName: "traits.name", KeyValue: "lsh:beef", IsFuzzy: true},
		{AttributeName: "identity_attributes.email", KeyValue: "j@acme.com"},
	}
	lookup := func(_, _ string, _ []string, _ string, _ int) ([]string, error) {
		return []string{"p1", "p2"}, nil
	}

	got := FindCandidatesByIndex(keys, "acme", "self", lookup)
	if len(got) != 2 {
		t.Errorf("expected 2 deduplicated candidates, got %v", got)
	}
}

// TestExactBlockingKeyAgreesWithMatcher is the property that makes blocking correct: any
// pair the matcher would score 1.0 must share an exact blocking key, or the two profiles
// are never compared in the first place.
func TestExactBlockingKeyAgreesWithMatcher(t *testing.T) {
	tests := []struct {
		name     string
		attrType string
		a, b     string
	}{
		{"date formats", constants.AttributeTypeDate, "2026-01-02", "Jan 2, 2026"},
		{"date separators", constants.AttributeTypeDate, "2026-01-02", "2026/01/02"},
		{"unique id case", constants.AttributeTypeUniqueID, "ABC123", "abc123"},
		{"unique id padding", constants.AttributeTypeUniqueID, " ABC123 ", "abc123"},
		{"email case", constants.AttributeTypeEmail, "J.Smith@Acme.com", "j.smith@acme.com"},
		{"name token order", constants.AttributeTypeName, "John Smith", "Smith John"},
		{"phone formatting", constants.AttributeTypePhone, "+94 77 123 4567", "94771234567"},
		{"phone punctuation", constants.AttributeTypePhone, "(077) 123-4567", "0771234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, verdict := MatchAttribute(tt.a, tt.b, tt.attrType, constants.UnificationModeSmart)
			if verdict == model.VerdictUnknown || score < 1.0 {
				t.Fatalf("precondition: matcher should treat these as identical, got %.4f/%s", score, verdict)
			}

			keyA := ExactBlockingKey(tt.attrType, tt.a)
			keyB := ExactBlockingKey(tt.attrType, tt.b)
			if keyA != keyB {
				t.Errorf("matcher scores 1.0 but blocking keys differ: %q vs %q", keyA, keyB)
			}
		})
	}
}

// TestGenerateBlockingKeysRespectsUnificationMethod checks that a deterministic rule does
// not pay for recall-widening keys it can never score on.
func TestGenerateBlockingKeysRespectsUnificationMethod(t *testing.T) {
	fuzzyKeys := GenerateBlockingKeys(constants.AttributeTypeName, constants.UnificationMethodFuzzy,
		"traits.name", "Jonathan Smith")
	exactKeys := GenerateBlockingKeys(constants.AttributeTypeName, constants.UnificationMethodDeterministic,
		"traits.name", "Jonathan Smith")

	if len(exactKeys) != 1 {
		t.Errorf("deterministic rule should emit only the exact key, got %d: %v", len(exactKeys), exactKeys)
	}
	if exactKeys[0].IsFuzzy {
		t.Errorf("the sole deterministic key must not be marked fuzzy")
	}
	if len(fuzzyKeys) <= 1 {
		t.Errorf("fuzzy rule should emit recall-widening keys, got %d", len(fuzzyKeys))
	}

	var taggedFuzzy int
	for _, k := range fuzzyKeys {
		if k.IsFuzzy {
			taggedFuzzy++
		}
	}
	if taggedFuzzy != len(fuzzyKeys)-1 {
		t.Errorf("exactly one key should be the exact key; %d of %d tagged fuzzy",
			taggedFuzzy, len(fuzzyKeys))
	}
}

func TestGenerateBlockingKeysSkipsEmptyValues(t *testing.T) {
	for _, value := range []string{"", "   ", "\t"} {
		if keys := GenerateBlockingKeys(constants.AttributeTypeEmail, constants.UnificationMethodFuzzy,
			"identity_attributes.email", value); keys != nil {
			t.Errorf("expected no keys for %q, got %v", value, keys)
		}
	}
}

// TestNormalizePhoneKeepsInternationalPrefix documents a known limitation rather than
// asserting desired behaviour: "+94..." and "0094..." are the same number, but only "+" is
// stripped, so the two normalize differently and reach the matcher as a suffix match (0.9)
// instead of an exact one. That still clears the agreement bar, so it does not lose the
// match — but the pair is scored as weaker evidence than it is, and the exact blocking
// keys differ. Update this test when prefix normalization lands.
func TestNormalizePhoneKeepsInternationalPrefix(t *testing.T) {
	plus := ExactBlockingKey(constants.AttributeTypePhone, "+94 77 123 4567")
	zeros := ExactBlockingKey(constants.AttributeTypePhone, "0094771234567")

	if plus == zeros {
		t.Fatalf("international prefixes now normalize alike (%q) — fold this case into "+
			"TestExactBlockingKeyAgreesWithMatcher and delete this test", plus)
	}

	score, verdict := MatchAttribute("+94 77 123 4567", "0094771234567",
		constants.AttributeTypePhone, constants.UnificationModeSmart)
	if verdict == model.VerdictUnknown {
		t.Fatalf("expected a comparable pair, got %s", verdict)
	}
	if score != constants.PhoneSuffixMatchScore {
		t.Errorf("expected the suffix fallback score %.2f, got %.4f",
			constants.PhoneSuffixMatchScore, score)
	}
}
