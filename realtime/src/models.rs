use serde::{Deserialize, Serialize};

// ── Location ─────────────────────────────────────────

#[derive(Debug, Deserialize)]
pub struct LocationUpdate {
    pub user_id: String,
    pub latitude: f64,
    pub longitude: f64,
}

// ── Nearby ───────────────────────────────────────────

#[derive(Debug, Deserialize)]
pub struct NearbyQuery {
    pub latitude: f64,
    pub longitude: f64,
    /// Search radius in meters (default: 500)
    #[serde(default = "default_radius")]
    pub radius: f64,
    /// Max results to return (default: 50)
    #[serde(default = "default_limit")]
    pub limit: usize,
}

fn default_radius() -> f64 {
    500.0
}
fn default_limit() -> usize {
    50
}

#[derive(Debug, Serialize)]
pub struct NearbyUser {
    pub user_id: String,
    pub distance_meters: f64,
    pub track: Option<CurrentTrack>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CurrentTrack {
    pub track_name: String,
    pub artist_name: String,
    pub album_name: String,
    pub album_art_url: Option<String>,
    pub is_playing: bool,
}

#[derive(Debug, Deserialize)]
pub struct TrackUpdate {
    pub user_id: String,
    pub track: CurrentTrack,
}

// ── Presence ─────────────────────────────────────────

#[derive(Debug, Deserialize)]
pub struct PresenceUpdate {
    pub user_id: String,
    #[serde(default = "default_online")]
    pub online: bool,
}

fn default_online() -> bool {
    true
}

// ── Cache ────────────────────────────────────────────

#[derive(Debug, Serialize, Deserialize)]
pub struct TrackCacheEntry {
    pub track_name: String,
    pub artist_name: String,
    pub album_name: String,
    pub album_art_url: Option<String>,
    pub is_playing: bool,
    pub cached_at: i64,
}
