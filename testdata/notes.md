# Architecture Notes

Acme Orders is a single-process Go HTTP service.

The current runtime keeps routing inside `cmd/server/main.go`. The service has no database dependency in the fixture, so order responses are synthetic and deterministic.

Important visualization facts:

- Client traffic enters through the Go HTTP server.
- Health checks use `/healthz`.
- Order reads use `/orders/{id}`.
- The fixture should render as a small local service, not a distributed system.

Known limitations:

- Authentication is outside the fixture.
- Persistence is outside the fixture.
- The server uses standard library HTTP routing only.
