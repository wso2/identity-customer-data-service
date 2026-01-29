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
	defaultCount = 5
	maxCount     = 200
)

func ParseCount(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("count")
	if raw == "" {
		raw = r.URL.Query().Get("limit")
	}

	if raw == "" {
		return defaultCount, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid count")
	}

	// RFC9865 allows 0 (return empty page)
	if v == 0 {
		return 0, nil
	}

	if v > maxCount {
		v = maxCount
	}
	return v, nil
}
