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
