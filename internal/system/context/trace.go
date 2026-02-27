/*
 * Copyright (c) 2025-2026, WSO2 LLC. (http://www.wso2.com).
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

package context

import (
	"context"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// GetOrGenerateTraceID extracts the trace ID from the context or generates a new one
func GetOrGenerateTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(constants.TraceIDContextKey).(string); ok && traceID != "" {
		return traceID
	}
	return GenerateTraceID()
}

// GenerateTraceID generates a new UUID-based trace ID
func GenerateTraceID() string {
	return uuid.New().String()
}

// GetTraceID extracts the trace ID from the context, returns empty string if not found
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(constants.TraceIDContextKey).(string); ok {
		return traceID
	}
	return ""
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, constants.TraceIDContextKey, traceID)
}
