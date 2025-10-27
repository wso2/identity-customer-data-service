CREATE TABLE unification_rules
(
    rule_id       VARCHAR(255) PRIMARY KEY,
    tenant_id     VARCHAR(255) NOT NULL,
    rule_name     VARCHAR(255) NOT NULL,
    property_name VARCHAR(255) NOT NULL,
    priority      INT          NOT NULL,
    is_active     BOOLEAN      NOT NULL,
    created_at    BIGINT       NOT NULL,
    updated_at    BIGINT       NOT NULL
);

CREATE TABLE profile_unification_modes
(
    id         SERIAL PRIMARY KEY,
    tenant_id  VARCHAR(255) NOT NULL,
    merge_type VARCHAR(255) NOT NULL,
    rule       VARCHAR(255) NOT NULL,
    UNIQUE (tenant_id, merge_type, rule)
);

CREATE TABLE profile_unification_triggers
(
    id           SERIAL PRIMARY KEY,
    tenant_id    VARCHAR(255) NOT NULL UNIQUE,
    trigger_type VARCHAR(255) NOT NULL,
    last_trigger BIGINT DEFAULT 0,
    duration     BIGINT DEFAULT 0
);

-- Profiles Table
CREATE TABLE profiles
(
    profile_id          VARCHAR(255) PRIMARY KEY,
    user_id             VARCHAR(255),
    tenant_id           VARCHAR(255),
    created_at          BIGINT,
    updated_at          BIGINT,
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
    tenant_id                   VARCHAR(255) NOT NULL,
    profile_status              VARCHAR(255),
    reference_profile_id        VARCHAR(255),
    reference_profile_tenant_id VARCHAR(255),
    reference_reason            VARCHAR(255)
);

CREATE TABLE profile_schema
(
    attribute_id           VARCHAR(255) NOT NULL PRIMARY KEY,
    scope                  VARCHAR(255),
    tenant_id              VARCHAR(255) NOT NULL,
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
    tenant_id           VARCHAR(255)        NOT NULL,
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
    consent_status   BOOLEAN NOT NULL,
    consented_at     BIGINT NOT NULL,
    UNIQUE (profile_id, category_id)
);

CREATE TABLE profile_cookies (
    cookie_id VARCHAR (255) PRIMARY KEY,
    profile_id VARCHAR (255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Prevents duplicate entries for the same profile and app (it generally does upsert)
ALTER TABLE application_data
    ADD CONSTRAINT unique_profile_app UNIQUE (profile_id, app_id);

-- CDS Config Table
CREATE TABLE cds_config (
                            tenant_id VARCHAR(255) ,
                            cds_enabled BOOLEAN DEFAULT FALSE,
                            initial_schema_sync_done BOOLEAN DEFAULT FALSE
);