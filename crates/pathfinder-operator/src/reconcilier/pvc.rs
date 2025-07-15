use std::collections::BTreeMap;

use async_trait::async_trait;
use k8s_openapi::api::core::v1::{
    PersistentVolumeClaim, PersistentVolumeClaimSpec, VolumeResourceRequirements,
};
use kube::{
    Api, Error, Resource, ResourceExt,
    api::{ListParams, ObjectMeta, Patch, PatchParams, PostParams},
    runtime::events::{Event, EventType},
};
use serde_json::json;
use tracing::{debug, info};

use crate::{
    StarknetNode,
    k8s_helper::{metadata::ObjectMetaBuilder, resources::ResourceRequirementBuilder},
    reconcilier::implementation::Context,
};

#[async_trait]
pub trait NodeSystem {
    /// Gets the prefixed name of a resource dependant on the node.
    fn get_prefixed_name(&self, value: &str) -> String;
    /// Gets the PersistentVolumeClaims associated with the node.
    async fn get_or_create_pvc(&self, state: &Context) -> Result<PersistentVolumeClaim, Error>;
}

#[async_trait]
impl NodeSystem for StarknetNode {
    fn get_prefixed_name(&self, value: &str) -> String {
        // TODO: Migrate this to a proper error, so as to not explode the runtime.
        return format!("{}-{}", self.name_any(), value);
    }
    async fn get_or_create_pvc(&self, state: &Context) -> Result<PersistentVolumeClaim, Error> {
        let pvc_name = self.get_prefixed_name("storage");

        // Find the PVCs associated with the node
        let pvc_api: Api<PersistentVolumeClaim> = Api::default_namespaced(state.client.clone());

        let pvc = match pvc_api.get_opt(&pvc_name).await? {
            Some(e) => e,
            None => {
                let pvc = create_pvc(self, state).await?;

                let pvc = pvc_api.create(&PostParams::default(), &pvc).await?;

                state
                    .recorder
                    .publish(
                        &Event {
                            type_: EventType::Normal,
                            reason: "Storage".into(),
                            note: Some(format!("Created storage for node `{}`", self.name_any())),
                            action: "CreatedPvc".into(),
                            secondary: None,
                        },
                        &self.object_ref(&()),
                    )
                    .await?;

                pvc
            }
        };

        // TODO: Patch the requested size, as the storage class cannot be changed
        let patched = json!({
            "spec": {
                "resources": {
                    "requests": {
                        "storage": self.spec.storage.size.clone(),
                    }
                }
            }
        });

        let pvc = pvc_api
            .patch(
                &pvc.name_any(),
                &PatchParams::default(),
                &Patch::Merge(&patched),
            )
            .await?;

        debug!("Updated PVC size");

        Ok(pvc)
    }
}

async fn create_pvc(
    object: &StarknetNode,
    state: &Context,
) -> Result<PersistentVolumeClaim, Error> {
    let name = object.get_prefixed_name("storage");
    info!("creating pvc: {}", name);
    // Create the PVC
    let pvc = PersistentVolumeClaim {
        metadata: ObjectMetaBuilder::new()
            .name(name)
            .owned_by(object.owner_ref(&()).unwrap())
            .namespace(object.namespace().unwrap())
            .build(),
        spec: Some(PersistentVolumeClaimSpec {
            access_modes: Some(vec!["ReadWriteOnce".to_string()]),
            storage_class_name: object.spec.storage.class.clone(),
            resources: Some(VolumeResourceRequirements {
                // Request the storage provided.
                requests: ResourceRequirementBuilder::new()
                    .storage(object.spec.storage.size.clone())
                    .into(),
                ..Default::default()
            }),
            ..Default::default()
        }),
        ..Default::default()
    };

    Ok(pvc)
}
