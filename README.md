# Foundation

[![Go Build](https://github.com/foundation-go/foundation/actions/workflows/go.yml/badge.svg)](https://github.com/foundation-go/foundation/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/foundation-go/foundation)](https://goreportcard.com/report/github.com/foundation-go/foundation)
[![License](https://img.shields.io/github/license/foundation-go/foundation)](https://opensource.org/licenses/MIT)

> **Early Development Notice:** Foundation is currently in an early development stage. While you're welcome to explore and experiment, it's not yet ready for production use.

## 🔍 Overview

Foundation is a Go-based microservices framework aimed to help developers write scalable, resilient, and highly available applications with ease. By providing a cohesive set of well-chosen tools and features, Foundation aims to minimize boilerplate and allow developers to focus on writing business logic.

Foundation is built upon several proven technologies including:

- **gRPC**: A high-performance, open-source RPC framework.
- **gRPC Gateway**: A gRPC to JSON reverse proxy.
- **Protocol Buffers**: A language-neutral extensible mechanism for serializing structured data, used for gRPC and Kafka message serialization.
- **Kafka**: A powerful distributed streaming platform.
- **OAuth 2.0**: An industry-standard authorization framework.
- **PostgreSQL**: A robust open-source relational database system.
- **WebSockets**: Enabling real-time, bi-directional, and full-duplex communication channels over TCP connections.

## ⭐ Key Features

- 🌉 **Running Modes**: Adapt Foundation to cater to diverse operational requirements:
  - **Gateway Mode**: Facilitate the exposure of gRPC services as HTTP endpoints, leveraging gRPC Gateway. This mode acts as a bridge, allowing HTTP clients to communicate with your gRPC microservices transparently.
  - **gRPC Mode**: Operate as a standard gRPC server, enabling high-performance RPC communication, ideal for microservices interaction.
  - **HTTP Mode**: Deploy as a traditional HTTP server, offering a more general-purpose approach for serving web requests.
  - **Spin Worker Mode**: This is your background worker, designed to continuously execute tasks. It offers configurability in terms of processing functions and the interval between task iterations.
  - **Jobs Worker Mode**: A mode to run background jobs with Gocraft Work. Support scheduled jobs, retrying, and concurrency.
  - **Events Worker Mode**: Building on the Worker Mode, this variant is tailored for Kafka. It ingests messages from Kafka topics and triggers associated Go function handlers.
  - **Cable gRPC Mode**: Function as an AnyCable-compatible gRPC server, ideal for real-time WebSocket functionalities without sacrificing scalability.
  - **Cable Courier Mode**: This mode specializes in reading events from Kafka and then broadcasting them to Redis, readying the events for AnyCable processing. _Yeah, it would be much better if we could just use Kafka directly, but AnyCable doesn't support it._
  - **Outbox Courier Mode**: A mode to run a Kafka producer that reads messages from the database and publishes them to Kafka. _This is useful for implementing the transactional outbox pattern._
- 📬 **Transactional Outbox**: Implement the transactional outbox pattern for transactional message publishing to Kafka. Several courier replicas can run at once.
- ✏️ **Unified Logging**: Conveniently log with colors during development and structured logging in production using `logrus`.
- 🔍 **Tracing**: OpenTelemetry traces across every running mode, exported over OTLP, with parent-based sampling so a trace stays in one piece across services.
- 📊 **Metrics**: Prometheus metrics for HTTP and gRPC requests, event processing and lag, outbox depth, background jobs, and component health.
- 💓 **Health Check**: Separate liveness and readiness endpoints, with a draining phase on shutdown.
- 🔐 **(m)TLS**: TLS authentication for Kafka and mTLS for gRPC and AnyCable.
- ⏳ **Graceful Shutdown**: Clean shutdown on `SIGTERM`, bounded by `SHUTDOWN_TIMEOUT`, waiting for work in flight before closing anything.
- 🛠️ **Helpers**: A variety of helpers for common tasks.
- 🖥️ **CLI**: A CLI tool to help you get started and manage your project.

## 🔌 Integrations

Foundation comes with built-in support for:

- **PostgreSQL**: Easily connect to a PostgreSQL database.
- **Dotenv**: Load environment variables from .env files.
- **ORY Hydra**: Authenticate users on a gateway with ORY Hydra.
- **gRPC Gateway**: Expose gRPC services as JSON endpoints.
- **Kafka**: Produce and consume messages with Kafka (via `kafka-go`).
- **AnyCable**: Implement real-time WebSocket functionalities with AnyCable.
- **Gocraft Work**: Run background jobs with Gocraft Work.
- **Sentry**: Report errors to Sentry.

## 🚀 Getting Started

Currently, the best way to get started is by exploring the [examples](./examples) directory. There is an example application called `clubchat` that demonstrates how to use Foundation to create a simple event-driven microservices application.

## 🖥️ CLI Tool

To install the CLI tool, run:

```bash
go install github.com/foundation-go/foundation/cmd/foundation@main
```

There are several commands available:

```bash
foundation completion  # Generate shell completion scripts (prints to stdout)
foundation db:migrate  # Run database migrations
foundation db:rollback # Roll back database migrations
foundation db:force    # Set the migration version, clearing the dirty flag
foundation init-outbox # Copy the outbox migrations into this service
foundation start       # Start the service (you will be prompted to choose one)
foundation test        # Run tests
foundation new         # Create `--app` or `--service`
```

You can also run `foundation` without any arguments to see a list of available commands, or run `foundation <command> --help` to see the available options for a specific command.

## 🧪 Tests

```bash
make test              # unit suite, race detector on, no Docker needed
make test-integration  # against real Postgres, Redis and Kafka in containers
make lint
```

The [integration tests](./test/integration) verify what the unit tests can only
assume: that `FOR UPDATE SKIP LOCKED` really keeps two outbox couriers from
publishing the same event, that a consumer group's committed offset really
advances past unhandled messages, that a Redis URL's database number is really
honoured. They live in their own Go module so that testcontainers stays out of
the framework's dependency graph.

## ⬆️ Upgrading

Breaking changes and behaviour changes between releases are listed in
[MIGRATION.md](./MIGRATION.md).

## 🔒 Trust boundary

The gateway authenticates a request and forwards the caller's identity to
downstream services as `X-Authenticated`, `X-User-Id`, `X-Client-Id`, `X-Scope`
and `X-Metadata`. It strips those headers — and their `Grpc-Metadata-` aliases —
from every incoming request, so a client cannot supply its own.

Downstream gRPC services trust that metadata unconditionally: `fctx.GetUserID`
returns whatever arrived. Anything that can reach a service's gRPC port can
therefore claim any identity. Restrict the port at the network level, enable
mTLS with `GRPC_TLS_DIR`, or both.

## 🤝 Contributing

We're always looking for contributions from the community! If you've found a bug, have a suggestion, or want to add a new feature, feel free to open an issue or submit a pull request.

## 📜 License

Foundation is released under the [MIT License](./LICENSE).
