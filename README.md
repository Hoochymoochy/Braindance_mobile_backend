# 🧠 Braindance Nearby — Backend

A real-time music discovery platform that lets users see what people around them are listening to through an AR-powered social layer built on top of Spotify.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Mobile    │────▶│   Gateway    │────▶│  PostgreSQL │
│   / AR      │     │   (Go)       │     │  (persist)  │
│   Client    │     │  :8080       │     └─────────────┘
└─────────────┘     └──────┬───────┘
                            │
                     ┌──────▼───────┐
                     │    Redis     │
                     │  (hot data)  │
                     └──────────────┘
```

| Service  | Language | Port  | Responsibility |
|----------|----------|-------|----------------|
| **Gateway**  | Go       | 8080  | Auth, user management, Spotify API polling, location tracking, currently-playing cache |

---

## Project Structure

```
Backend/
├── README.md
├── docker-compose.yml              # PostgreSQL 17 + Redis 8
├── .env
├── .gitignore
│
├── gateway/                        # Go — Auth, Users, Location, Spotify
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── auth/spotify.go         # Spotify OAuth flow
│   │   ├── database/
│   │   │   ├── postgres.go         # PostgreSQL connection + queries
│   │   │   └── redis.go            # Redis connection + location/song ops
│   │   ├── handlers/
│   │   │   ├── auth.go             # /login, /callback
│   │   │   ├── user.go             # /me
│   │   │   ├── location.go         # /ws (WebSocket), /location (REST)
│   │   │   └── song.go             # /currently-playing
│   │   ├── models/models.go        # Shared types
│   │   └── spotify/client.go       # Spotify API (currently-playing)
│   ├── go.mod
│   └── go.sum
│
└── sql/                            # Database schema
    ├── 001_users.sql
    └── 002_tokens.sql
```

---

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- [Docker](https://www.docker.com/products/docker-desktop/) + Docker Compose
- A [Spotify Developer App](https://developer.spotify.com/dashboard) (for OAuth credentials)

### 1. Configure environment

```bash
cp .env.example .env   # or create .env manually
```

Required `.env` variables:

```env
# Spotify OAuth
SPOTIFY_CLIENT_ID=your_client_id
SPOTIFY_CLIENT_SECRET=your_client_secret
REDIRECT_URI=http://localhost:8080/callback

# PostgreSQL (optional — defaults below)
POSTGRES_USER=spotify
POSTGRES_PASSWORD=spotify
POSTGRES_DB=braindance
DATABASE_URL=postgres://spotify:spotify@localhost:5432/braindance?sslmode=disable

# Redis (optional — defaults to localhost:6379)
REDIS_ADDR=localhost:6379

# Gateway
GATEWAY_PORT=8080
```

### 2. Start infrastructure

```bash
docker compose up -d
# Starts PostgreSQL on :5432 and Redis on :6379
```

### 3. Run the Gateway

```bash
cd gateway
go run cmd/server/main.go
# Listening on :8080
```

### 4. Test the OAuth flow

Open `http://localhost:8080/login` in a browser — it redirects to Spotify, authenticates, and saves your user + tokens.

---

## API Reference

Base URL: `http://localhost:8080`

All endpoints that need a user identity accept `?spotify_id=<id>` as a query parameter. *(In production this will be replaced with a JWT/session token.)*

---

### `GET /login`

Initiate Spotify OAuth. Redirects the browser to Spotify's authorization page.

**Response:** `302 Found` → Spotify auth page

---

### `GET /callback`

Spotify OAuth callback. Handles the token exchange and persists the user.

**Query params:** `?code=<spotify_auth_code>`

<details>
<summary>Response (200)</summary>

```json
{
  "user": {
    "id": "0e3zr17a4l7gvh2jer3svq4tq",
    "display_name": "Khayd",
    "email": "khayd@example.com"
  },
  "access_token": "BQ...spotify-access-token...",
  "refresh_token": "AQ...spotify-refresh-token...",
  "expires_in": 3600
}
```
</details>

---

### `GET /me`

Get the authenticated user's profile from PostgreSQL.

```
GET /me?spotify_id=0e3zr17a4l7gvh2jer3svq4tq
```

<details>
<summary>Response (200)</summary>

```json
{
  "id": 1,
  "spotify_id": "0e3zr17a4l7gvh2jer3svq4tq",
  "display_name": "Khayd",
  "email": "khayd@example.com",
  "created_at": "2026-06-10T22:00:00Z"
}
```
</details>

| Status | Meaning |
|--------|---------|
| 200 | User found |
| 400 | Missing `spotify_id` |
| 404 | User not found |

---

### `GET /currently-playing`

Polls Spotify for the user's currently playing track, stores the result in Redis, and returns it.

```
GET /currently-playing?spotify_id=0e3zr17a4l7gvh2jer3svq4tq
```

**When listening to music:**

<details>
<summary>Response (200)</summary>

