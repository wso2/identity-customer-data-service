CREATE TABLE profile_unification_modes
(
    id         SERIAL PRIMARY KEY,
    org_handle  VARCHAR(255) NOT NULL,
    merge_type VARCHAR(255) NOT NULL,
    rule       VARCHAR(255) NOT NULL,
    UNIQUE (org_handle, merge_type, rule)
);

CREATE TABLE profile_unification_triggers
(
    id           SERIAL PRIMARY KEY,
    org_handle    VARCHAR(255) NOT NULL UNIQUE,
    trigger_type VARCHAR(255) NOT NULL,
    last_trigger BIGINT DEFAULT 0,
    duration     BIGINT DEFAULT 0
);

-- Profiles Table
CREATE TABLE profiles
(
    profile_id          VARCHAR(255) PRIMARY KEY,
    user_id             VARCHAR(255),
    org_handle           VARCHAR(255),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    location            VARCHAR(255),
    origin_country      VARCHAR(255),
    list_profile        BOOLEAN DEFAULT TRUE,
    delete_profile      BOOLEAN DEFAULT FALSE,
    traits              JSONB   DEFAULT '{}'::jsonb,
    identity_attributes JSONB   DEFAULT '{}'::jsonb
);

CREATE TABLE profile_reference
(
    profile_id                  VARCHAR(255) PRIMARY KEY,
    org_handle                  VARCHAR(255) NOT NULL,
    profile_status              VARCHAR(255),
    reference_profile_id        VARCHAR(255),
    reference_profile_org_handle VARCHAR(255),
    reference_reason            VARCHAR(255)
);

CREATE TABLE profile_schema
(
    attribute_id           VARCHAR(255) NOT NULL PRIMARY KEY,
    scope                  VARCHAR(255),
    org_handle             VARCHAR(255) NOT NULL,
    attribute_name         VARCHAR(255) NOT NULL,
    display_name           VARCHAR(255) NOT NULL,
    value_type             VARCHAR(255) NOT NULL,
    merge_strategy         VARCHAR(255) NOT NULL,
    application_identifier VARCHAR(255) NOT NULL,
    mutability             VARCHAR(255) NOT NULL,
    multi_valued           BOOLEAN DEFAULT FALSE,
    canonical_values       JSONB   DEFAULT '[]'::jsonb,
    sub_attributes         JSONB   DEFAULT '[]'::jsonb,
    scim_dialect VARCHAR(255)
);

