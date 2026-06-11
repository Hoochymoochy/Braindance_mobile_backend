use redis::aio::MultiplexedConnection;

use crate::models::{NearbyQuery, NearbyUser};
use crate::redis_ops::{cache, location, presence};

/// Discover nearby listeners and what they're playing.
///
/// Flow:
/// 1. GEOSEARCH for nearby user IDs within the radius.
/// 2. For each nearby user, check presence (online?) and fetch cached track.
/// 3. Return enriched results sorted by distance.
pub async fn discover_nearby(
    conn: &mut MultiplexedConnection,
    query: &NearbyQuery,
) -> redis::RedisResult<Vec<NearbyUser>> {
    // Step 1: Geospatial lookup
    let nearby = location::nearby_users(
        conn,
        query.longitude,
        query.latitude,
        query.radius,
        query.limit,
    )
    .await?;

    // Step 2: Enrich with presence + track data
    let mut results = Vec::with_capacity(nearby.len());

    for (user_id, distance) in nearby {
        let online = presence::is_online(conn, &user_id).await.unwrap_or(false);
        if !online {
            continue; // Skip offline users
        }

        let track = cache::get_current_track(conn, &user_id)
            .await
            .unwrap_or(None);

        results.push(NearbyUser {
            user_id,
            distance_meters: distance,
            track,
        });
    }

    Ok(results)
}
