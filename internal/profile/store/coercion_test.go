/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package store

import (
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// typeCoercionCase describes a single (oldType, newType) pair expectation.
type typeCoercionCase struct {
	old        string
	new        string
	canCoerce  bool
	// wantNoop is true when the data does not actually need to change (epoch↔integer).
	wantNoop   bool
	// wantContains are substrings that must appear in the setExpr when canCoerce=true.
	wantContains []string
}

func TestTypeCoercionExpr(t *testing.T) {
	cases := []typeCoercionCase{
		// ── → string (always succeeds) ──────────────────────────────────────────
		{old: constants.IntegerDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},
		{old: constants.DecimalDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},
		{old: constants.BooleanDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},
		{old: constants.DateDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},
		{old: constants.DateTimeDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},
		{old: constants.EpochDataType, new: constants.StringDataType, canCoerce: true, wantContains: []string{"to_jsonb"}},

		// ── → integer ───────────────────────────────────────────────────────────
		{old: constants.DecimalDataType, new: constants.IntegerDataType, canCoerce: true, wantContains: []string{"floor", "bigint"}},
		{old: constants.EpochDataType, new: constants.IntegerDataType, canCoerce: true, wantNoop: true},
		{old: constants.BooleanDataType, new: constants.IntegerDataType, canCoerce: true, wantContains: []string{"CASE", "true"}},
		{old: constants.StringDataType, new: constants.IntegerDataType, canCoerce: true, wantContains: []string{"CASE", "bigint"}},
		{old: constants.DateDataType, new: constants.IntegerDataType, canCoerce: false},
		{old: constants.DateTimeDataType, new: constants.IntegerDataType, canCoerce: false},

		// ── → decimal ───────────────────────────────────────────────────────────
		{old: constants.IntegerDataType, new: constants.DecimalDataType, canCoerce: true, wantContains: []string{"numeric"}},
		{old: constants.EpochDataType, new: constants.DecimalDataType, canCoerce: true, wantContains: []string{"numeric"}},
		{old: constants.BooleanDataType, new: constants.DecimalDataType, canCoerce: true, wantContains: []string{"CASE", "true"}},
		{old: constants.StringDataType, new: constants.DecimalDataType, canCoerce: true, wantContains: []string{"CASE", "numeric"}},
		{old: constants.DateDataType, new: constants.DecimalDataType, canCoerce: false},
		{old: constants.DateTimeDataType, new: constants.DecimalDataType, canCoerce: false},

		// ── → boolean ───────────────────────────────────────────────────────────
		{old: constants.IntegerDataType, new: constants.BooleanDataType, canCoerce: true, wantContains: []string{"numeric", "!= 0"}},
		{old: constants.DecimalDataType, new: constants.BooleanDataType, canCoerce: true, wantContains: []string{"numeric", "!= 0"}},
		{old: constants.StringDataType, new: constants.BooleanDataType, canCoerce: true, wantContains: []string{"CASE", "true", "false"}},
		{old: constants.DateDataType, new: constants.BooleanDataType, canCoerce: false},
		{old: constants.DateTimeDataType, new: constants.BooleanDataType, canCoerce: false},
		{old: constants.EpochDataType, new: constants.BooleanDataType, canCoerce: false},

		// ── → date_time ─────────────────────────────────────────────────────────
		{old: constants.DateDataType, new: constants.DateTimeDataType, canCoerce: true, wantContains: []string{"::date::timestamptz"}},
		{old: constants.EpochDataType, new: constants.DateTimeDataType, canCoerce: true, wantContains: []string{"to_timestamp"}},
		{old: constants.StringDataType, new: constants.DateTimeDataType, canCoerce: true, wantContains: []string{"CASE", "timestamptz"}},
		{old: constants.IntegerDataType, new: constants.DateTimeDataType, canCoerce: false},
		{old: constants.DecimalDataType, new: constants.DateTimeDataType, canCoerce: false},
		{old: constants.BooleanDataType, new: constants.DateTimeDataType, canCoerce: false},

		// ── → date ──────────────────────────────────────────────────────────────
		{old: constants.DateTimeDataType, new: constants.DateDataType, canCoerce: true, wantContains: []string{"::timestamptz::date"}},
		{old: constants.EpochDataType, new: constants.DateDataType, canCoerce: true, wantContains: []string{"to_timestamp", "::date"}},
		{old: constants.StringDataType, new: constants.DateDataType, canCoerce: true, wantContains: []string{"CASE", "::date"}},
		{old: constants.IntegerDataType, new: constants.DateDataType, canCoerce: false},
		{old: constants.DecimalDataType, new: constants.DateDataType, canCoerce: false},
		{old: constants.BooleanDataType, new: constants.DateDataType, canCoerce: false},

		// ── → epoch ─────────────────────────────────────────────────────────────
		{old: constants.IntegerDataType, new: constants.EpochDataType, canCoerce: true, wantNoop: true},
		{old: constants.DecimalDataType, new: constants.EpochDataType, canCoerce: true, wantContains: []string{"floor", "bigint"}},
		{old: constants.DateTimeDataType, new: constants.EpochDataType, canCoerce: true, wantContains: []string{"extract", "epoch"}},
		{old: constants.DateDataType, new: constants.EpochDataType, canCoerce: true, wantContains: []string{"extract", "epoch"}},
		{old: constants.StringDataType, new: constants.EpochDataType, canCoerce: true, wantContains: []string{"CASE", "bigint"}},
		{old: constants.BooleanDataType, new: constants.EpochDataType, canCoerce: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.old+"_to_"+tc.new, func(t *testing.T) {
			expr, ok := typeCoercionExpr(tc.old, tc.new)

			if ok != tc.canCoerce {
				t.Errorf("typeCoercionExpr(%q, %q): canCoerce = %v, want %v", tc.old, tc.new, ok, tc.canCoerce)
			}
			if !tc.canCoerce {
				if expr != "" {
					t.Errorf("typeCoercionExpr(%q, %q): expected empty expr when canCoerce=false, got %q", tc.old, tc.new, expr)
				}
				return
			}

			if tc.wantNoop {
				if expr != "col = col" {
					t.Errorf("typeCoercionExpr(%q, %q): expected no-op expr, got %q", tc.old, tc.new, expr)
				}
				return
			}

			if expr == "" {
				t.Errorf("typeCoercionExpr(%q, %q): expected non-empty expr", tc.old, tc.new)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(expr, want) {
					t.Errorf("typeCoercionExpr(%q, %q): expr %q does not contain %q", tc.old, tc.new, expr, want)
				}
			}
		})
	}
}

// TestTypeCoercionExpr_UsesPlaceholders verifies that every generated expression
// uses the "col" and "$2" placeholders expected by execProfileAttributeUpdate.
func TestTypeCoercionExpr_UsesPlaceholders(t *testing.T) {
	coerciblePairs := [][2]string{
		{constants.StringDataType, constants.IntegerDataType},
		{constants.IntegerDataType, constants.StringDataType},
		{constants.DecimalDataType, constants.IntegerDataType},
		{constants.BooleanDataType, constants.IntegerDataType},
		{constants.DateDataType, constants.DateTimeDataType},
		{constants.EpochDataType, constants.DateDataType},
	}

	for _, pair := range coerciblePairs {
		expr, ok := typeCoercionExpr(pair[0], pair[1])
		if !ok || expr == "col = col" {
			continue
		}
		if !strings.Contains(expr, "col") {
			t.Errorf("typeCoercionExpr(%q, %q): expr missing 'col' placeholder: %s", pair[0], pair[1], expr)
		}
		if !strings.Contains(expr, "$2") {
			t.Errorf("typeCoercionExpr(%q, %q): expr missing '$2' placeholder: %s", pair[0], pair[1], expr)
		}
	}
}
