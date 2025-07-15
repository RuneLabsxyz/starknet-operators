use k8s_openapi::{
    api::core::v1::{
        EnvVarSource, PersistentVolumeClaim, PersistentVolumeClaimVolumeSource, Pod,
        PodSecurityContext, PodSpec, Probe, SecurityContext, Volume,
    },
    apimachinery::pkg::util::intstr::IntOrString,
};
use kube::{
    Api, Resource, ResourceExt,
    api::{DeleteParams, Patch, PatchParams, PostParams},
    runtime::events::{Event, EventType},
};
use tracing::info;

use crate::reconcilier::Error;

use crate::{
    Context, StarknetNode,
    drift::{Diff, compare_values},
    k8s_helper::{container::ContainerBuilder, metadata::ObjectMetaBuilder},
};

pub trait StarknetNodePod {
    fn get_pod_name(&self) -> String;
    async fn get_or_create_pod(
        &self,
        ctx: &Context,
        pvc: &PersistentVolumeClaim,
    ) -> Result<Pod, Error>;
}

fn create_pod_def(node: &StarknetNode, pvc: &PersistentVolumeClaim) -> Pod {
    Pod {
        metadata: ObjectMetaBuilder::new()
            .name(node.get_pod_name())
            .namespace(node.namespace().unwrap())
            .owned_by(node.owner_ref(&()).unwrap())
            .into(),
        spec: Some(PodSpec {
            containers: vec![
                ContainerBuilder::new("pathfinder")
                    // TODO: Make this configurable
                    .image("eqlabs/pathfinder:latest")
                    .pull_policy("IfNotPresent")
                    .with_env("RUST_LOG", "info")
                    .with_env("PATHFINDER_DATA_DIR", "/usr/share/pathfinder/data")
                    .with_env("PATHFINDER_MONITOR_ADDRESS", "0.0.0.0:9000")
                    .with_env_from(
                        "PATHFINDER_ETHEREUM_API_URL",
                        EnvVarSource {
                            secret_key_ref: Some(node.spec.l1_rpc_secret.clone()),
                            ..Default::default()
                        },
                    )
                    .with_resources(node.spec.resources.clone())
                    .with_port("rpc", 9545)
                    .with_port("monitoring", 9000)
                    .with_mount("pathfinder-data", "/usr/share/pathfinder/data") /*.with_readiness_probe(Probe {
                        http_get: Some(k8s_openapi::api::core::v1::HTTPGetAction {
                            path: Some("/ready".to_string()),
                            port: IntOrString::Int(9000),
                            scheme: Some("HTTP".to_string()),
                            ..Default::default()
                        }),
                        initial_delay_seconds: Some(10),
                        period_seconds: Some(5),
                        ..Default::default()
                    })
                    .with_liveness_probe(Probe {
                        http_get: Some(k8s_openapi::api::core::v1::HTTPGetAction {
                            path: Some("/health".to_string()),
                            port: IntOrString::Int(9000),
                            scheme: Some("HTTP".to_string()),
                            ..Default::default()
                        }),
                        initial_delay_seconds: Some(10),
                        period_seconds: Some(5),
                        ..Default::default()
                    })*/
                    .into(),
            ],
            volumes: Some(vec![Volume {
                name: "pathfinder-data".to_string(),
                persistent_volume_claim: Some(PersistentVolumeClaimVolumeSource {
                    claim_name: pvc.name_unchecked().to_string(),
                    read_only: None,
                    ..Default::default()
                }),
                ..Default::default()
            }]),
            // Force security context to make it work
            security_context: Some(PodSecurityContext {
                run_as_user: Some(1000),
                run_as_group: Some(1000),
                fs_group: Some(1000),
                ..Default::default()
            }),
            ..Default::default()
        }),
        ..Default::default()
    }
}

impl StarknetNodePod for StarknetNode {
    fn get_pod_name(&self) -> String {
        format!("{}-node", self.name_any())
    }

    async fn get_or_create_pod(
        &self,
        ctx: &Context,
        pvc: &PersistentVolumeClaim,
    ) -> Result<Pod, Error> {
        let api = Api::<Pod>::namespaced(ctx.client.clone(), &self.namespace().unwrap());

        let pod = match api.get_opt(&self.get_pod_name()).await? {
            Some(pod) => {
                // First of all, check if the pod is trying to still be deleted
                if pod.metadata.deletion_timestamp.is_some() {
                    return Err(Error::RecreatedPod());
                }

                // Check if it has drifted.
                let current = serde_json::to_value(&pod).unwrap();
                let desired = serde_json::to_value(create_pod_def(self, pvc)).unwrap();
                let diff = compare_values(&current, &desired);

                if diff.non_empty() {
                    info!("Recreating Pod because of diff: {}", diff);
                    // delete the pod.
                    api.delete(&self.get_pod_name(), &DeleteParams::default())
                        .await?;

                    return Err(Error::RecreatedPod());
                }
                pod
            }
            None => {
                let pod = create_pod_def(self, pvc);

                // Create the pod
                api.create(&PostParams::default(), &pod).await?;

                // Create an event
                ctx.recorder
                    .publish(
                        &Event {
                            type_: EventType::Normal,
                            reason: "Pod".into(),
                            note: Some(format!(
                                "Created pod for `{}`: {}",
                                self.name_any(),
                                pod.name_any()
                            )),
                            action: "CreatedPod".into(),
                            secondary: None,
                        },
                        &self.object_ref(&()),
                    )
                    .await?;

                pod
            }
        };

        Ok(pod)
    }
}
