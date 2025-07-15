use kube::{
    Api, Error, ResourceExt,
    api::{Patch, PatchParams},
    core::Resource,
};
use serde_json::json;

use crate::{
    Context, StarknetNode,
    crd::{StarknetNodeStatus, StarknetNodeStatusEnum},
};

pub trait StarknetNodeStatusExt {
    async fn ensure_status(&self, ctx: &Context) -> Result<(), Error>;
    async fn set_status(&self, ctx: &Context, status: StarknetNodeStatusEnum) -> Result<(), Error>;
}

impl StarknetNodeStatusExt for StarknetNode {
    async fn ensure_status(&self, ctx: &Context) -> Result<(), Error> {
        let api = Api::<StarknetNode>::namespaced(ctx.client.clone(), &self.namespace().unwrap());

        if self.status.is_none() {
            api.patch_status(
                &self.name_unchecked(),
                &PatchParams::apply("pathfinder-operator"),
                &Patch::Apply(StarknetNode {
                    status: Some(StarknetNodeStatus {
                        snapshot_restored: false,
                        ..Default::default()
                    }),
                    ..Default::default()
                }),
            )
            .await?;
        }

        Ok(())
    }

    async fn set_status(&self, ctx: &Context, status: StarknetNodeStatusEnum) -> Result<(), Error> {
        let api = Api::<StarknetNode>::namespaced(ctx.client.clone(), &self.namespace().unwrap());

        api.patch_status(
            &self.name_unchecked(),
            &PatchParams::apply("pathfinder-operator").force(),
            &Patch::Apply(json!({
                "kind": Self::kind(&()),
                "apiVersion": Self::api_version(&()),
                "status": {
                    "snapshot_restored": self.status.as_ref().map_or(false, |status| status.snapshot_restored),
                    "status": status
                }
            })),
        )
        .await?;

        Ok(())
    }
}
