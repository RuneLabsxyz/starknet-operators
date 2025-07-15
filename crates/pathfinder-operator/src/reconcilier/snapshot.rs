use async_trait::async_trait;
use k8s_openapi::api::batch::v1::{Job, JobSpec};
use k8s_openapi::api::core::v1::{
    Container, EnvVar, EphemeralContainer, EphemeralVolumeSource, PersistentVolumeClaim,
    PersistentVolumeClaimSpec, PersistentVolumeClaimTemplate, PersistentVolumeStatus, PodSpec,
    PodTemplateSpec, ResourceRequirements, Volume, VolumeResourceRequirements,
};
use kube::api::{DeleteParams, ObjectMeta, Patch, PatchParams, PostParams};
use kube::runtime::events::{Event, EventType};
use kube::{Api, Error, Resource, ResourceExt};
use serde_json::json;
use tracing::info;

use crate::crd::{StarknetNodeStatus, StarknetNodeStatusEnum, StarknetSnapshot};
use crate::k8s_helper::container::ContainerBuilder;
use crate::k8s_helper::metadata::ObjectMetaBuilder;
use crate::k8s_helper::resources::ResourceRequirementBuilder;
use crate::k8s_helper::volume::VolumeBuilder;
use crate::reconcilier::pvc::NodeSystem;
use crate::reconcilier::status::StarknetNodeStatusExt;
use crate::{Context, StarknetNode};

pub trait StarknetNodeSnapshot {
    async fn ensure_job_cleanup(&self, ctx: &Context) -> Result<(), kube::Error>;
    fn snapshot_info(&self) -> Option<StarknetSnapshot>;
    fn should_snapshot(&self, pvc: &PersistentVolumeClaim) -> bool;
    async fn get_or_create_job(
        &self,
        pvc: &PersistentVolumeClaim,
        ctx: &Context,
    ) -> Result<Job, kube::Error>;

    async fn get_job(&self, ctx: &Context) -> Result<Option<Job>, kube::Error>;

    async fn is_job_finished(&self, job: &Job, ctx: &Context) -> bool;

    async fn mark_snapshot_finished(&self, state: &Context) -> Result<(), Error>;
}

fn get_job_name(obj: &StarknetNode) -> String {
    obj.get_prefixed_name("restore-snapshot-job")
}

fn create_job(obj: &StarknetNode, pvc: &PersistentVolumeClaim) -> Job {
    let job_name = get_job_name(obj);
    info!(
        "Creating snapshot job ({}) for {}",
        job_name,
        obj.name_any()
    );

    let config = obj
        .snapshot_info()
        .expect("Should not get here without a snapshot info");

    Job {
        metadata: ObjectMetaBuilder::new()
            .name(job_name)
            .owned_by(obj.owner_ref(&()).unwrap())
            .namespace(obj.namespace().unwrap())
            .build(),
        spec: Some(JobSpec {
            template: PodTemplateSpec {
                spec: Some(PodSpec {
                    // If something fails, we will handle the failure ourselves.
                    restart_policy: Some("Never".to_string()),
                    containers: vec![
                        ContainerBuilder::new("snapshot-downloader")
                            // TODO: Make the image configurable
                            .image("ghcr.io/runelabsxyz/pathfinder-snapshotter:latest")
                            .with_env("PATHFINDER_NETWORK", &obj.spec.network)
                            .with_env("PATHFINDER_FILE_NAME", &config.file_name)
                            .with_env("PATHFINDER_CHECKSUM", &config.checksum)
                            .with_env_opt("PATHFINDER_DOWNLOAD_URL", &config.rsync_config)
                            .with_mount("snapshot-scratch", "/scratch")
                            .with_mount("data", "/data")
                            .into(),
                    ],
                    volumes: vec![
                        VolumeBuilder::new("snapshot-scratch")
                            .ephemeral(EphemeralVolumeSource {
                                volume_claim_template: Some(PersistentVolumeClaimTemplate {
                                    metadata: Some(
                                        ObjectMetaBuilder::new()
                                            .with_label("type", "pathfinder-snapshot-scratch")
                                            .into(),
                                    ),
                                    spec: PersistentVolumeClaimSpec {
                                        access_modes: Some(vec!["ReadWriteOnce".to_string()]),
                                        storage_class_name: config.storage.class.clone(),
                                        resources: VolumeResourceRequirements {
                                            requests: ResourceRequirementBuilder::new()
                                                .storage(config.storage.size.clone())
                                                .into(),
                                            ..Default::default()
                                        }
                                        .into(),
                                        ..Default::default()
                                    },
                                }),
                            })
                            .into(),
                        VolumeBuilder::new("data")
                            .volume_claim(&pvc.name_any())
                            .into(),
                    ]
                    .into(),
                    ..Default::default()
                }),
                ..Default::default()
            },
            ..Default::default()
        }),
        ..Default::default()
    }
}

