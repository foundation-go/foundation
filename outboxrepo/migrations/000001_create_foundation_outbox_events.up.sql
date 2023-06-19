CREATE TABLE foundation_outbox_events (
    id BIGINT PRIMARY KEY,
    topic TEXT NOT NULL,
    key TEXT NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
