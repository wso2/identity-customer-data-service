#!/bin/bash
#
# Identity resolution: schema delta for the existing CDS database.
#
# Adds the three tables and four columns identity resolution needs, and nothing else.
# Purely additive — no table, column, index or row is dropped or rewritten — so it is safe
# against a live database, and every statement is guarded, so it is safe to run twice.
#
# Usage: ./identityResolutionSchema.sh <env>

set -e
set -o pipefail
env=$1

if [ "$env" == "prod-us" ]; then
    echo "exporting prod-us..."
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-pr-eus2-01
    export PG_HOST="asgardeo-prod-us.postgres.database.azure.com"
    export PG_DB="cdsdb"
elif [ "$env" == "prod-eu" ]; then
    echo "exporting prod-eu..."
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-eupr-ne-02
    export PG_HOST="asgardeo-prod-eu.postgres.database.azure.com"
    export PG_DB="cdsdb"
elif [ "$env" == "stage-us" ]; then
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-st-eus2-01
    export PG_HOST="asgardeo-stage-us.postgres.database.azure.com"
    export PG_DB="cdsdb"
elif [ "$env" == "stage-eu" ]; then
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-eust-ne-01
    export PG_HOST="asgardeo-stage-eu.postgres.database.azure.com"
    export PG_DB="cdsdb"
elif [ "$env" == "dev-us" ]; then
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-de-eus2-01
    export PG_HOST="asgardeo-dev-us.postgres.database.azure.com"
    export PG_DB="cdsdb"
elif [ "$env" == "dev-eu" ]; then
    export ADMIN_KEY_VAULT_NAME=kv-asgr-admin-eu-ne-01
    export PG_HOST="asgardeo-dev-eu.postgres.database.azure.com"
    export PG_DB="cdsdb"
else
  echo "Invalid environment."
  exit 1
fi

echo "Configuring DB credentials for $env..."

export PG_USER=$(az keyvault secret show --name postgres-admin-username --vault-name $ADMIN_KEY_VAULT_NAME --query value -o tsv)
export PG_PASS=$(az keyvault secret show --name postgres-admin-password --vault-name $ADMIN_KEY_VAULT_NAME --query value -o tsv)

export PGPASSWORD=$PG_PASS

echo "Applying the identity resolution schema delta..."

# ON_ERROR_STOP with --single-transaction: either every statement lands or none does, so a
# failure part-way cannot leave the database half-migrated.
psql -h "$PG_HOST" -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 --single-transaction << 'EOF'

-- UNIFICATION RULES: how a rule compares values.
--
-- attribute_type and unification_method carry defaults on purpose: a rule written before
-- typed matching is an exact match on a plain value, which is what the engine applied then
-- and must keep applying now.
ALTER TABLE unification_rules
    ADD COLUMN IF NOT EXISTS attribute_type     VARCHAR(255) NOT NULL DEFAULT 'PRIMITIVE_EXACT';
ALTER TABLE unification_rules
    ADD COLUMN IF NOT EXISTS unification_method VARCHAR(255) NOT NULL DEFAULT 'deterministic';

-- The strengths are deliberately nullable WITH NO DEFAULT. A default here would stamp a
-- concrete value onto every existing rule, which the engine cannot tell apart from an
-- operator's deliberate override — it would stop deriving the strength from attribute_type
-- and silently change which profiles merge. Do not add a default or NOT NULL here.
ALTER TABLE unification_rules
    ADD COLUMN IF NOT EXISTS match_strength     VARCHAR(20);
ALTER TABLE unification_rules
    ADD COLUMN IF NOT EXISTS mismatch_strength  VARCHAR(20);

-- BLOCKING KEYS: the candidate index.
--
-- Hash partitioned on org_handle, and again on key_value beneath it, giving 8 x 4 = 32
-- leaves. The moduli cannot be changed later without rebuilding the table.
CREATE TABLE IF NOT EXISTS blocking_keys (
    profile_id      VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    org_handle      VARCHAR(255) NOT NULL,
    attribute_name  VARCHAR(255) NOT NULL,
    key_value       VARCHAR(512) NOT NULL,
    PRIMARY KEY (org_handle, attribute_name, key_value, profile_id)
) PARTITION BY HASH (org_handle);

