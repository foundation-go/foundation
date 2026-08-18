# Available environment variables

Every setting below is read once, at startup. Durations accept a unit suffix
(`500ms`, `30s`, `2m`); where a bare number is also accepted it is noted.

## General

The following environment variables are available for all running modes.

- `FOUNDATION_ENV`: Application (service) environment. Default: `development`. Possible values: `development`, `production`, `test`.
- `LOG_LEVEL`: Log level. Default: `INFO`.
- `LOG_PAYLOADS`: Log full gRPC request and response bodies at debug level. Default: `false`.
  A protobuf message printed whole contains whatever the caller sent — passwords,
  tokens, personal data — so this stays off unless you have decided the
  environment can hold that.
- `PORT`: Port to listen on (for server-based running modes). Default: `51051`.
- `SHUTDOWN_TIMEOUT`: How long the service spends finishing work in flight before it stops its components. Default: `30s`.
  Keep it below the supervisor's own grace period (Kubernetes'
  `terminationGracePeriodSeconds`), so the service gets to finish on its own terms.
- `DRAIN_DELAY`: How long the service keeps serving after it starts failing readiness, before it shuts anything down. Default: `0`.
  Set it to a few seconds to give load balancers time to notice before
  connections start being refused.
- `HEALTH_CHECK_TIMEOUT`: Budget for a single readiness probe. Default: `2s`.
- `FOUNDATION_SKIP_DOTENV`: Set to any value to stop Foundation reading a `.env` file at startup.
- `SENTRY_DSN`: The DSN for the Sentry service. Leave empty to disable Sentry.
- `SENTRY_ENVIRONMENT`: Environment reported to Sentry. Default: the value of `FOUNDATION_ENV`.
- `SENTRY_RELEASE`: Release reported to Sentry. Default: empty.
- `SENTRY_FLUSH_TIMEOUT`: How long the service waits for buffered Sentry events to be delivered before exiting. Default: `2s`.
- `REDIS_URL`: The URL of the Redis instance to use for caching or communicating with Redis. Leave empty to disable.
  Understood in full, including the database number in the path
  (`redis://host:6379/3`) and the `rediss://` scheme for TLS.

## HTTP servers

Applied to the gateway, the HTTP server mode and the metrics server.

- `HTTP_READ_HEADER_TIMEOUT`: How long a client may take to send its request headers. Default: `10s`.
  This is the setting that closes Slowloris; leave it set.
- `HTTP_IDLE_TIMEOUT`: How long an idle keep-alive connection is kept. Default: `120s`.
- `HTTP_READ_TIMEOUT`: Bounds reading the whole request, body included. Default: `0` (off).
  A limit here also caps how long a client may take to upload.
- `HTTP_WRITE_TIMEOUT`: Bounds writing the response. Default: `0` (off).
  A limit here cuts off long-running responses, including server-streaming
  methods exposed through the gateway. When responses are bounded, set it to
  comfortably more than `GatewayOptions.Timeout`.
- `HTTP_MAX_REQUEST_BODY_SIZE`: Maximum request body in bytes. Default: `33554432` (32 MiB), `0` disables the cap.
  grpc-gateway unmarshals the whole body into a protobuf message, so an
  unbounded body is an unbounded allocation. Applies to the gateway and the HTTP
  server mode; the metrics server serves no request bodies.

## Authentication

The following environment variables are only applicable when using authentication.

- `HYDRA_ADMIN_URL`: The URL of the Hydra Admin API. Required for the `hydra` authentication provider.
- `HYDRA_TIMEOUT`: Bounds a single introspection call. Default: `5s`.
- `HYDRA_INTROSPECTION_CACHE_TTL`: How long an introspection result is reused. Default: `30s`, `0` disables caching.
  Caching means a revoked token keeps working until the entry expires. An entry
  never outlives the token's own `exp`, whatever the TTL is.

> **Trust boundary.** The gateway derives `X-Authenticated`, `X-User-Id`,
> `X-Client-Id`, `X-Scope` and `X-Metadata` from the authentication result and
> forwards them to downstream services, which trust them. It strips both those
> headers and their `Grpc-Metadata-` aliases from every incoming request, so a
> client cannot supply its own.
>
> Downstream gRPC services trust that metadata unconditionally. Anything that
> can reach a service's gRPC port can therefore claim any identity — restrict
> the port at the network level, use mTLS (`GRPC_TLS_DIR`), or both.

## Gateway

The following environment variables are only applicable when running in `gateway` mode.

- `GRPC_*_ENDPOINT`: The endpoint of the gRPC service. E.g. `GRPC_USERS_ENDPOINT` for the `users` service.

## Tracing

Applies to every running mode.

