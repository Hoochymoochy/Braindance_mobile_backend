use redis::aio::MultiplexedConnection;
use redis::AsyncCommands;

const PRESENCE_PREFIX: &str = "user";
const PRESENCE_TTL_SECONDS: u64 = 30;

/// Set a user's online presence with a TTL.
/// Call this on a heartbeat interval (e.g. every 15s) to keep the key alive.
pub async fn heartbeat(
    conn: &mut MultiplexedConnection,
    user_id: &str,
    online: bool,
) -> redis::RedisResult<()> {
    let key = format!("{PRESENCE_PREFIX}:{user_id}:presence");
    let _: () = conn
        .set_ex(&key, if online { "1" } else { "0" }, PRESENCE_TTL_SECONDS)
        .await?;
    Ok(())
}

/// Check if a user is currently online.
pub async fn is_online(
    conn: &mut MultiplexedConnection,
    user_id: &str,
) -> redis::RedisResult<bool> {
    let key = format!("{PRESENCE_PREFIX}:{user_id}:presence");
    let val: Option<String> = conn.get(&key).await?;
    Ok(val.as_deref() == Some("1"))
}
