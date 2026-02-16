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
    org_handle                   VARCHAR(255) NOT NULL,
    profile_status              VARCHAR(255),
    reference_profile_id        VARCHAR(255),
    reference_profile_org_handle VARCHAR(255),
    reference_reason            VARCHAR(255)
);

CREATE TABLE profile_schema
(
    attribute_id           VARCHAR(255) NOT NULL PRIMARY KEY,
    scope                  VARCHAR(255),
    org_handle              VARCHAR(255) NOT NULL,
    attribute_name         VARCHAR(255) NOT NULL,
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
    property_id   VARCHAR(255) REFERENCES profile_schema(attribute_id) ON DELETE CASCADE,
    priority      INT          NOT NULL,
    is_active     BOOLEAN      NOT NULL,
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
    destinations        TEXT[]
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

-- CDS Config Table
CREATE TABLE cds_config (
    org_handle VARCHAR(255) NOT NULL,
    config VARCHAR(255) NOT NULL,
    value VARCHAR(500),
    PRIMARY KEY (org_handle, config)
);

-- Indexes for performance optimization
-- Indexes on profiles table
CREATE INDEX idx_profiles_org_handle ON profiles(org_handle);
CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_profiles_org_user ON profiles(org_handle, user_id);
CREATE INDEX idx_profiles_created_at ON profiles(created_at DESC, profile_id DESC);

-- Indexes on profile_reference table
CREATE INDEX idx_profile_reference_org_handle ON profile_reference(org_handle);
CREATE INDEX idx_profile_reference_status ON profile_reference(profile_status);
CREATE INDEX idx_profile_reference_ref_profile ON profile_reference(reference_profile_id);
CREATE INDEX idx_profile_reference_org_status ON profile_reference(org_handle, profile_status);

-- Indexes on profile_schema table
CREATE INDEX idx_profile_schema_org_handle ON profile_schema(org_handle);
CREATE INDEX idx_profile_schema_org_scope ON profile_schema(org_handle, scope);
CREATE INDEX idx_profile_schema_org_attr_name ON profile_schema(org_handle, attribute_name);
CREATE INDEX idx_profile_schema_scope ON profile_schema(scope);

-- Indexes on unification_rules table
CREATE INDEX idx_unification_rules_org_handle ON unification_rules(org_handle);
CREATE INDEX idx_unification_rules_property_id ON unification_rules(property_id);
CREATE INDEX idx_unification_rules_is_active ON unification_rules(is_active);

-- Indexes on application_data table (profile_id already has FK index)
CREATE INDEX idx_application_data_app_id ON application_data(app_id);

-- Indexes on consent_categories table
CREATE INDEX idx_consent_categories_org_handle ON consent_categories(org_handle);
CREATE INDEX idx_consent_categories_name ON consent_categories(category_name);

-- Indexes on profile_consents table (profile_id and category_id already have FK indexes)

-- Indexes on profile_cookies table (profile_id already has FK index)
CREATE INDEX idx_profile_cookies_is_active ON profile_cookies(is_active);
