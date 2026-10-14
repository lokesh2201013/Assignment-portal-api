# Email Microservice

This Go/Fiber service provides user authentication, SMTP sender identities, templates, email delivery, and aggregate analytics. See [API and run guide](docs/README.md), [database reference](docs/database.md), and [OpenAPI 3 specification](docs/openapi.yaml).

Run from this directory with `go run .` after configuring the required values. HTTP defaults to port `3000`; gRPC defaults to `50051`. Live Swagger UI is served at `/docs`, and `/openapi.yaml` returns the embedded spec. Only `/health` and `/api/v1/auth/*` are public; remaining REST routes require the JWT from login.

The runtime initializes PostgreSQL tables on startup. Email delivery is synchronous; a durable queue and per-message delivery history are not currently implemented. Avoid using the older endpoint examples from archived documentation; the registered paths are documented in the OpenAPI contract.
