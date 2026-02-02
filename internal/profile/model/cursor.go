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
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func EncodeProfileCursor(c ProfileCursor) string {
	dir := strings.TrimSpace(c.Direction)
	if dir == "" {
		dir = "next"
	}

	raw := fmt.Sprintf(
		"%s|%s|%s",
		c.CreatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(c.ProfileId),
		dir,
	)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeProfileCursor(s string) (*ProfileCursor, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding")
	}

	parts := strings.Split(string(b), "|")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid cursor timestamp")
	}

	id := strings.TrimSpace(parts[1])
	if id == "" {
		return nil, fmt.Errorf("invalid cursor profile_id")
	}

	dir := "next"
	if len(parts) == 3 {
		dir = strings.TrimSpace(parts[2])
		if dir == "" {
			dir = "next"
		}
	}
	if dir != "next" && dir != "prev" {
		return nil, fmt.Errorf("invalid cursor direction")
	}

	return &ProfileCursor{
		CreatedAt: t.UTC(),
		ProfileId: id,
		Direction: dir,
	}, nil
}
