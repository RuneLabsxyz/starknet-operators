use axum::{Json, Router, routing::get};
use controller::controller;
use tokio::join;
use tracing::info;
use tracing_subscriber::{EnvFilter, Registry, layer::SubscriberExt, util::SubscriberInitExt};

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        use tokio::signal;

        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}

#[tokio::main]
async fn main() -> Result<(), kube::Error> {
    let logger = tracing_subscriber::fmt::layer().compact();
    let env_filter = EnvFilter::try_from_default_env()
        .or(EnvFilter::try_new("info"))
        .unwrap();

    let reg = Registry::default();
    reg.with(env_filter).with(logger).init();

    let controller = controller();

    // Also start a webserver for health checks and other metrics
    let app = Router::new()
        .route("/", get(|| async { "Hello, World!" }))
        .route("/health", get(|| async { Json("healthy") }));

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8080").await.unwrap();

    let serve = axum::serve(listener, app).with_graceful_shutdown(shutdown_signal());

    info!("Started listening on {:#?}", "0.0.0.0:8080");

    join!(controller, serve).1.unwrap();

    Ok(())
}
