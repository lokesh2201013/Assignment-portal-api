# Video Processing and Chat Service

## Run locally

Create a virtual environment, install dependencies, configure PostgreSQL/GCS/RabbitMQ, and start the app:

```powershell
python -m venv .venv
.\.venv\Scripts\python -m pip install -r requirements.txt
.\.venv\Scripts\python -m uvicorn app.app:app --host 0.0.0.0 --port 5000
```

FastAPI serves Swagger UI at `http://localhost:5000/docs`, ReDoc at `/redoc`, OpenAPI JSON at `/openapi.json`, and liveness at `/health`. The HTTP schema is generated from the active FastAPI routes. WebSocket endpoints are listed below because OpenAPI does not include WebSockets.

## HTTP and WebSocket routes

- `GET /video/serve/{video_id}` looks up video metadata and returns a signed GCS playback URL. The current route accepts an integer ID.
- `GET /chat/info`, `GET /api/stats`, and `GET /api/users` report chat state.
- `POST /api/broadcast?message=...&sender=...` broadcasts a system message.
- `DELETE /api/users/{target_user_id}` removes a connected user.
- `WS /ws/{user_id}/{username}` accepts JSON text frames. Send `{"type":"chat_message","message":"..."}` to broadcast a chat message or `{"type":"ping"}` for a pong response.

The broadcast and kick routes are not authenticated in the current implementation. Do not expose them publicly until admin authentication is added.

## Configuration and services

Settings are loaded from environment variables and `.env`: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_MIN_CONN`, `DB_MAX_CONN`, `BUCKET_NAME`, `GCS_SIGNED_URL_HOURS`, `RABBITMQ_URL`, `RABBITMQ_QUEUE`, `START_WORKER`, `CORS_ORIGINS`, `MAX_CHAT_USERS`, `LOG_LEVEL`, and `JAEGER_ENDPOINT`/`OTEL_EXPORTER_OTLP_ENDPOINT` (default collector base `http://localhost:4318`). Set Google Application Default Credentials for GCS access.

The background consumer expects integer video IDs on its durable RabbitMQ queue and calls the local FFmpeg processing controller. See [database setup](docs/database.md).

## Checks

```powershell
.\.venv\Scripts\python -m unittest discover -s tests -v
```
