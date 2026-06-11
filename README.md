# 🧠 Braindance Nearby — Backend

A real-time music discovery platform that lets users see what people around them are listening to through an AR-powered social layer built on top of Spotify.

## Architecture

Braindance uses a **dual-service backend** — each service is built with the language best suited to its workload:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Mobile    │────▶│   Gateway    │────▶│  PostgreSQL │
│   / AR      │     │   (Go)       │     │  (persist)  │
│   Client    │     │  :8080       │     └─────────────┘
└─────────────┘     └──────┬───────┘
                            │
                     ┌──────▼───────┐     ┌─────────────┐
                     │   Realtime   │────▶│    Redis    │
                     │   (Rust)     │     │  (hot data) │
                     │  :3000       │     └─────────────┘
                     └──────────────┘
```

| Service  | Language | Port  | Responsibility |
|----------|----------|-------|----------------|
| **Gateway**  | Go       | 8080  | Auth, user management, data persistence, Spotify API |
| **Realtime** | Rust     | 3000  | Location updates, nearby discovery, WebSockets, caching |

### Why two services?

**Go (Gateway)** — Lower traffic, I/O-bound work. Go's lightweight goroutines and excellent standard library make it ideal for OAuth flows, database writes, and Spotify API polling. The concurrency model handles hundreds of polling goroutines without complexity.

**Rust (Realtime)** — Low-latency, CPU-efficient work. The AR client needs sub-100ms responses for nearby lookups. Rust's zero-cost abstractions, no-GC guarantees, and `axum`/`tokio` stack deliver predictable p99 latency under load. Redis GEO operations and WebSocket fan-out stay fast even as active users scale.

---

## Project Structure

```
Backend/
├── README.md
├── docker-compose.yml              # PostgreSQL 17 + Redis 8
├── .env.example                    # Environment variable template
├── .gitignore
│
├── gateway/                        # Go — Auth, Users, Persistence
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── auth/spotify.go         # Spotify OAuth flow
│   │   ├── database/
│   │   │   ├── postgres.go         # Connection pool + queries
│   │   │   └── migrations/         # SQL migration files
│   │   ├── handlers/
│   │   │   ├── auth.go             # /login, /callback
│   │   │   └── user.go             # /me, /history
│   │   ├── models/models.go        # Shared types
│   │   └── spotify/client.go       # Spotify API (currently-playing, top tracks)
│   ├── go.mod
│   └── go.sum
│
├── realtime/                       # Rust — Low-latency real-time ops
│   ├── Cargo.toml
│   └── src/
│       ├── main.rs                 # Axum server entry
│       ├── config.rs               # Environment config
│       ├── redis/
│       │   ├── mod.rs
│       │   ├── presence.rs         # User heartbeat / online status
│       │   ├── location.rs         # GEOADD, GEOSEARCH
│       │   └── cache.rs            # Current track cache
│       ├── nearby/
│       │   ├── mod.rs
│       │   └── discovery.rs        # Nearby listener lookup
│       ├── ws/
│       │   ├── mod.rs
│       │   └── handler.rs          # WebSocket upgrade + events
│       └── models.rs               # Request/response types
│
└── sql/                            # Shared schema reference
    ├── 001_users.sql
    └── 002_tokens.sql
```

---

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- [Rust](https://rustup.rs/) 1.85+ (edition 2024)
- [Docker](https://www.docker.com/products/docker-desktop/) + Docker Compose
- A [Spotify Developer App](https://developer.spotify.com/dashboard) (for OAuth credentials)

### 1. Clone & configure

```bash
git clone <repo-url> && cd Backend
cp .env.example .env
# Edit .env with your Spotify Client ID, Secret, and Redirect URI
```

### 2. Start infrastructure

```bash
docker compose up -d
# Starts PostgreSQL on :5432 and Redis on :6379
```

### 3. Run the Gateway (Go)

```bash
cd gateway
cp ../.env .env
go run cmd/server/main.go
# Listening on :8080
```

### 4. Run the Realtime service (Rust)

```bash
cd realtime
cargo run
# Listening on :3000
```

### 5. Test the OAuth flow

Open `http://localhost:8080/` — click "Login with Spotify" to test the full OAuth flow.

