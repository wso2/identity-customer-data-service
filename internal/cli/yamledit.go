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

package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// setScalar rewrites one scalar value in a YAML document, in place.
//
// The shipped deployment.yaml is a documented file: almost every setting in it
// carries a comment explaining what it is for, and an operator reads it. A
// parse-and-marshal round trip would drop every one of those comments, so
// enablement edits the lines it owns and leaves the rest of the file exactly
// as it found it.
//
// parent is the top-level key the setting sits under, or "" for a top-level
// scalar. Strings are written quoted and numbers and booleans bare, because
// the runtime decodes these settings into typed fields and a quoted number
// would not decode into an int at all.
func setScalar(content, parent, key, value string) (string, error) {

	lines := strings.Split(content, "\n")
	inParent := parent == ""
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		atTopLevel := !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t")

		if parent != "" && atTopLevel {
			// Entering the parent block, or leaving it for the next top-level
			// key. Either way the question is only ever about this one block.
			inParent = strings.HasPrefix(trimmed, parent+":")
			continue
		}
		if !inParent {
			continue
		}
		if parent == "" && !atTopLevel {
			continue
		}

		name, _, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}

		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = fmt.Sprintf("%s%s: %s%s", indent, key, yamlScalar(value), trailingComment(line))
		found = true
		break
	}

	if !found {
		return "", fmt.Errorf("no %s setting to update", settingPath(parent, key))
	}
	return strings.Join(lines, "\n"), nil
}

// trailingComment preserves an end-of-line comment on a rewritten setting, so
// the note explaining a value survives the value changing.
func trailingComment(line string) string {

	_, after, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	index := strings.Index(after, "#")
	if index < 0 {
		return ""
	}
	return " " + strings.TrimSpace(after[index:])
}

// yamlScalar renders a value the way the runtime's decoder expects to read it
// back.
func yamlScalar(value string) string {

	switch value {
	case "true", "false", "null", "~":
		return value
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return value
	}
	return strconv.Quote(value)
}

func settingPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}
