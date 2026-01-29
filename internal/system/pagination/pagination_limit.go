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
	"fmt"
	"net/http"
	"strconv"
)

const (
	defaultLimit = 5
	maxLimit     = 200
)

func ParseLimit(r *http.Request) (int, error) {
	limit := defaultLimit

	if l := r.URL.Query().Get("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil || v <= 0 {
			return 0, fmt.Errorf("invalid limit")
		}
		if v > maxLimit {
			v = maxLimit
		}
		limit = v
	}

	return limit, nil
}

func StrPtr(s string) *string { return &s }
