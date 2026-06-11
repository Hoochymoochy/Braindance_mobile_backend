use redis::aio::MultiplexedConnection;
use redis::AsyncCommands;

use crate::models::{CurrentTrack, TrackCacheEntry};

const TRACK_PREFIX: &str = "user";
const TRACK_TTL_SECONDS: u64 = 120;

/// Cache a user's currently playing track.
pub async fn set_current_track(
    conn: &mut MultiplexedConnection,
    user_id: &str,
    track: &CurrentTrack,
) -> redis::RedisResult<()> {
    let key = format!("{TRACK_PREFIX}:{user_id}:track");

    let entry = TrackCacheEntry {
        track_name: track.track_name.clone(),
        artist_name: track.artist_name.clone(),
        album_name: track.album_name.clone(),
        album_art_url: track.album_art_url.clone(),
        is_playing: track.is_playing,
        cached_at: chrono::Utc::now().timestamp(),
    };

    let json = serde_json::to_string(&entry).unwrap_or_default();
    let _: () = conn.set_ex(&key, json, TRACK_TTL_SECONDS).await?;
    Ok(())
}

/// Retrieve a cached track for a user.
pub async fn get_current_track(
    conn: &mut MultiplexedConnection,
    user_id: &str,
) -> redis::RedisResult<Option<CurrentTrack>> {
    let key = format!("{TRACK_PREFIX}:{user_id}:track");
    let val: Option<String> = conn.get(&key).await?;

    Ok(val.and_then(|json| {
        serde_json::from_str::<TrackCacheEntry>(&json).ok().map(|e| CurrentTrack {
            track_name: e.track_name,
            artist_name: e.artist_name,
            album_name: e.album_name,
            album_art_url: e.album_art_url,
            is_playing: e.is_playing,
        })
    }))
}
