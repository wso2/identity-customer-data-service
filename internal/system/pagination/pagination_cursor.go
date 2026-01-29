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

package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

type ProfileCursor struct {
	UpdatedAt time.Time
	ProfileId string
}

func EncodeProfileCursor(c ProfileCursor) string {
	raw := fmt.Sprintf("%s|%s", c.UpdatedAt.UTC().Format(time.RFC3339Nano), c.ProfileId)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeProfileCursor(s string) (*ProfileCursor, error) {
	if s == "" {
		return nil, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding")
	}

	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor timestamp")
	}

	id := strings.TrimSpace(parts[1])
	if id == "" {
		return nil, fmt.Errorf("invalid cursor profile_id")
	}

	return &ProfileCursor{UpdatedAt: t.UTC(), ProfileId: id}, nil
}
