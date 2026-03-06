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

package workers

import (
	"fmt"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

var cookieCleanupDone chan struct{}

func StartCookieCleanupWorker(cfg config.CookieCleanupConfig) {

	logger := log.GetLogger()

	if cfg.Interval <= 0 {
		cfg.Interval = constants.DefaultCookieCleanupTime
		logger.Info("Cookie cleanup interval not set or invalid. Defaulting to 24 hours.")
	}

	interval := time.Duration(cfg.Interval) * time.Second
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	cookieCleanupDone = make(chan struct{})

	logger.Info(fmt.Sprintf("Cookie cleanup worker started. Interval: %s, Batch size: %d",
		interval, batchSize))

	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runCookieCleanup(batchSize)
			case <-cookieCleanupDone:
				logger.Info("Cookie cleanup worker stopped")
				return
			}
		}
	}()
}

func StopCookieCleanupWorker() {
	if cookieCleanupDone != nil {
		close(cookieCleanupDone)
	}
}

func runCookieCleanup(batchSize int) {

	logger := log.GetLogger()
	total := 0

	for {
		deleted, err := store.DeleteInactiveCookieProfiles(batchSize)
		if err != nil {
			logger.Debug("Cookie cleanup batch error", log.Error(err))
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
}
