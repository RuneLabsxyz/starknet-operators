use std::{sync::Arc, time::Duration};

use crate::{Context, StarknetNode, reconcile, reconcilier::Error};
use futures::StreamExt;
use k8s_openapi::api::batch::v1::Job;
use k8s_openapi::api::core::v1::{PersistentVolumeClaim, Pod};
use kube::{
    Api, Client, ResourceExt,
    api::ListParams,
    runtime::{
        controller::{Action, Controller},
        events::{Event, EventType, Recorder, Reporter},
        finalizer::{Event as Finalizer, finalizer},
        reflector::Lookup,
        watcher::Config,
    },
};
use tracing::{error, info};

fn error_policy(object: Arc<StarknetNode>, err: &Error, _ctx: Arc<Context>) -> Action {
    error!("Error reconciling object {}: {:#?}", object.name_any(), err);
    Action::requeue(Duration::from_secs(5))
}

pub async fn controller() {
    let client = Client::try_default()
        .await
        .expect("failed to create kube Client");

    let reporter = Reporter {
        controller: "pathfinder-controller".into(),
        instance: std::env::var("CONTROLLER_POD_NAME").ok(),
    };

    let recorder = Recorder::new(client.clone(), reporter);

    let context = Arc::new(Context {
        client: client.clone(),
        recorder,
    });

    let nodes = Api::<StarknetNode>::all(client.clone());

    let pvcs = Api::<PersistentVolumeClaim>::all(client.clone());

    let pods = Api::<Pod>::all(client.clone());

    let jobs = Api::<Job>::all(client.clone());

    if let Err(e) = nodes.list(&ListParams::default().limit(1)).await {
        error!("CRD is not queryable; {e:?}. Is the CRD installed?");
        info!("Installation: cargo run --bin crdgen | kubectl apply -f -");
        std::process::exit(1);
    }
    Controller::new(nodes, Config::default().any_semantic())
        .owns(pvcs, Config::default())
        .owns(pods, Config::default())
        .owns(jobs, Config::default())
        .shutdown_on_signal()
        .run(reconcile, error_policy, context.clone())
        .filter_map(|x| async move { std::result::Result::ok(x) })
        .for_each(|_| futures::future::ready(()))
        .await;
}
