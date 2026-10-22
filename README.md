# Golang 9 Practice

<figure>
    <img width="300" src="docs/goodmorningvietnam.png"
         alt="Illustration for the payment retry and idempotency demo">
    <figcaption>Payment retries and idempotency</figcaption>
</figure>

## About

A Gin REST API for user registration, login, and profile management. It issues JWTs for authenticated requests and protects the user promotion endpoint with an admin-role check. The repository also includes a standalone payment simulator demonstrating retries with exponential backoff and jitter, plus idempotency keys that prevent duplicate work and replay a completed response.

## Project Structure

| Path | Purpose |
| --- | --- |
| `cmd/main.go` | Starts the REST API. |
| `cmd/payment-sim/` | Runs the retry and idempotency demos. |
| `internal/app/` | Loads configuration and wires the application. |
| `internal/controller/` | HTTP routes and user handlers. |
| `internal/service/` | User registration, login, and role operations. |
| `internal/repository/` | PostgreSQL user data access. |
| `internal/middlewares/` | JWT, role, rate-limit, and idempotency middleware. |
| `internal/payment/` | Payment handler and retry client. |
| `internal/config/` | Reads database environment variables. |
| `db/migrations/` | SQL files initialized by the PostgreSQL container. |

## Getting Started

### Docker Compose

Install Docker with Compose, then run these commands from the repository root:

```sh
cp .example.env .env
docker compose up --build
```

Compose reads `.example.env` directly for both containers. The API is available at `http://localhost:8080`; PostgreSQL is published on host port `5433` and listens on container port `5432`. The API connects to the database using the Compose service name `postgres`. The database health check must pass before the API starts.

To stop the services, run `docker compose down`. The database volume is retained. To remove it and re-run the initialization migrations on the next start, run `docker compose down -v` (this deletes the local database data).

### Local API

Install Go 1.25.5 or newer and have PostgreSQL running. For example, start only the database service with `docker compose up -d postgres`; its host port is `5433`. Copy the environment file and set `DB_HOST=localhost` in `.env` (the example file uses `postgres`, which is the hostname needed by the API container):

```sh
cp .example.env .env
go run ./cmd/main.go
```

The API listens on port `8080`. PostgreSQL listens on port `5433` on your host and `5432` inside Compose. The migrations in `db/migrations/` are mounted into PostgreSQL's initialization directory and run when its data volume is first created. To recreate a clean local database, stop Compose with `docker compose down -v` before starting it again.

### Run the payment demos

The simulator uses in-memory test servers and does not require PostgreSQL or the API to be running:

```sh
go run ./cmd/payment-sim all
# Or run just one mode:
go run ./cmd/payment-sim retry
go run ./cmd/payment-sim idempotency
```

## Environment Variables

Copy `.example.env` to `.env` for local execution. Replace the sample credentials and JWT secret before using the service outside a local development environment.

| Variable | Purpose | Example |
| --- | --- | --- |
| `DB_HOST` | PostgreSQL hostname; use `postgres` in Compose or `localhost` for a local API process. | `postgres` |
| `DB_PORT` | PostgreSQL port as seen by the application. | `5432` |
| `DB_NAME` | Application database name. | `golang_practice` |
| `DB_USER` | Application database user. | `app_user` |
| `DB_PASSWORD` | Application database password. | `change-me` |
| `DB_SSL` | PostgreSQL SSL mode (`disable` for the included local Compose database). | `disable` |
| `JWT_SECRET` | Secret used to sign and verify JWTs. | `replace-with-a-long-random-secret` |
| `POSTGRES_USER` | Initial PostgreSQL superuser created by the image. | `app_user` |
| `POSTGRES_PASSWORD` | Password for the initial PostgreSQL user. | `change-me` |
| `POSTGRES_DB` | Initial database created by the PostgreSQL image. | `golang_practice` |

For the included Compose setup, keep `DB_NAME` equal to `POSTGRES_DB`, `DB_USER` equal to `POSTGRES_USER`, and `DB_PASSWORD` equal to `POSTGRES_PASSWORD`.