- `OTEL_EXPORTER_OTLP_ENDPOINT`: The base OTLP endpoint where OpenTelemetry traces will be exported to. Common values are `http://localhost:4317` for OTLP/gRPC and `http://localhost:4318` for OTLP HTTP/protobuf. Leave empty to disable tracing.
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`: Optional trace-specific OTLP endpoint. When set, this takes precedence over `OTEL_EXPORTER_OTLP_ENDPOINT`.
- `OTEL_EXPORTER_OTLP_PROTOCOL`: Optional OTLP transport selector. Supported values: `grpc`, `http/protobuf`. If unset, the transport is inferred from the endpoint.
- `OTEL_TRACES_SAMPLER_RATIO`: The sampling ratio for traces, between `0.0` and `1.0`. Default: `1.0`.
  The sampler is parent-based, so a service honours the decision made at the
  head of the trace rather than re-deciding and fragmenting it. A value outside
  the range falls back to the default.

## Events Worker

The following environment variables are only applicable when running in `events_worker` mode.

> Note: Do not forget to add the `cable_courier` service to your application in order to deliver events
> to the originators (users) via WebSockets.

- `EVENTS_WORKER_ERRORS_TOPIC`: The Kafka topic handler errors are published to. Default: empty, meaning the topic is derived from the error's proto name like any other event, which puts Foundation errors on `foundation.errors`.
- `EVENTS_WORKER_DELIVER_ERRORS`: Whether handler errors are published back to the originator of the request. Default: `true`.

## Outbox Courier

> **Ordering.** Several courier replicas can run at once — each takes a batch
> nobody else holds — but that costs ordering: a replica skips rows another has
> locked, so two events for the same key can reach Kafka out of order, on the
> same partition. Run a single courier where per-key ordering matters.

## Jobs Worker

The following environment variables are only applicable when running in `jobs_worker` mode.

- `REDIS_URL`: The Redis URL to use for the `gocraft_work` backend, e.g. `redis://localhost:6379`. Required.
- `REDIS_POOL`: The maximum number of active connections to the Redis instance. Default: `5`.
- `REDIS_NAMESPACE`: The namespace to use for the Redis keys. Default: `__foundation_jobs__`.

## Cable

The following environment variables are only applicable when running in `cable_grpc` or `cable_courier` mode.

- `ANYCABLE_REDIS_CHANNEL`: The Redis PubSub channel AnyCable listens on. Default: `__anycable__`.

## gRPC

- `GRPC_TLS_DIR`: The directory containing the TLS certificates for gRPC. Leave empty to disable mTLS.

  The same directory serves both sides, so a gateway and a gRPC server sharing
  it need all five files:

  | File         | Used by                                  |
  |--------------|------------------------------------------|
  | `ca.crt`     | both, to verify the other end            |
  | `server.crt` | the gRPC server (`grpc`, `cable_grpc`)   |
  | `server.key` | the gRPC server                          |
  | `client.crt` | the gateway, calling downstream services |
  | `client.key` | the gateway                              |

## Metrics

- `METRICS_ENABLED`: Whether to enable the server with `/health`, `/live`, `/ready` and `/metrics`. Default: `true`.
- `METRICS_PORT`: Port to expose the metrics server on. Default: `51077`.

The endpoints are:

| Path       | Meaning                                                                 |
|------------|-------------------------------------------------------------------------|
| `/live`    | The process is running. Point Kubernetes' **liveness** probe here.       |
| `/ready`   | Every component is healthy and the service is not shutting down. Point the **readiness** probe here. |
| `/health`  | The same as `/ready`; kept for existing manifests.                      |
| `/metrics` | Prometheus metrics.                                                     |

Pointing a liveness probe at a check that includes dependencies means a
database blip restarts every replica, which turns one outage into two. Use
`/live` for liveness.

## Kafka

- `KAFKA_BROKERS`: A comma-separated list of Kafka brokers to connect to. Must be set when using any of the Kafka features.
- `KAFKA_CONSUMER_GROUP`: The consumer group an events worker joins. Default: `<service name>-foundation`.
  Set it when one application runs several workers reading different topics:
  sharing a group makes them take partitions away from each other on every
  rebalance.
- `KAFKA_ALLOW_AUTO_TOPIC_CREATION`: Whether writing to an unknown topic creates it. Default: `true` outside production, `false` in production.
- `KAFKA_PRODUCER_BATCH_SIZE`: The maximum number of messages to batch before sending to Kafka. Default: `1`.
- `KAFKA_PRODUCER_BATCH_TIMEOUT`: How long the writer waits for a batch to fill before sending it. Default: `1s`. A bare number is read as seconds.
- `KAFKA_SASL_USERNAME`, `KAFKA_SASL_PASSWORD`: SASL credentials. Both must be set for SASL to be used.
- `KAFKA_SASL_PROTOCOL`: SASL mechanism. Supported values: `plain`, `scram-sha-512`.
  SASL/PLAIN without TLS sends the password verbatim; Foundation warns at
  startup when `KAFKA_TLS_DIR` is empty.
- `KAFKA_TLS_DIR`: The directory containing the TLS certificates for Kafka. Leave empty to disable TLS.
  The file names follow the Kubernetes TLS secret convention:
  - `ca.crt`: The CA certificate.
  - `tls.crt`: The client certificate.
  - `tls.key`: The client key.

## PostgreSQL

- `DATABASE_URL`: The URL of the PostgreSQL database. Must be set when using the PostgreSQL database.
- `DATABASE_POOL`: The maximum number of open connections to the database. Default: `5`.