impl StarknetNodeSnapshot for StarknetNode {
    fn snapshot_info(&self) -> Option<StarknetSnapshot> {
        self.spec.snapshot.clone()
    }

    fn should_snapshot(&self, pvc: &PersistentVolumeClaim) -> bool {
        // Only start snapshot if:
        // - The node has snapshot information
        // - The PVC is not annotated with info
        let has_snapshot_config = self.spec.snapshot.is_some();

        let has_already_snapshotted = self
            .status
            .as_ref()
            .map(|e| e.snapshot_restored)
            .unwrap_or(false);

        has_snapshot_config && !has_already_snapshotted
    }

    async fn get_or_create_job(
        &self,
        pvc: &PersistentVolumeClaim,
        ctx: &Context,
    ) -> Result<Job, kube::Error> {
        let job_api: Api<Job> = Api::default_namespaced(ctx.client.clone());

        // Check by name the job
        let job_name = get_job_name(self);
        let job = job_api.get_opt(&job_name).await?;

        match job {
            Some(job) => Ok(job),
            None => {
                let job = create_job(self, pvc);
                let job = job_api
                    .create(&PostParams::default(), &job)
                    .await
                    .map_err(Into::into);

                self.set_status(ctx, StarknetNodeStatusEnum::DownloadingSnapshot)
                    .await?;

                ctx.recorder
                    .publish(
                        &Event {
                            type_: EventType::Normal,
                            reason: "SnapshotRestoreStarted".into(),
                            note: Some(format!(
                                "Started snapshot restore for `{}`: {}",
                                self.name_any(),
                                job_name
                            )),
                            action: "Snapshot".into(),
                            secondary: None,
                        },
                        &self.object_ref(&()),
                    )
                    .await?;

                job
            }
        }
    }

    async fn is_job_finished(&self, job: &Job, _ctx: &Context) -> bool {
        job.status
            .as_ref()
            .and_then(|e| e.conditions.as_ref())
            .map(|e| {
                e.iter()
                    .any(|c| c.type_ == "Complete" && c.status == "True")
            })
            .unwrap_or(false)
    }

    async fn mark_snapshot_finished(&self, state: &Context) -> Result<(), Error> {
        // Modify the status to indicate that the snapshot is finished
        let status_patch = json!({ "status": { "snapshot_restored": true } });

        let api = Api::<StarknetNode>::namespaced(state.client.clone(), &self.namespace().unwrap());

        api.patch_status(
            &self.name_unchecked(),
            &PatchParams::default(),
            &Patch::Merge(status_patch),
        )
        .await?;

        self.set_status(state, StarknetNodeStatusEnum::SnapshotDownloaded)
            .await?;

        info!("Added snapshot finished!");

        let oref = self.object_ref(&());

        state
            .recorder
            .publish(
                &Event {
                    type_: EventType::Normal,
                    reason: "SnapshotFinished".into(),
                    note: Some(format!("Snapshot finished for `{}`", self.name_any())),
                    action: "Snapshot".into(),
                    secondary: None,
                },
                &oref,
            )
            .await?;

        Ok(())
    }

    async fn get_job(&self, ctx: &Context) -> Result<Option<Job>, kube::Error> {
        let job_api: Api<Job> = Api::default_namespaced(ctx.client.clone());

        // Check by name the job
        let job_name = get_job_name(self);

        job_api.get_opt(&job_name).await
    }

    async fn ensure_job_cleanup(&self, ctx: &Context) -> Result<(), kube::Error> {
        let job_api: Api<Job> = Api::default_namespaced(ctx.client.clone());
        let job_name = get_job_name(self);

        if self.get_job(ctx).await?.is_some() {
            // Force a ttl cleanup
            job_api
                .patch(
                    &job_name,
                    &PatchParams::apply("pathfinder-operator"),
                    &Patch::Apply(json!({
                        "apiVersion": "batch/v1",
                        "kind": "Job",
                        "spec": {
                            "ttlSecondsAfterFinished": 1
                        }
                    })),
                )
                .await?;
            // The GC is going to do its job
        }

        Ok(())
    }
}
