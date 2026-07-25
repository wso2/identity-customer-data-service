-- SQLite schema for the inbuilt CDS database.
--
-- This is the SQLite counterpart of postgres.sql and MUST be kept in sync with
-- it. It is embedded into the binary (see embed.go) and applied automatically on
-- startup when `datasource.type` is "sqlite", so every statement is written with
-- IF NOT EXISTS and the script is safe to re-run on an existing database.
--
-- Translation notes, relative to postgres.sql:
--   * SERIAL PRIMARY KEY          -> INTEGER PRIMARY KEY AUTOINCREMENT
--   * TIMESTAMPTZ                 -> TIMESTAMP. The declared type matters: the
--                                   driver scans TIMESTAMP/DATETIME columns back
--                                   into time.Time, but not TIMESTAMPTZ.
--   * DEFAULT now()               -> DEFAULT (strftime(...)||'+00:00'). Do NOT
--                                   replace this with datetime('now'): that drops
--                                   the sub-second part and the UTC offset, and
--                                   these columns are compared as text by the
--                                   keyset pagination in GetProfilesByOrgId.
--   * JSONB                       -> JSONB, holding JSON text. The declared type
--                                   is kept so the client can hand these columns
--                                   back to the stores as []byte, matching
--                                   lib/pq.
--   * TEXT[] (destinations)       -> TEXT, holding a JSON array.
--   * pg_trgm and all GIN indexes -> omitted; SQLite has no equivalent. The
--                                   `co`/`sw` filters and JSON filtering fall
--                                   back to table scans, which is acceptable at
--                                   the scale the inbuilt database targets.
--   * ALTER TABLE ... ADD CONSTRAINT -> omitted; SQLite does not support it, and
--                                   the inline UNIQUE (profile_id, app_id) on
--                                   application_data already provides the
--                                   constraint that ON CONFLICT relies on.
--
-- VARCHAR(n) lengths are retained for readability but are not enforced by
-- SQLite (TEXT affinity).

CREATE TABLE IF NOT EXISTS profile_unification_modes
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    org_handle VARCHAR(255) NOT NULL,
    merge_type VARCHAR(255) NOT NULL,
    rule       VARCHAR(255) NOT NULL,
    UNIQUE (org_handle, merge_type, rule)
);

CREATE TABLE IF NOT EXISTS profile_unification_triggers
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    org_handle   VARCHAR(255) NOT NULL UNIQUE,
    trigger_type VARCHAR(255) NOT NULL,
    last_trigger BIGINT DEFAULT 0,
    duration     BIGINT DEFAULT 0
);

-- Profiles Table
CREATE TABLE IF NOT EXISTS profiles
(
    profile_id          VARCHAR(255) PRIMARY KEY,
    user_id             VARCHAR(255),
    org_handle          VARCHAR(255),
    created_at          TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    updated_at          TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    location            VARCHAR(255),
    origin_country      VARCHAR(255),
    list_profile        BOOLEAN DEFAULT TRUE,
    delete_profile      BOOLEAN DEFAULT FALSE,
    traits              JSONB   DEFAULT '{}',
    identity_attributes JSONB   DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS profile_reference
(
    profile_id                   VARCHAR(255) PRIMARY KEY,
    org_handle                   VARCHAR(255) NOT NULL,
    profile_status               VARCHAR(255),
    reference_profile_id         VARCHAR(255),
    reference_profile_org_handle VARCHAR(255),
    reference_reason             VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS profile_schema
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
    canonical_values       JSONB   DEFAULT '[]',
    sub_attributes         JSONB   DEFAULT '[]',
    scim_dialect           VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS unification_rules
(
    rule_id       VARCHAR(255) PRIMARY KEY,
    org_handle    VARCHAR(255) NOT NULL,
    rule_name     VARCHAR(255) NOT NULL,
    property_name VARCHAR(255) NOT NULL,
    property_id   VARCHAR(255) REFERENCES profile_schema (attribute_id) ON DELETE CASCADE,
    priority      INT          NOT NULL,
    is_active     BOOLEAN      NOT NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    updated_at    TIMESTAMP    NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00')
);

-- Application Data Table
CREATE TABLE IF NOT EXISTS application_data
(
    app_data_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id       VARCHAR(255) REFERENCES profiles (profile_id) ON DELETE CASCADE,
    app_id           VARCHAR(255) NOT NULL,
    application_data JSONB DEFAULT '{}',
    UNIQUE (profile_id, app_id)
);

CREATE TABLE IF NOT EXISTS applications
(
    app_id     VARCHAR(255) PRIMARY KEY,
    org_handle VARCHAR(255) NOT NULL,
    client_id  VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    updated_at TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    UNIQUE (org_handle, client_id)
);

CREATE TABLE IF NOT EXISTS consent_categories
(
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    org_handle          VARCHAR(255)        NOT NULL,
    category_name       VARCHAR(255)        NOT NULL,
    category_identifier VARCHAR(255) UNIQUE NOT NULL,
    purpose             VARCHAR(255)        NOT NULL,
    destinations        TEXT,
    is_mandatory        BOOLEAN             NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS consent_category_attributes
(
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    category_id            VARCHAR(255) REFERENCES consent_categories (category_identifier) ON DELETE CASCADE,
    scope                  VARCHAR(50)  NOT NULL,
    attribute_name         VARCHAR(255) NOT NULL,
    attribute_id           VARCHAR(255) REFERENCES profile_schema (attribute_id) ON DELETE CASCADE,
    application_identifier VARCHAR(255) NOT NULL DEFAULT '',
    UNIQUE (category_id, scope, attribute_name, application_identifier)
);

CREATE TABLE IF NOT EXISTS profile_consents
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id     VARCHAR(255) REFERENCES profiles (profile_id) ON DELETE CASCADE,
    category_id    VARCHAR(255) REFERENCES consent_categories (category_identifier) ON DELETE CASCADE,
    consent_status BOOLEAN   NOT NULL,
    consented_at   TIMESTAMP NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now') || '+00:00'),
    UNIQUE (profile_id, category_id)
);

CREATE TABLE IF NOT EXISTS profile_cookies
(
    cookie_id  VARCHAR(255) PRIMARY KEY,
    profile_id VARCHAR(255) NOT NULL REFERENCES profiles (profile_id) ON DELETE CASCADE,
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE
);

-- CDS Config Table
CREATE TABLE IF NOT EXISTS cds_config
(
    org_handle VARCHAR(255) NOT NULL,
    config     VARCHAR(255) NOT NULL,
    value      VARCHAR(500),
    PRIMARY KEY (org_handle, config)
);

-- ================================
-- PROFILES (Hot path: tenant + cursor pagination + ordering)
-- ================================
CREATE INDEX IF NOT EXISTS idx_profiles_org_created_profile
    ON profiles (org_handle, created_at, profile_id);

-- user_id filtering within tenant
CREATE INDEX IF NOT EXISTS idx_profiles_org_user
    ON profiles (org_handle, user_id);


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
CREATE INDEX IF NOT EXISTS idx_application_data_profile_id
    ON application_data (profile_id);

-- Useful if querying by app_id alone
CREATE INDEX IF NOT EXISTS idx_application_data_app_id
    ON application_data (app_id);


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
