CREATE TABLE unification_rules (
    rule_id VARCHAR(255) PRIMARY KEY,
    rule_name VARCHAR(255) NOT NULL,
    property VARCHAR(255) NOT NULL,
    priority INT NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
