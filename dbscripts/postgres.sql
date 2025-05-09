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

CREATE TABLE events (
    profile_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_name VARCHAR(255) NOT NULL,
    event_id VARCHAR(255) PRIMARY KEY,
    application_id VARCHAR(255) NOT NULL,
    org_id VARCHAR(255) NOT NULL,
    event_timestamp BIGINT NOT NULL, -- Assuming Unix timestamp in seconds
    properties JSONB,
    context JSONB
);

CREATE TABLE profiles (
    profile_id VARCHAR(255) PRIMARY KEY, -- Using your natural key as the PK
    profile_data JSONB NOT NULL -- The entire Profile struct goes here
);

