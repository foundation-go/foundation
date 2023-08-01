# Available environment variables

## General

The following environment variables are available for all running modes.

- `APP_ENV`: Application environment. Default: `development`. Possible values: `development`, `production`.
- `LOG_LEVEL`: Log level. Default: `INFO`.
- `PORT`: Port to listen on. Default: `51051`.

## Authentication

The following environment variables are only applicable when using authentication.

- `HYDRA_ADMIN_URL`: The URL of the Hydra Admin API. Required for the `hydra` authentication provider.

## Gateway

The following environment variables are only applicable when running in `gateway` mode.

- `GRPC_*_ENDPOINT`: The endpoint of the gRPC service.

## Insight

- `INSIGHT_ENABLED`: Whether to enable the server with `/health` and `/metrics`. Default: `true`.
- `INSIGHT_PORT`: Port to expose health check on. Default: `51077`.

## Kafka

- `KAFKA_BROKERS`: A coma-separated list of Kafka brokers to connect to. Must be set when using any of the Kafka features.
- `KAFKA_CONSUMER_ENABLED`: Whether to enable the Kafka consumer. Default: `false`.
- `KAFKA_PRODUCER_ENABLED`: Whether to enable the Kafka producer. Default: `false`.

## Outbox

- `OUTBOX_ENABLED`: Whether to enable the outbox or publish messages directly to Kafka. Default: `false`.

## PostgreSQL

- `DATABASE_POOL`: The maximum number of open connections to the database. Default: `5`.
- `DATABASE_URL`: The URL of the PostgreSQL database. Must be set when using the PostgreSQL database.
