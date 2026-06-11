mod config;
mod models;
mod nearby;
mod redis_ops;
mod ws;

use std::sync::Arc;

use axum::{
    extract::{Query, State, WebSocketUpgrade},
    http::StatusCode,
    response::{IntoResponse, Json},
    routing, Router,
};
use redis::aio::MultiplexedConnection;
use tokio::sync::Mutex;
use tower_http::cors::CorsLayer;
use tracing::info;

use crate::config::Config;
use crate::models::{LocationUpdate, NearbyQuery, PresenceUpdate, TrackUpdate};
use crate::nearby::discovery;
use crate::redis_ops as redis_mod;

/// Shared application state — a Redis connection pool accessible from all handlers.
struct AppState {
    redis: Mutex<MultiplexedConnection>,
}

#[tokio::main]
async fn main() {
    // Load environment
    dotenvy::dotenv().ok();

    // Initialize tracing
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "braindance_realtime=info,tower_http=info".into()),
        )
        .init();

    // Load config
    let config = Config::from_env();

    // Connect to Redis
    let redis_conn = redis_mod::connect(&config.redis_url)
        .await
        .expect("Failed to connect to Redis");

    info!("Connected to Redis at {}", config.redis_url);

    // Build shared state
    let state = Arc::new(AppState {
        redis: Mutex::new(redis_conn),
    });

    // Build router
    let app = Router::new()
        .route("/health", routing::get(health_check))
        .route("/location", routing::post(update_location))
        .route("/nearby", routing::get(get_nearby))
        .route("/presence", routing::post(update_presence))
        .route("/presence/:user_id", routing::get(get_presence))
        .route("/track", routing::post(update_track))
        .route("/ws", routing::get(ws_upgrade))
        .layer(CorsLayer::permissive())
        .with_state(state);

    // Bind and serve
    let addr = format!("0.0.0.0:{}", config.port);
    info!("Realtime server running on http://{}", addr);

    let listener = tokio::net::TcpListener::bind(&addr)
        .await
        .expect("Failed to bind");

    axum::serve(listener, app)
        .await
        .expect("Server error");
}

// ── Handlers ─────────────────────────────────────────

/// GET /health
async fn health_check() -> impl IntoResponse {
    Json(serde_json::json!({ "status": "ok" }))
}

/// POST /location — Update a user's geospatial position.
async fn update_location(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<LocationUpdate>,
) -> impl IntoResponse {
    let mut conn = state.redis.lock().await;
    match redis_mod::location::update_location(
        &mut conn,
        &payload.user_id,
        payload.longitude,
        payload.latitude,
    )
    .await
    {
        Ok(_) => StatusCode::OK.into_response(),
        Err(e) => {
            tracing::error!("Location update failed: {e}");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

/// GET /nearby?latitude=40.7&longitude=-74.0&radius=500&limit=50
async fn get_nearby(
    State(state): State<Arc<AppState>>,
    Query(query): Query<NearbyQuery>,
) -> impl IntoResponse {
    let mut conn = state.redis.lock().await;
    match discovery::discover_nearby(&mut conn, &query).await {
        Ok(users) => Json(serde_json::json!({
            "center": { "lat": query.latitude, "lng": query.longitude },
            "radius_meters": query.radius,
            "count": users.len(),
            "users": users,
        }))
        .into_response(),
        Err(e) => {
            tracing::error!("Nearby discovery failed: {e}");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

/// POST /presence — Heartbeat: mark a user as online or offline.
async fn update_presence(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<PresenceUpdate>,
) -> impl IntoResponse {
    let mut conn = state.redis.lock().await;
    match redis_mod::presence::heartbeat(&mut conn, &payload.user_id, payload.online).await {
        Ok(_) => StatusCode::OK.into_response(),
        Err(e) => {
            tracing::error!("Presence update failed: {e}");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

/// POST /track — Cache a user's currently playing track.
async fn update_track(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<TrackUpdate>,
) -> impl IntoResponse {
    let mut conn = state.redis.lock().await;
    match redis_mod::cache::set_current_track(&mut conn, &payload.user_id, &payload.track).await {
        Ok(_) => StatusCode::OK.into_response(),
        Err(e) => {
            tracing::error!("Track update failed: {e}");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

/// GET /presence/:user_id — Check if a user is online.
async fn get_presence(
    State(state): State<Arc<AppState>>,
    axum::extract::Path(user_id): axum::extract::Path<String>,
) -> impl IntoResponse {
    let mut conn = state.redis.lock().await;
    match redis_mod::presence::is_online(&mut conn, &user_id).await {
        Ok(online) => Json(serde_json::json!({ "user_id": user_id, "online": online }))
            .into_response(),
        Err(e) => {
            tracing::error!("Presence check failed: {e}");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

/// GET /ws — Upgrade to WebSocket for real-time events.
async fn ws_upgrade(
    State(state): State<Arc<AppState>>,
    ws: WebSocketUpgrade,
) -> impl IntoResponse {
    let conn = state.redis.lock().await;
    // Clone the underlying connection — MultiplexedConnection is Clone and all clones
    // share the same connection pool, so this is cheap.
    let conn_clone = conn.clone();
    drop(conn);

    ws.on_upgrade(move |socket| ws::handler::handle_socket(socket, conn_clone))
}
