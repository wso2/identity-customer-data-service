/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package lock

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"hash/fnv" // For hashing string keys to integers
)

type DistributedLock interface {
	Acquire(key string) (bool, error)
	Release(key string) error
}

// PostgresLock implements DistributedLock using PostgreSQL advisory locks.
type PostgresLock struct{}

func NewPostgresLock() *PostgresLock {
	return &PostgresLock{}
}

// PostgreSQL advisory locks use bigint or two integers. We'll use a single bigint.
func (l *PostgresLock) generateLockKey(key string) (int64, error) {

	logger := log.GetLogger()
	h := fnv.New64a() // FNV-1a is a good general-purpose non-cryptographic hash
	_, err := h.Write([]byte(key))
	if err != nil {
		errorMsg := fmt.Sprintf("failed to hash lock key '%s'", key)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_KEY_GEN.Code,
			Message:     errors.LOCK_KEY_GEN.Message,
			Description: errorMsg,
		}, err)
		return 0, serverError
	}
	return int64(h.Sum64()), nil // Cast to int64 for pg_advisory_lock
}

func (l *PostgresLock) Acquire(key string) (bool, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed during DB client creation for advisory lock acquiring."
		logger.Error(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return false, serverError
	}
	defer dbClient.Close()
	lockID, err := l.generateLockKey(key)
	if err != nil {
		errorMsg := "Could not create advisory lock key from input."
		logger.Error(errorMsg, log.Error(err))
		return false, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_KEY_GEN.Code,
			Message:     errors.LOCK_KEY_GEN.Message,
			Description: errorMsg,
		}, err)
	}
	logger.Debug(fmt.Sprintf("Generated lock Id: %s", lockID))

	var acquired bool
	results, err := dbClient.ExecuteQuery("SELECT pg_try_advisory_lock($1)", lockID)
	if err != nil {
		errorMsg := "Failed to execute pg_try_advisory_lock"
		logger.Error(errorMsg, log.Error(err))
		return false, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_ACQUIRE.Code,
			Message:     errors.LOCK_ACQUIRE.Message,
			Description: errorMsg,
		}, err)
	}

	if len(results) == 0 || results[0]["pg_try_advisory_lock"] == nil {
		errorMsg := fmt.Sprintf("pg_try_advisory_lock returned no results or invalid field for "+
			"lock Id %d", lockID)
		logger.Error(errorMsg, log.Error(err))
		return false, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_RESULT_INVALID.Code,
			Message:     errors.LOCK_RESULT_INVALID.Message,
			Description: errorMsg,
		}, err)
	}

	acquired = results[0]["pg_try_advisory_lock"].(bool)
	return acquired, nil
}

func (l *PostgresLock) Release(key string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed during DB client creation for advisory lock releasing."
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()
	lockID, err := l.generateLockKey(key)
	if err != nil {
		errorMsg := "Could not create advisory lock key from input."
		logger.Error(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_KEY_GEN.Code,
			Message:     errors.LOCK_KEY_GEN.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	var released bool
	results, err := dbClient.ExecuteQuery("SELECT pg_advisory_unlock($1)", lockID)
	row := results[0]
	released = row["pg_advisory_unlock"].(bool)
	if err != nil || !released {
		errorMsg := "pg_advisory_unlock failed"
		logger.Error(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.LOCK_RELEASE.Code,
			Message:     errors.LOCK_RELEASE.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Debug(fmt.Sprintf("Advisory lock released for lock id: %s", lockID))
	return nil
}
