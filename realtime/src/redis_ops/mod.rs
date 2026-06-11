pub mod cache;
pub mod location;
pub mod presence;

use redis::aio::MultiplexedConnection;
use redis::{Client, RedisResult};

/// Create a Redis client and return a multiplexed async connection.
pub async fn connect(redis_url: &str) -> RedisResult<MultiplexedConnection> {
    let client = Client::open(redis_url)?;
    client.get_multiplexed_async_connection().await
}
