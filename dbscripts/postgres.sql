CREATE TABLE unification_rules (
    rule_id VARCHAR(255) PRIMARY KEY,
    rule_name VARCHAR(255) NOT NULL,
    property_name VARCHAR(255) NOT NULL,
    priority INT NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE profile_enrichment_rules (
    rule_id VARCHAR(255) PRIMARY KEY,
    property_name VARCHAR(255) NOT NULL,
    value_type VARCHAR(255) NOT NULL,
    merge_strategy VARCHAR(255) NOT NULL,
    value VARCHAR(255),
    computation_method VARCHAR(255),
    source_field VARCHAR(255),
    time_range BIGINT,
    event_type VARCHAR(255) NOT NULL,
    event_name VARCHAR(255) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE profile_enrichment_trigger_conditions (
    trigger_condition_id SERIAL PRIMARY KEY,
    rule_id VARCHAR(255) REFERENCES profile_enrichment_rules(rule_id) ON DELETE CASCADE,
    field VARCHAR(255) NOT NULL,
    operator VARCHAR(255) NOT NULL,
    value VARCHAR(255) NOT NULL
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

CREATE TABLE events (
    profile_id VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    event_name VARCHAR(255) NOT NULL,
    event_id VARCHAR(255) PRIMARY KEY,
    application_id VARCHAR(255) NOT NULL,
    org_id VARCHAR(255) NOT NULL,
    event_timestamp BIGINT NOT NULL,
    properties JSONB,
    context JSONB
);
