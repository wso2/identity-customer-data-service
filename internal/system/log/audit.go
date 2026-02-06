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

package log

import (
	"encoding/json"
	"log/slog"
	"time"
)

// AuditEvent represents a structured audit log entry
type AuditEvent struct {
	RecordedAt    string      `json:"recordedAt"`
	InitiatorID   string      `json:"initiatorId"`
	InitiatorType string      `json:"initiatorType"`
	TargetID      string      `json:"targetId"`
	TargetType    string      `json:"targetType"`
	ActionID      string      `json:"actionId"`
	TraceID       string      `json:"traceId,omitempty"`
	Data          interface{} `json:"data,omitempty"`
}

// Audit logs an audit event with structured fields
func (l *Logger) Audit(event AuditEvent) {
	// Ensure timestamp is set
	if event.RecordedAt == "" {
		event.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Convert to JSON for structured logging
	jsonData, err := json.Marshal(event)
	if err != nil {
		l.Error("Failed to marshal audit event", Error(err))
		return
	}

	// Log at Info level with "AUDIT" prefix
	l.internal.Info("AUDIT", slog.String("audit_event", string(jsonData)))
}

// Action IDs for audit logging
const (
	// Profile operations
	ActionAddProfile    = "add-profile"
	ActionUpdateProfile = "update-profile"
	ActionDeleteProfile = "delete-profile"

	// Schema attribute operations
	ActionAddSchemaAttribute    = "add-schema-attribute"
	ActionUpdateSchemaAttribute = "update-schema-attribute"
	ActionDeleteSchemaAttribute = "delete-schema-attribute"

	// Unification rule operations
	ActionAddUnificationRule    = "add-unification-rule"
	ActionUpdateUnificationRule = "update-unification-rule"
	ActionDeleteUnificationRule = "delete-unification-rule"

	// Sync and queuing operations
	ActionProfileUnification = "profile-unification"
	ActionSchemaSync         = "schema-sync"

	// Authentication operations
	ActionAuthenticationSuccess = "authentication-success"
	ActionAuthenticationFailure = "authentication-failure"
)

// Initiator types
const (
	InitiatorTypeUser   = "user"
	InitiatorTypeSystem = "system"
	InitiatorTypeAdmin  = "admin"
)

// Target types
const (
	TargetTypeProfile         = "profile"
	TargetTypeSchemaAttribute = "schema-attribute"
	TargetTypeUnificationRule = "unification-rule"
	TargetTypeSchema          = "schema"
)
