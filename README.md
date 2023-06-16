# Foundation

> **Note:** Foundation is currently in an early development stage and not ready for production use.

Foundation is a Go framework designed for developing microservices.

It leverages the following technologies:

- **gRPC**: An open-source, high-performance RPC (Remote Procedure Call) framework.
- **gRPC Gateway**: A gRPC to JSON reverse proxy.
- **Kafka**: A distributed streaming platform.
- **PostgreSQL**: A powerful, open-source relational database system.
- **WebSockets**: A protocol enabling bi-directional, persistent communication channels over TCP connections.

The primary objective of Foundation is to offer a simple and user-friendly set of tools for creating scalable, resilient, and highly available applications.

## Features

- [ ] **Health Checks**: An interface for exposing the application's health to Kubernetes.
- [ ] **Multiple Running Modes**: Foundation supports various running modes to cater to different application requirements: `gateway`, `grpc`, `http`, `worker`, or `job`.
- [ ] **Tracing**: Trace your requests and log them in a structured format using OpenTracing.
- [ ] Wide range of different helpers for common tasks.
- [x] **Graceful Shutdown**: gracefully shutdown your application when receiving a `SIGTERM` signal.
- [x] **Metrics**: A metrics interface for collecting application metrics and exposing them to Prometheus.
- [x] **Transactional Outbox**: An implementation of the transactional outbox pattern, facilitating transactional message publishing to Kafka.
- [x] **Unified Logging**: A unified logging interface (via `logrus`), enabling convenient colored logging during development and structured logging in production.

## Out of the Box Integrations

- [ ] **Kafka**: Integration for consuming and producing messages with Kafka.
- [ ] **Sentry**: Integration for reporting errors to Sentry.
- [x] **Dotenv**: Integration for loading environment variables from .env files.
- [x] **gRPC Gateway**: Integration for exposing gRPC services as JSON endpoints.
- [x] **ORY Hydra**: Integration for authenticating users on a gateway with ORY Hydra.
- [x] **ORY Kratos**: Integration for authenticating users on a gateway with ORY Kratos.
- [x] **PostgreSQL**: Integration for connecting to a PostgreSQL database.

## Getting Started

Currently, the most straightforward way to get started is to review the library's code or explore some pre-implemented services (not publicly available yet, sorry!).
