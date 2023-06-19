-- name: CreateOutboxEvent :exec
INSERT INTO foundation_outbox_events (topic, key, payload, headers, created_at)
VALUES ($1, $2, $3, $4, NOW());

-- name: ListOutboxEvents :many
SELECT * FROM foundation_outbox_events ORDER BY id ASC LIMIT $1;

-- name: DeleteOutboxEvents :exec
DELETE FROM foundation_outbox_events WHERE id <= $1;
