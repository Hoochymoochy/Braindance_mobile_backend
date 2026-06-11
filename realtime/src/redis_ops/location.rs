use redis::aio::MultiplexedConnection;
use redis::AsyncCommands;

const LOCATION_KEY: &str = "users:locations";

/// Update a user's location in the geospatial index.
/// Uses GEOADD — coordinates are (longitude, latitude) per Redis convention.
pub async fn update_location(
    conn: &mut MultiplexedConnection,
    user_id: &str,
    longitude: f64,
    latitude: f64,
) -> redis::RedisResult<()> {
    let _: () = conn
        .geo_add(LOCATION_KEY, (longitude, latitude, user_id))
        .await?;
    Ok(())
}

/// Find nearby users within a given radius (meters) around a point.
/// Returns a list of (user_id, distance_in_meters).
pub async fn nearby_users(
    conn: &mut MultiplexedConnection,
    longitude: f64,
    latitude: f64,
    radius_meters: f64,
    limit: usize,
) -> redis::RedisResult<Vec<(String, f64)>> {
    // GEOSEARCH with BYRADIUS — returns members within the radius
    let results: Vec<(String, f64)> = redis::cmd("GEOSEARCH")
        .arg(LOCATION_KEY)
        .arg("FROMLONLAT")
        .arg(longitude)
        .arg(latitude)
        .arg("BYRADIUS")
        .arg(radius_meters)
        .arg("m")
        .arg("WITHDIST")
        .arg("ASC")
        .arg("COUNT")
        .arg(limit)
        .query_async(conn)
        .await?;

    Ok(results)
}
