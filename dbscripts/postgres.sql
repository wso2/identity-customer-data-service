CREATE TABLE unification_rules (
    rule_id VARCHAR(255) PRIMARY KEY,
    org_id VARCHAR(255) NOT NULL,
    rule_name VARCHAR(255) NOT NULL,
    property_name VARCHAR(255) NOT NULL,
    priority INT NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- Profiles Table
CREATE TABLE profiles (
    profile_id VARCHAR(255) PRIMARY KEY,
    origin_country VARCHAR(255),
    is_parent BOOLEAN DEFAULT TRUE,
    parent_profile_id VARCHAR(255),
    list_profile BOOLEAN DEFAULT TRUE,
    traits JSONB DEFAULT '{}'::jsonb,
    identity_attributes JSONB DEFAULT '{}'::jsonb
);

CREATE TABLE profile_schema (
    attribute_id SERIAL PRIMARY KEY,
    org_id VARCHAR(255) NOT NULL,
    attribute_name VARCHAR(255) NOT NULL,
    attribute_type VARCHAR(255) NOT NULL,
    merge_strategy VARCHAR(255) NOT NULL,
);

-- Application Data Table
CREATE TABLE application_data (
    app_data_id SERIAL PRIMARY KEY,
    profile_id VARCHAR(255) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    app_id VARCHAR(255) NOT NULL,
    application_data JSONB DEFAULT '{}'::jsonb,
    UNIQUE (profile_id, app_id)
);

-- Child Profiles Table
CREATE TABLE child_profiles (
    parent_profile_id VARCHAR(255) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    child_profile_id VARCHAR(255) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    rule_name VARCHAR(255),
    PRIMARY KEY (parent_profile_id, child_profile_id)
);

CREATE TABLE consent_categories (
    id SERIAL PRIMARY KEY ,
    org_id VARCHAR (255) NOT NULL,
    category_name VARCHAR (255) NOT NULL,
    category_identifier VARCHAR (255) UNIQUE NOT NULL,
    purpose VARCHAR (255) NOT NULL,
    destinations TEXT[]
);

