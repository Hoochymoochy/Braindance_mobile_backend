use axum::extract::ws::{Message, WebSocket};
use futures_util::{SinkExt, StreamExt};
use redis::aio::MultiplexedConnection;
use tracing::{debug, info};

/// Handle a WebSocket upgrade.
/// The client can send JSON commands and receive real-time events.
///
/// Supported inbound messages:
/// - `{"type": "subscribe", "channel": "nearby", "lat": 40.7, "lng": -74.0}`
/// - `{"type": "ping"}`
///
/// Outbound events:
/// - `{"type": "nearby_update", "users": [...]}`
/// - `{"type": "pong"}`
pub async fn handle_socket(socket: WebSocket, conn: MultiplexedConnection) {
    let (mut sender, mut receiver) = socket.split();
    let conn = std::sync::Arc::new(tokio::sync::Mutex::new(conn));

    info!("WebSocket connection established");

    // Process messages inline.
    // In production, spawn a dedicated task per subscription with a broadcast channel.
    while let Some(Ok(msg)) = receiver.next().await {
        match msg {
            Message::Text(text) => {
                debug!("Received: {}", text);

                // Parse the inbound command
                let parsed: Result<serde_json::Value, _> = serde_json::from_str(&text);
                if let Ok(cmd) = parsed {
                    let msg_type = cmd["type"].as_str().unwrap_or("");

                    match msg_type {
                        "ping" => {
                            let pong = r#"{"type":"pong"}"#;
                            let _ = sender.send(Message::Text(pong.into())).await;
                        }
                        "subscribe" => {
                            // Acknowledge subscription
                            let ack = format!(
                                r#"{{"type":"subscribed","channel":"{}"}}"#,
                                cmd["channel"].as_str().unwrap_or("unknown")
                            );
                            let _ = sender.send(Message::Text(ack.into())).await;

                            // If subscribing to nearby, do an immediate lookup
                            if cmd["channel"].as_str() == Some("nearby") {
                                let mut c = conn.lock().await;
                                let nearby = crate::nearby::discovery::discover_nearby(
                                    &mut c,
                                    &crate::models::NearbyQuery {
                                        latitude: cmd["lat"].as_f64().unwrap_or(0.0),
                                        longitude: cmd["lng"].as_f64().unwrap_or(0.0),
                                        radius: 500.0,
                                        limit: 50,
                                    },
                                )
                                .await;

                                if let Ok(users) = nearby {
                                    let payload = serde_json::json!({
                                        "type": "nearby_update",
                                        "users": users,
                                    });
                                    let _ = sender
                                        .send(Message::Text(payload.to_string().into()))
                                        .await;
                                }
                            }
                        }
                        _ => {
                            let err = r#"{"type":"error","message":"unknown command"}"#;
                            let _ = sender.send(Message::Text(err.into())).await;
                        }
                    }
                }
            }
            Message::Close(_) => {
                info!("WebSocket closed");
                break;
            }
            _ => {}
        }
    }
}
