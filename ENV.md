# Available environment variables

## General

The following environment variables are available for all running modes.

- `APP_ENV`: Application environment. Default: `development`. Possible values: `development`, `production`.
- `LOG_LEVEL`: Log level. Default: `INFO`.
- `METRICS_PORT`: Port to expose metrics on. Default: `51077`.
- `PORT`: Port to listen on. Default: `51051`.

## Authentication

The following environment variables are only applicable when using authentication.

- `HYDRA_ADMIN_URL`: The URL of the Hydra Admin API. Required for the `hydra` authentication provider.
- `KRATOS_FRONTEND_URL`: The URL of the Kratos Public API. Required for the `kratos` authentication provider.

## Gateway

The following environment variables are only applicable when running in `gateway` mode.

- `GRPC_*_ENDPOINT`: The endpoint of the gRPC service.

## Metrics

- `METRICS_ENABLED`: Whether to enable Prometheus metrics endpoint. Default: `false`.

## Kafka

- `KAFKA_BROKERS`: A coma-separated list of Kafka brokers to connect to. Must be set when using any of the Kafka features.
- `KAFKA_CONSUMER_ENABLED`: Whether to enable the Kafka consumer. Default: `false`.
- `KAFKA_PRODUCER_ENABLED`: Whether to enable the Kafka producer. Default: `false`.

## PostgreSQL

- `DATABASE_ENABLED`: Whether to enable the PostgreSQL database. Default: `false`.
- `DATABASE_POOL`: The maximum number of open connections to the database. Default: `5`.
- `DATABASE_URL`: The URL of the PostgreSQL database. Must be set when using the PostgreSQL database.
