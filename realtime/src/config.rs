use std::env;

pub struct Config {
    pub redis_url: String,
    pub port: u16,
}

impl Config {
    pub fn from_env() -> Self {
        let redis_url = env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".into());
        let port = env::var("REALTIME_PORT")
            .ok()
            .and_then(|p| p.parse().ok())
            .unwrap_or(3000);

        Self { redis_url, port }
    }
}
