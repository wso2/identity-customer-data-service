-- Apply once before deploying atomic profile unification to an existing PostgreSQL database.
CREATE TABLE IF NOT EXISTS profile_unification_events
(
    event_id     VARCHAR(255) PRIMARY KEY,
    profile_id   VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
