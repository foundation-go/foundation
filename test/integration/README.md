# Integration tests

These tests run Foundation against real Postgres, Redis and Kafka, in
containers, through the same public API a service uses.

```bash
make test-integration          # from the repository root
go test -tags=integration ./... # from this directory
```

They need a running Docker daemon and take about 70 seconds; the containers are
started once for the package and shared.

Without a daemon the run **fails**, rather than skipping. A skip would make a CI
runner with no Docker report a green integration job that tested nothing, which
is the failure mode this repository has already been bitten by. Set
`FOUNDATION_INTEGRATION_SKIP_WITHOUT_DOCKER=1` to opt into the soft skip
locally.

## Why they exist

A large part of the framework's behaviour is only true if the servers behave the
way the code assumes. No stub can settle these:

| Property | Test |
|---|---|
| `FOR UPDATE SKIP LOCKED` lets two outbox couriers share the work without either publishing an event the other already published | `TestTwoConcurrentCouriersPublishEachEventExactlyOnce` |
| Deleting by explicit id leaves a row another transaction holds untouched | `TestCourierDeletesOnlyWhatItPublished` |
| A consumer group's committed offset advances past event types the worker has no handler for | `TestEventsWorkerCommitsOffsetsForUnhandledEventTypes` |
| A payload that cannot be parsed does not pin its partition forever | `TestEventsWorkerCommitsPastAnUnparsablePayload` |
| A Redis URL's database number is honoured | `TestBuildRedisPoolHonoursTheDatabaseNumber` |
| The Kafka health check notices an unreachable cluster | `TestKafkaHealthFailsAgainstAStoppedCluster` |
| Queries produce spans and metrics, and the pool reports its real occupancy | `TestPostgresQueriesAreTracedAndMeasured`, `TestPostgresPoolStatsAreExposed` |

Each of these was, at some point, believed to hold because the code looked
right. Two of them were wrong.

The worker and courier tests start a real `EventsWorker` / `OutboxCourier`
through `Service.Start`, so they cover the whole path — components, the spin
loop, the drain, the shutdown — rather than calling internals.

## A test that cannot fail is worse than no test

The concurrency tests are only worth having if they detect the bug they
describe. `TestTwoConcurrentCouriersPublishEachEventExactlyOnce` was checked by
removing `FOR UPDATE SKIP LOCKED` from the query: it reported 150 of 300 events
published twice. Do the same when changing that query.

## Separate module

This directory is its own Go module. testcontainers pulls in the Docker client
and a large transitive graph, and a library's `go.mod` is a floor for everyone
who imports it — adding testcontainers to the framework's module would raise
every consumer's minimum versions, OpenTelemetry among them, for dependencies
they never use.

The consequence is that these tests can only use Foundation's exported API,
which is the right constraint for an integration test anyway: they exercise what
a service can actually reach.

## Adding a test

Shared plumbing is in `main_test.go`: `createTopic`, `committedOffset`,
`truncateOutbox`, `unique` (for topic and key names, since tests share
containers), and `eventually`.

Keep new tests to properties that need a real server. Anything decidable without
one belongs in the unit tests next to the code, where it runs in milliseconds.
