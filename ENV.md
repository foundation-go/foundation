# Available environment variables

## General

The following environment variables are available for all running modes.

- `FOUNDATION_ENV`: Application (service) environment. Default: `development`. Possible values: `development`, `production`, `test`.
- `LOG_LEVEL`: Log level. Default: `INFO`.
- `PORT`: Port to listen on (for server-based running modes). Default: `51051`.

## Authentication

The following environment variables are only applicable when using authentication.

- `HYDRA_ADMIN_URL`: The URL of the Hydra Admin API. Required for the `hydra` authentication provider.

## Gateway

The following environment variables are only applicable when running in `gateway` mode.

- `GRPC_*_ENDPOINT`: The endpoint of the gRPC service. E.g. `GRPC_USERS_ENDPOINT` for the `users` service.

## Events Worker

The following environment variables are only applicable when running in `events_worker` mode.

> Note: Do not forget to add the `cable_courier` service to your application in order to deliver events
> to the originators (users) via WebSockets.

- `EVENTS_WORKER_ERRORS_TOPIC`: The Kafka topic to publish errors to. Default: `foundation.events_worker.errors`.
- `EVENTS_WORKER_DELIVER_ERRORS`: Whether to deliver errors to the `EVENTS_WORKER_ERRORS_TOPIC`. Default: `true`.

## Jobs Worker

The following environment variables are only applicable when running in `jobs_worker` mode.

- `JOBS_REDIS_URL`: The URL of the Redis instance to use for jobs. Must be set when using the `jobs_worker` mode.

## gRPC

- `GRPC_TLS_DIR`: The directory containing the TLS certificates for gRPC. Leave empty to disable TLS.
  The directory must contain the following files:
  - `ca.crt`: The CA certificate.
  - `client.crt`: The client certificate.
  - `client.key`: The client key.

## Metrics

- `METRICS_ENABLED`: Whether to enable the server with `/health` and `/metrics`. Default: `true`.
- `METRICS_PORT`: Port to expose metrics server on. Default: `51077`.

## Kafka

- `KAFKA_BROKERS`: A coma-separated list of Kafka brokers to connect to. Must be set when using any of the Kafka features.
- `KAFKA_TLS_DIR`: The directory containing the TLS certificates for Kafka. Leave empty to disable TLS.
  The directory must contain the following files:
  - `ca.crt`: The CA certificate.
  - `server.crt`: The client certificate.
  - `server.key`: The client key.

## PostgreSQL

- `DATABASE_POOL`: The maximum number of open connections to the database. Default: `5`.
- `DATABASE_URL`: The URL of the PostgreSQL database. Must be set when using the PostgreSQL database.