## API Endpoints

The examples below assume the API is running at `http://localhost:8080` and use `curl` with a JSON request body. Register first, then log in and copy the returned token into `TOKEN`. Replace `USER_ID` with the `id` returned by registration when promoting a user.

| Method | Path | Authentication | Success |
| --- | --- | --- | --- |
| `POST` | `/register` | None | `201 Created` |
| `POST` | `/login` | None | `200 OK` with a JWT |
| `GET` | `/users/me` | Bearer JWT | `200 OK` with the current user |
| `PATCH` | `/users/promote/:id` | Bearer JWT with `admin` role | `200 OK` with the promoted user |

### Register

Username, valid email, and a password of at least six characters are required. New accounts receive the `user` role.

```sh
curl -i -X POST http://localhost:8080/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com","password":"secret123"}'
```

The response contains the new user's `id`, for example `"id":"550e8400-e29b-41d4-a716-446655440000"`.

### Log in

```sh
curl -i -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}'
```

On success, the response body contains `{"token":"..."}`. Set `TOKEN` to that token in your shell before making authenticated requests.

### Get the current user

```sh
curl -i http://localhost:8080/users/me \
  -H "Authorization: Bearer $TOKEN"
```

### Promote a user

Promotion requires a valid JWT whose role is already `admin`. Registration always creates a regular user, so an operator must provision the first admin role directly in the database before logging in as that account (for example, `UPDATE users SET role = 'admin' WHERE username = 'alice';`):

```sh
curl -i -X PATCH http://localhost:8080/users/promote/USER_ID \
  -H "Authorization: Bearer $TOKEN"
```

Replace `USER_ID` in the URL with the target user's UUID. A regular user's token receives `403 Forbidden`.

## Rate Limiting

The middleware allows up to five requests per minute for each authenticated user. Requests without a valid user identity are limited by client IP. The limit applies across API routes in this process. Once exceeded, the API returns HTTP `429 Too Many Requests` with `{"error":"too many requests"}`. The counter is in memory and resets when its one-minute window expires or the process restarts.

## Payment Simulator

The standalone simulator demonstrates two modes; it is not an HTTP payment endpoint exposed by the main API.

### Retry mode

`go run ./cmd/payment-sim retry` starts a local test server that returns `503 Service Unavailable` three times and then succeeds. The retry client retries network timeouts and HTTP `429`, `500`, `502`, `503`, or `504` responses. It does not retry cancellation, deadline expiration, `401`, or `404`. The configured maximum counts total attempts, including the first request. Between retryable failures it uses exponential backoff based on a 500 ms default, capped by attempt, with random jitter from zero up to that cap.

Representative output (the delay values vary because of jitter):

```text
Running retry demo
Attempt 1 failed: status 503 Service Unavailable; waiting 321ms...
Attempt 2 failed: status 503 Service Unavailable; waiting 701ms...
Attempt 3 failed: status 503 Service Unavailable; waiting 1.4s...
Attempt 4: Success!
Retry demo status: 200 OK
Retry demo response: {"status": "success"}
```

### Idempotency mode

`go run ./cmd/payment-sim idempotency` launches an in-memory test endpoint and sends eight concurrent payment requests using the same `Idempotency-Key`. The first request for a key starts processing. Concurrent requests with that key receive HTTP `409 Conflict` while it is processing. After completion, the same key returns the stored status code and response body without running the handler again. The key is required; a request without it receives HTTP `400 Bad Request`. Use a new key for a distinct payment.

Representative output (which concurrent request wins can vary):

```text
Running idempotency demo
Processing started
Request 2: 409 Conflict (already processing)
Request 3: 409 Conflict (already processing)
Request 1: 200 OK {"status":"paid","amount":1000,"transaction_id":"uuid-..."}
Replay request: 200 {"status":"paid","amount":1000,"transaction_id":"uuid-..."}
Replay matched stored response
```