DO $$
BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS blocking_keys_p%s PARTITION OF blocking_keys '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s) PARTITION BY HASH (key_value)', i, i);
        FOR j IN 0..3 LOOP
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS blocking_keys_p%s_k%s PARTITION OF blocking_keys_p%s '
                'FOR VALUES WITH (MODULUS 4, REMAINDER %s)', i, j, i, j);
        END LOOP;
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS idx_blocking_keys_org_profile ON blocking_keys(org_handle, profile_id);

-- REVIEW TASKS: pairs awaiting an administrator's decision.
CREATE TABLE IF NOT EXISTS review_tasks (
    id                     VARCHAR(255) PRIMARY KEY,
    org_handle             VARCHAR(255) NOT NULL,
    incoming_profile_id    VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    candidate_profile_id   VARCHAR(255) NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    match_score            DECIMAL(5,4) NOT NULL,
    status                 VARCHAR(50)  NOT NULL DEFAULT 'PENDING',
    match_reason           TEXT,
    score_breakdown        JSONB DEFAULT '{}'::jsonb,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    resolved_at            TIMESTAMPTZ,
    resolved_by            VARCHAR(255),
    resolution_notes       TEXT,
    CONSTRAINT uq_review_task_profiles UNIQUE (incoming_profile_id, candidate_profile_id)
);

CREATE INDEX IF NOT EXISTS idx_review_tasks_org_status ON review_tasks(org_handle, status);
CREATE INDEX IF NOT EXISTS idx_review_tasks_created   ON review_tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_tasks_incoming  ON review_tasks(incoming_profile_id, status);

-- REJECTION PAIRS: pairs an administrator ruled are different people.
CREATE TABLE IF NOT EXISTS rejection_pairs (
    id              VARCHAR(255) PRIMARY KEY,
    org_handle      VARCHAR(255) NOT NULL,
    profile_id_1    VARCHAR(255) NOT NULL,
    profile_id_2    VARCHAR(255) NOT NULL,
    match_score     DECIMAL(5,4),
    score_breakdown JSONB DEFAULT '{}'::jsonb,
    rejected_by     VARCHAR(255),
    rejected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rejection_pair UNIQUE (profile_id_1, profile_id_2)
);

CREATE INDEX IF NOT EXISTS idx_rejection_pairs_p1 ON rejection_pairs(org_handle, profile_id_1);
CREATE INDEX IF NOT EXISTS idx_rejection_pairs_p2 ON rejection_pairs(org_handle, profile_id_2);

-- SCHEMA DELTA ENDS
EOF

echo "Verifying..."

# Each check prints the value and what it should be. The last two matter most: a default on
# either strength column stamps a value onto every migrated rule, which the engine reads as
# a deliberate override instead of deriving it from attribute_type.
psql -h "$PG_HOST" -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 << 'EOF'
\echo 'expected: tables=3  partitions=32  columns=4  nullable_strengths=2  rules_with_stored_strength=0'
SELECT
  (SELECT count(*) FROM pg_tables
    WHERE tablename IN ('blocking_keys','review_tasks','rejection_pairs'))        AS tables,
  (SELECT count(*) FROM pg_class
    WHERE relname ~ '^blocking_keys_p[0-9]+_k[0-9]+$' AND relkind = 'r')          AS partitions,
  (SELECT count(*) FROM information_schema.columns
    WHERE table_name = 'unification_rules'
      AND column_name IN ('attribute_type','unification_method',
                          'match_strength','mismatch_strength'))                  AS columns,
  (SELECT count(*) FROM information_schema.columns
    WHERE table_name = 'unification_rules'
      AND column_name IN ('match_strength','mismatch_strength')
      AND is_nullable = 'YES' AND column_default IS NULL)                         AS nullable_strengths,
  (SELECT count(*) FROM unification_rules
    WHERE match_strength IS NOT NULL OR mismatch_strength IS NOT NULL)            AS rules_with_stored_strength;
EOF

echo "Schema delta completed successfully!"
