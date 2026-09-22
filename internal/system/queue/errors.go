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

package queue

import "errors"

// ErrDeferred leaves a message unacknowledged when its worker is stopping.
var ErrDeferred = errors.New("queue: processing deferred until a later delivery")

type retryableError struct{ error }

func (e *retryableError) Unwrap() error { return e.error }

// Retryable marks an operation that can safely be repeated after failure.
// Unknown errors are terminal: retry safety must be decided by the worker.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{err}
}

// IsRetryable reports whether the worker explicitly permits another attempt.
func IsRetryable(err error) bool {
	var retryable *retryableError
	return errors.As(err, &retryable)
}
