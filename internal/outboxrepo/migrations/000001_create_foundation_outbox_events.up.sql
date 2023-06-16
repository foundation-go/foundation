CREATE TABLE foundation_outbox_events (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    partition INT NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