CREATE TABLE unification_rules
(
    rule_id       VARCHAR(255) PRIMARY KEY,
    org_handle     VARCHAR(255) NOT NULL,
    rule_name     VARCHAR(255) NOT NULL,
    property_name VARCHAR(255) NOT NULL,
    property_id  VARCHAR(255) REFERENCES profile_schema(attribute_id) ON DELETE CASCADE,
    priority      INT          NOT NULL,
    is_active     BOOLEAN      NOT NULL,
    attribute_type     VARCHAR(255) NOT NULL DEFAULT 'PRIMITIVE_EXACT',
    unification_method VARCHAR(255) NOT NULL DEFAULT 'deterministic',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Application Data Table
CREATE TABLE application_data
(
    app_data_id      SERIAL PRIMARY KEY,
    profile_id       VARCHAR(255) REFERENCES profiles (profile_id) ON DELETE CASCADE,
    app_id           VARCHAR(255) NOT NULL,
    application_data JSONB DEFAULT '{}'::jsonb,
    UNIQUE (profile_id, app_id)
);

CREATE TABLE consent_categories
(
    id                  SERIAL PRIMARY KEY,
    org_handle           VARCHAR(255)        NOT NULL,
    category_name       VARCHAR(255)        NOT NULL,
    category_identifier VARCHAR(255) UNIQUE NOT NULL,
    purpose             VARCHAR(255)        NOT NULL,
    destinations        TEXT[],
    is_mandatory        BOOLEAN             NOT NULL DEFAULT FALSE
);

CREATE TABLE consent_category_attributes
(
    id                     SERIAL PRIMARY KEY,
    category_id            VARCHAR(255) REFERENCES consent_categories (category_identifier) ON DELETE CASCADE,
    scope                  VARCHAR(50)  NOT NULL,
    attribute_name         VARCHAR(255) NOT NULL,
    attribute_id           VARCHAR(255) REFERENCES profile_schema (attribute_id) ON DELETE CASCADE,
    application_identifier VARCHAR(255) NOT NULL DEFAULT '',
    UNIQUE (category_id, scope, attribute_name, application_identifier)
);

CREATE TABLE profile_consents
(
    id      SERIAL PRIMARY KEY,
    profile_id       VARCHAR(255) REFERENCES profiles (profile_id) ON DELETE CASCADE,
    category_id      VARCHAR (255) REFERENCES consent_categories (category_identifier) ON DELETE CASCADE,
    consent_status   BOOLEAN     NOT NULL,
    consented_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (profile_id, category_id)
);

CREATE TABLE profile_cookies (
                                 cookie_id VARCHAR (255) PRIMARY KEY,
                                 profile_id VARCHAR (255) NOT NULL REFERENCES profiles (profile_id) ON DELETE CASCADE,
                                 is_active BOOLEAN NOT NULL DEFAULT true
);

-- Prevents duplicate entries for the same profile and app (it generally does upsert)
ALTER TABLE application_data
    ADD CONSTRAINT unique_profile_app UNIQUE (profile_id, app_id);

CREATE TABLE cds_config (
    org_handle VARCHAR(255) NOT NULL,
    config VARCHAR(255) NOT NULL,
    value VARCHAR(500),
    PRIMARY KEY (org_handle, config)
);

CREATE TABLE IF NOT EXISTS blocking_keys (
    key_id          VARCHAR(255) PRIMARY KEY,
    profile_id      VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    org_handle      VARCHAR(255) NOT NULL,
    attribute_name  VARCHAR(255) NOT NULL,
    key_value       VARCHAR(512) NOT NULL,
    CONSTRAINT uq_blocking_key UNIQUE (org_handle, attribute_name, key_value, profile_id)
);

CREATE INDEX IF NOT EXISTS idx_blocking_keys_profile ON blocking_keys(profile_id);

CREATE TABLE IF NOT EXISTS review_tasks (
    id                      VARCHAR(255) PRIMARY KEY,
    org_handle              VARCHAR(255) NOT NULL,
    incoming_profile_id       VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    candidate_profile_id    VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    match_score             DECIMAL(5,4) NOT NULL,
    status                  VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    match_reason            TEXT,
    score_breakdown         JSONB DEFAULT '{}'::jsonb,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at             TIMESTAMPTZ,
    resolved_by             VARCHAR(255),
    resolution_notes        TEXT,
    CONSTRAINT uq_review_task_profiles UNIQUE (incoming_profile_id, candidate_profile_id)
);

CREATE INDEX IF NOT EXISTS idx_review_tasks_org_status ON review_tasks(org_handle, status);
CREATE INDEX IF NOT EXISTS idx_review_tasks_created ON review_tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_tasks_incoming ON review_tasks(incoming_profile_id, status);

CREATE TABLE IF NOT EXISTS rejection_pairs (
    id              VARCHAR(255) PRIMARY KEY,
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
    id                      VARCHAR(255) PRIMARY KEY,
    org_handle              VARCHAR(255) NOT NULL,
    primary_profile_id      VARCHAR(255) NOT NULL,
    secondary_profile_id    VARCHAR(255) NOT NULL,
    merge_type              VARCHAR(50) NOT NULL,
    match_score             DECIMAL(5,4),
    merged_by               VARCHAR(255),
    merge_timestamp         TIMESTAMPTZ NOT NULL DEFAULT now(),
    merge_details           JSONB DEFAULT '{}'::jsonb,
    rollback_data           JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_merge_audit_org ON merge_audit_log(org_handle, merge_timestamp DESC);

-- ================================
-- PROFILES (Hot path: tenant + cursor pagination + ordering)
-- ================================
CREATE INDEX IF NOT EXISTS idx_profiles_org_created_profile
    ON profiles (org_handle, created_at, profile_id);

-- user_id filtering within tenant
CREATE INDEX IF NOT EXISTS idx_profiles_org_user
    ON profiles (org_handle, user_id);

-- Optional: speeds ILIKE/contains on user_id
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_profiles_user_id_trgm
    ON profiles USING GIN (user_id gin_trgm_ops);


-- ================================
-- PROFILE_REFERENCE (Join + status filtering)
-- ================================
CREATE INDEX IF NOT EXISTS idx_profile_reference_status_profile
    ON profile_reference (profile_status, profile_id);

-- Optional but useful if org filtering is frequent on reference table
CREATE INDEX IF NOT EXISTS idx_profile_reference_org_status_profile
    ON profile_reference (org_handle, profile_status, profile_id);

-- For lookups by reference_profile_id
CREATE INDEX IF NOT EXISTS idx_profile_reference_reference_profile
    ON profile_reference (reference_profile_id);


-- ================================
-- APPLICATION_DATA (Joins + filtering)
-- ================================
-- Postgres does NOT auto-index FK columns → required
CREATE INDEX IF NOT EXISTS idx_application_data_profile_id
    ON application_data (profile_id);

-- Useful if querying by app_id alone
CREATE INDEX IF NOT EXISTS idx_application_data_app_id
    ON application_data (app_id);

-- JSONB filtering inside app_specific_data
CREATE INDEX IF NOT EXISTS idx_application_data_app_specific_gin
    ON application_data USING GIN ((application_data -> 'app_specific_data'));


-- ================================
-- JSONB FILTERING (Profiles)
-- ================================
-- These help mainly after switching eq → @> (still safe to add now)
CREATE INDEX IF NOT EXISTS idx_profiles_traits_gin
    ON profiles USING GIN (traits);

CREATE INDEX IF NOT EXISTS idx_profiles_identity_attributes_gin
    ON profiles USING GIN (identity_attributes);


-- ================================
-- PROFILE_SCHEMA (Rare filtering, minimal indexes)
-- ================================
CREATE INDEX IF NOT EXISTS idx_profile_schema_org_scope
    ON profile_schema (org_handle, scope);

CREATE INDEX IF NOT EXISTS idx_profile_schema_org_attr_name
    ON profile_schema (org_handle, attribute_name);


-- ================================
-- UNIFICATION_RULES
-- ================================
CREATE INDEX IF NOT EXISTS idx_unification_rules_org_active_priority
    ON unification_rules (org_handle, is_active, priority);

CREATE INDEX IF NOT EXISTS idx_unification_rules_property_id
    ON unification_rules (property_id);
