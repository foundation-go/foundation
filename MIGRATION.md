# Upgrading

## 0.2.x → 0.3.0

Most services need no code changes. Everything that could require one is
below, roughly in the order you are likely to hit it.

### Defaults that changed

These are behaviour changes, not compile errors — check them before deploying.

**Identity headers are stripped at the gateway.** `X-Authenticated`,
`X-User-Id`, `X-Client-Id`, `X-Scope` and `X-Metadata` are removed from every
incoming request, along with their `Grpc-Metadata-` aliases. If a proxy in
front of your gateway sets them, set
`GatewayOptions.TrustInboundAuthenticationHeaders` — otherwise those requests
now arrive unauthenticated.

**A gateway with `WithAuthentication` refuses to start** without an
`AuthenticationDetailsMiddleware` or `TrustInboundAuthenticationHeaders`. That
combination could never authenticate anything: it accepted whatever the client
sent.

**Probes.** Point liveness at `/live` and readiness at `/ready`. `/health`
keeps its readiness semantics, so existing manifests keep working, but an
unhealthy service answers `503` where it used to answer `500`.

**Tracing samples everything.** `OTEL_TRACES_SAMPLER_RATIO` defaulted to `0`
and now defaults to `1.0`, which is what ENV.md always claimed. Set it
explicitly if your collector cannot take the volume.

**gRPC payload logging is off.** `LOG_PAYLOADS=true` restores it.

**Auto topic creation is off in production.**
`KAFKA_ALLOW_AUTO_TOPIC_CREATION=true` restores it.

**`EVENTS_WORKER_ERRORS_TOPIC` now does something.** It defaults to empty,
which reproduces the old behaviour (the topic is derived from the proto name).
If you had it set, error events will start going where it points.

**Duplicate handler registrations are a startup error.** Registering handlers
for one event type under two different `proto.Message` keys used to keep one
and drop the other silently. Merge them into a single map entry.

**Several outbox couriers can run at once**, and that costs ordering: a replica
skips rows another has locked, so two events for the same key can reach Kafka
out of order on the same partition. Run a single courier where per-key ordering
matters.

### Signature changes

| Before | After |
|---|---|
| `gateway.RegisterServices(services, opts)` | `gateway.RegisterServices(ctx, services, opts)`, returning `*gateway.Mux` |
| `Service.DeleteOutboxEvents(ctx, tx, maxID int64)` | `Service.DeleteOutboxEvents(ctx, tx, ids []int64)` |
| `cable_courier.Client.BroadcastMessage(name, msg, stream, corrID)` | `BroadcastMessage(ctx, name, msg, stream, corrID)` |
| `KafkaProducerConfig.BatchTimeout int` (seconds) | `KafkaProducerConfig.BatchFlushInterval time.Duration` |
| `ConnIdentifier.AccessToken` | `ConnIdentifier.ConnectionID` |
| `EventsWorkerOptions.ProtoNamesToMessages()` | removed — use `Registry()` |
| `foundation.Clone` | removed; it never cloned anything |

`BatchFlushInterval` is renamed rather than retyped on purpose: keeping the name
would have let `BatchTimeout = 5` compile against a `time.Duration` and quietly
mean five nanoseconds. The `KAFKA_PRODUCER_BATCH_TIMEOUT` variable is unchanged
and still reads a bare number as seconds.

### Semantic changes to existing signatures

`grpc.GetMetadataValue` returns the first value for a repeated key instead of
joining them with a comma. `GetMetadataValues` returns them all. The old join
produced strings that were not valid values of anything — a duplicated
`x-user-id` became `id1,id2` — and for `x-scope` it granted scopes nobody had.

The dependency getters — `GetPostgreSQL`, `GetRedis`, `GetKafkaConsumer`,
`GetKafkaProducer`, `GetJobsEnqueuer` — panic instead of calling `os.Exit` when
the component is missing. With the recovery middleware in place that costs one
request rather than the process. `TryGetX` returns an error instead.

`context` getters return the zero value when a key is absent, where they used
to panic on an unchecked type assertion. `LookupX` distinguishes absent from
empty; `MustGetX` panics with a message that names the key.

`Version` is a variable read from the build information, not a constant. A
downstream `const x = foundation.Version` no longer compiles.

`foundation.Init` and the CLI call `foundation.LoadEnv()`; the
`godotenv/autoload` side-effect import is gone. Call `LoadEnv` yourself if you
read the environment before either.

### Optional, but worth doing

Implement `HealthCheckerContext` on your own components so the readiness probe
can bound them. Components that only implement `Component` are still checked,
but a hanging `Health()` can only be abandoned, not cancelled.

Use `JobOptions.HandlerWithContext` instead of `Handler`: it receives a context
that is cancelled on shutdown and carries the job's correlation ID.