```json
{
  "id": "4cOdK2wGLETKBW3PvgPWqT",
  "name": "Blinding Lights",
  "artists": [
    {
      "id": "0fW8E0XdT2aG9a47h3e5aB",
      "name": "The Weeknd"
    }
  ],
  "album": {
    "id": "0P2DEzhB4FhJRTxI2Y6JHs",
    "name": "After Hours",
    "images": [
      {
        "url": "https://i.scdn.co/image/ab67616d0000b2738863bc11d2aa12b54f5aeb36",
        "height": 640,
        "width": 640
      }
    ]
  },
  "uri": "spotify:track:4cOdK2wGLETKBW3PvgPWqT"
}
```
</details>

**When not listening to anything:**

```
null
```

| Status | Meaning |
|--------|---------|
| 200 (JSON) | Track data if playing |
| 200 (`null`) | Nothing playing |
| 400 | Missing `spotify_id` |
| 401 | No valid Spotify token — user needs to re-auth |
| 404 | User not found in database |
| 502 | Spotify API error |

**Polling guidance:** Call this every 5 seconds to keep Redis in sync. The Redis key has a 5-second TTL, so if the client stops polling the song data disappears naturally.

---

### `GET /location`

Get a user's last-known position from Redis.

```
GET /location?spotify_id=0e3zr17a4l7gvh2jer3svq4tq
```

<details>
<summary>Response (200)</summary>

```json
{
  "spotify_id": "0e3zr17a4l7gvh2jer3svq4tq",
  "x": 1.5,
  "y": 2.3,
  "z": 0.0,
  "timestamp": "2026-06-10T22:45:00Z"
}
```
</details>

| Status | Meaning |
|--------|---------|
| 200 | Location found |
| 400 | Missing `spotify_id` |
| 404 | No location (expired or never set) |

---

### `WS /ws`

WebSocket endpoint for real-time location tracking. The phone opens a persistent connection and streams position data.

```
ws://localhost:8080/ws?spotify_id=0e3zr17a4l7gvh2jer3svq4tq
```

**1. Server sends handshake on connect:**

```json
{
  "type": "connected",
  "spotify_id": "0e3zr17a4l7gvh2jer3svq4tq",
  "message": "Location tracking active"
}
```

**2. Client sends position updates:**

```json
{"x": 1.5, "y": 2.3, "z": 0.0}
```

**3. Server echoes back the confirmed position:**

```json
{
  "type": "location_update",
  "spotify_id": "0e3zr17a4l7gvh2jer3svq4tq",
  "x": 1.5,
  "y": 2.3,
  "z": 0.0,
  "timestamp": "2026-06-10T22:45:05Z"
}
```

**4. On error:**

```json
{
  "type": "error",
  "message": "Failed to store location"
}
```

**5. On disconnect:** The user's location key is automatically removed from Redis.

**Mobile client example (JavaScript):**

```javascript
const ws = new WebSocket(`ws://<host>:8080/ws?spotify_id=${spotifyId}`);

ws.onopen = () => console.log('Location tracking active');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'location_update') {
    console.log(`Server confirmed: ${msg.x}, ${msg.y}, ${msg.z}`);
  }
};

// Send position every second from the device sensors
setInterval(() => {
  ws.send(JSON.stringify({ x: accelX, y: accelY, z: accelZ }));
}, 1000);
```

---

## Redis Key Reference

| Key | Type | TTL | Value |
|-----|------|-----|-------|
| `location:{spotify_id}` | JSON string | 2 min | `{"spotify_id":"...","x":1.5,"y":2.3,"z":0.0,"timestamp":"..."}` |
| `song:{spotify_id}` | JSON string | 5 sec | Track object from Spotify (see `/currently-playing` response) |

---

## Database (PostgreSQL)

| Table | Purpose |
|-------|---------|
| `users` | Spotify account linkage (`spotify_id` is UNIQUE) |
| `tokens` | OAuth access/refresh tokens per user |
| `listening_history` | Track play history |

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SPOTIFY_CLIENT_ID` | Yes | — | Spotify app client ID |
| `SPOTIFY_CLIENT_SECRET` | Yes | — | Spotify app client secret |
| `REDIRECT_URI` | Yes | — | OAuth callback URL |
| `DATABASE_URL` | No | built from `POSTGRES_*` | Full PostgreSQL connection string |
| `POSTGRES_USER` | No | — | PostgreSQL user |
| `POSTGRES_PASSWORD` | No | — | PostgreSQL password |
| `POSTGRES_DB` | No | — | PostgreSQL database name |
| `REDIS_ADDR` | No | `localhost:6379` | Redis host:port |
| `REDIS_PASSWORD` | No | — | Redis password (if any) |
| `GATEWAY_PORT` | No | `8080` | HTTP server port |

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| API | Go 1.26 (stdlib `net/http`) |
| Database | PostgreSQL 17 |
| Cache / Real-time | Redis 8 |
| Realtime Push | WebSockets (`gorilla/websocket`) |
| Auth | Spotify OAuth 2.0 |
| Infra | Docker Compose |
