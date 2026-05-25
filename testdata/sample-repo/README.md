# Acme Orders

Acme Orders is a small HTTP service used as visualization fixture data.

The service exposes a health endpoint and an order endpoint. The architecture is intentionally plain: one Go module, one server entrypoint, and no external runtime dependencies.

## Runtime Shape

- `cmd/server/main.go` owns routing and process startup.
- `/healthz` returns service health.
- `/orders/{id}` returns a placeholder order response for a requested id.
