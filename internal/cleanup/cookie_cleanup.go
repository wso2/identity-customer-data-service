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

package cleanup

import (
	"fmt"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func StartCookieCleanup(interval time.Duration, batchSize int, done <-chan struct{}) {

	ticker := time.NewTicker(interval)
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Cookie profile cleanup scheduled every %s", interval))

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				total := 0
				for {
					deleted, err := store.DeleteInactiveCookieProfiles(batchSize)
					if err != nil {
						logger.Debug("Cookie cleanup error", log.Error(err))
						break
					}
					total += deleted
					if deleted < batchSize {
						break
					}
				}
				if total > 0 {
					logger.Info(fmt.Sprintf("Cookie cleanup: purged %d inactive records", total))
				}
			case <-done:
				logger.Info("Cookie profile cleanup stopped")
				return
			}
		}
	}()
}