---

## Data Flow

### Authentication
```
Client ──GET /login──▶ Gateway ──302──▶ Spotify Auth
                ◀──callback── Gateway ◀──token exchange── Spotify
                     │
                     ▼
                PostgreSQL (users + tokens)
```

### Location & Nearby Discovery
```
Client ──POST /location──▶ Realtime ──GEOADD──▶ Redis
Client ──GET /nearby─────▶ Realtime ──GEOSEARCH──▶ Redis
                                          │
                                          ▼
                                   nearby_users + tracks
```

### Real-time Updates (WebSocket)
```
Client ──wss://──▶ Realtime
                     │
                     ├── subscribe: presence channel
                     ├── subscribe: nearby changes
                     └── push: new track detected
```

---

## API Overview

### Gateway (Go) — `:8080`

| Method | Path        | Description                      |
|--------|-------------|----------------------------------|
| GET    | `/`         | Home page with login button      |
| GET    | `/login`    | Initiate Spotify OAuth           |
| GET    | `/callback` | Spotify OAuth callback           |
| GET    | `/me`       | Get current user (auth required) |
| GET    | `/history`  | Get listening history            |

### Realtime (Rust) — `:3000`

| Method | Path              | Description                         |
|--------|-------------------|-------------------------------------|
| POST   | `/location`       | Update user location (lat, lng)     |
| GET    | `/nearby?radius=` | Discover nearby listeners + tracks  |
| GET    | `/presence/:id`   | Check user online status            |
| WS     | `/ws`             | WebSocket for real-time events      |

---

## Database

### PostgreSQL (long-term state)

| Table              | Purpose                        |
|--------------------|--------------------------------|
| `users`            | Spotify account linkage        |
| `tokens`           | OAuth access/refresh tokens    |
| `listening_history`| Track play history             |
| `music_milestones` | User listening achievements    |

### Redis (real-time state)

| Key Pattern           | Type     | Purpose                     |
|-----------------------|----------|-----------------------------|
| `user:{id}:presence`  | String   | Online heartbeat (TTL 30s)  |
| `user:{id}:track`     | JSON     | Currently playing song      |
| `users:locations`     | Geo Set  | `GEOADD users:locations lng lat user_id` |
| `user:{id}:session`   | Hash     | Active session metadata     |

---

## Environment Variables

| Variable              | Service  | Description                      |
|-----------------------|----------|----------------------------------|
| `SPOTIFY_CLIENT_ID`   | Gateway  | Spotify app client ID            |
| `SPOTIFY_CLIENT_SECRET`| Gateway | Spotify app client secret        |
| `REDIRECT_URI`        | Gateway  | OAuth callback URL               |
| `DATABASE_URL`        | Gateway  | PostgreSQL connection string     |
| `REDIS_URL`           | Both     | Redis connection string          |
| `REALTIME_PORT`       | Realtime | Server port (default: 3000)      |
| `GATEWAY_PORT`        | Gateway  | Server port (default: 8080)      |

---

## Future Features (planned)

- **Music Growth Timeline** — track taste evolution over time (first techno track, genre transitions, most influential song)
- **Music Identity Graph** — visualize listening journey as a network of songs, artists, and genres
- **Community Discovery** — find nearby people with similar taste, discover local music communities
- **Listening Milestones** — celebrate achievements (1000th track, most replayed artist, genre phases)

---

## Tech Stack

| Layer          | Technology                        |
|----------------|-----------------------------------|
| Gateway API    | Go 1.26 (stdlib `net/http`)       |
| Realtime API   | Rust (axum + tokio)               |
| Primary DB     | PostgreSQL 17                     |
| Cache / Geo    | Redis 8                           |
| Realtime Push | WebSockets (axum websocket)       |
| Auth           | Spotify OAuth 2.0 (PKCE-ready)    |
| Infra          | Docker Compose                    |
