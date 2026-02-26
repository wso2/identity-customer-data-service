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

CREATE TABLE IF NOT EXISTS blocking_keys (
    key_id          VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    profile_id      VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    org_handle      VARCHAR(255) NOT NULL,
    attribute_name  VARCHAR(255) NOT NULL,       -- Rule property name (e.g., "identity_attributes.emailaddress")
    key_value       VARCHAR(512) NOT NULL,       -- Normalized blocking key (e.g., "john.smith@example.com", "J500 S530")

    CONSTRAINT uq_blocking_key UNIQUE (org_handle, attribute_name, key_value, profile_id)
);

-- Index for profile-level operations
CREATE INDEX IF NOT EXISTS idx_blocking_keys_profile ON blocking_keys(profile_id);

CREATE TABLE IF NOT EXISTS review_tasks (
    id                  VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_handle          VARCHAR(255) NOT NULL,
    source_profile_id   VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    target_profile_id   VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    match_score         DECIMAL(5,4) NOT NULL,       -- Score between 0 and 1 (e.g., 0.8765)
    status              VARCHAR(50) NOT NULL DEFAULT 'PENDING',  -- PENDING, APPROVED, REJECTED, EXPIRED
    match_reason        TEXT,                        -- JSON or text explaining why matched
    score_breakdown     JSONB DEFAULT '{}'::jsonb,   -- Detailed score per attribute
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at         TIMESTAMPTZ,
    resolved_by         VARCHAR(255),                -- User ID who resolved
    resolution_notes    TEXT,
    CONSTRAINT uq_review_task_profiles UNIQUE (source_profile_id, target_profile_id)
);

-- Index for fetching pending reviews by org
CREATE INDEX IF NOT EXISTS idx_review_tasks_org_status ON review_tasks(org_handle, status);
CREATE INDEX IF NOT EXISTS idx_review_tasks_created ON review_tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_tasks_source ON review_tasks(source_profile_id, status);

-- Rejection pairs: profiles that an admin explicitly rejected as non-matches.
CREATE TABLE IF NOT EXISTS rejection_pairs (
    id              VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_handle      VARCHAR(255) NOT NULL,
    profile_id_1    VARCHAR(255) NOT NULL,
    profile_id_2    VARCHAR(255) NOT NULL,
    rejected_by     VARCHAR(255),
    rejected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rejection_pair UNIQUE (profile_id_1, profile_id_2)
);

CREATE INDEX IF NOT EXISTS idx_rejection_pairs_p1 ON rejection_pairs(org_handle, profile_id_1);
CREATE INDEX IF NOT EXISTS idx_rejection_pairs_p2 ON rejection_pairs(org_handle, profile_id_2);

CREATE TABLE IF NOT EXISTS merge_audit_log (
    id                  VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_handle          VARCHAR(255) NOT NULL,
    primary_profile_id  VARCHAR(255) NOT NULL,
    secondary_profile_id VARCHAR(255) NOT NULL,
    merge_type          VARCHAR(50) NOT NULL,        -- AUTO_MERGE, MANUAL_MERGE, ADMIN_MERGE
    match_score         DECIMAL(5,4),
    merged_by           VARCHAR(255),                -- System or User ID
    merge_timestamp     TIMESTAMPTZ NOT NULL DEFAULT now(),
    merge_details       JSONB DEFAULT '{}'::jsonb,   -- What data was merged
    rollback_data       JSONB DEFAULT '{}'::jsonb    -- Data needed to undo merge
);

CREATE INDEX IF NOT EXISTS idx_merge_audit_org ON merge_audit_log(org_handle, merge_timestamp DESC);
