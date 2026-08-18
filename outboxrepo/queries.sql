-- name: CreateOutboxEvent :exec
INSERT INTO foundation_outbox_events (topic, key, payload, headers, created_at)
VALUES ($1, $2, $3, $4, NOW());

-- Locks the batch it returns and skips rows another courier already holds, so
-- that several courier replicas can run without publishing the same events
-- twice.
--
-- N.B.: SKIP LOCKED trades ordering for parallelism. With one courier, events
-- reach Kafka in insertion order. With several, a replica skips rows another
-- has locked, so two events for the same key can be published out of order —
-- and the Hash balancer puts them on the same partition, so consumers see the
-- swap. Run a single courier where per-key ordering matters.
-- name: ListOutboxEvents :many
SELECT * FROM foundation_outbox_events
ORDER BY id ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- Deletes exactly the rows that were published. Deleting everything up to a
-- maximum id would also remove rows a concurrent courier had locked but not yet
-- committed — and if that courier then rolled back, those events would be gone
-- without ever having been published.
-- name: DeleteOutboxEvents :exec
DELETE FROM foundation_outbox_events WHERE id = ANY($1::bigint[]);

-- Counts the whole backlog, which is what an alert needs: the size of a batch
-- can never exceed the limit it was read with, so it says nothing about how far
-- behind the courier is.
-- name: CountOutboxEvents :one
SELECT COUNT(*) FROM foundation_outbox_events;
