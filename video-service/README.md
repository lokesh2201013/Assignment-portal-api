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
- `GET /api/messages?limit=50&before_id=...` returns persisted messages in chronological order; `before_id` loads an older page.
- `POST /api/broadcast?message=...&sender=...` persists and broadcasts a system message. Requires the `X-Admin-API-Key` header.
- `DELETE /api/users/{target_user_id}` removes a connected user. Requires the `X-Admin-API-Key` header.
- `WS /ws/{user_id}/{username}` accepts JSON text frames. Send `{"type":"chat_message","message":"..."}` to broadcast a chat message or `{"type":"ping"}` for a pong response.

User chat messages and system broadcasts are persisted to PostgreSQL. Apply `schema.sql` before using chat history. Set a strong, non-empty `ADMIN_API_KEY`; every `/admin/...` video management route and the chat broadcast/kick routes require it in `X-Admin-API-Key`. Missing or invalid credentials are rejected, and admin routes fail closed with `503` if the key is not configured. `X-User-Role` is not an authentication credential and never grants administrator access. WebSocket user IDs and usernames are currently client-supplied and are not authenticated, so deployments requiring verified identities must add token validation before exposing chat publicly.

## Configuration and services

Settings are read from environment variables: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_MIN_CONN`, `DB_MAX_CONN`, `BUCKET_NAME`, `GCS_SIGNED_URL_HOURS`, `RABBITMQ_URL`, `RABBITMQ_QUEUE`, `START_WORKER`, and `ADMIN_API_KEY`. The RabbitMQ consumer is disabled unless `START_WORKER=true`. Database and Google Cloud clients are created only when their services are used; importing the application does not connect to PostgreSQL, RabbitMQ, or GCS. Set Google Application Default Credentials for GCS access.

The background consumer expects integer video IDs on its durable RabbitMQ queue and calls the local FFmpeg processing controller. See [database setup](docs/database.md).

## Checks

```powershell
.\.venv\Scripts\python -m unittest discover -s tests -v
```
