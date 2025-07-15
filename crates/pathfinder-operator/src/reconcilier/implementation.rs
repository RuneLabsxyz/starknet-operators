use std::{env::consts::OS, sync::Arc, time::Duration};

use kube::{
    Client, ResourceExt,
    runtime::{controller::Action, events::Recorder},
};

use crate::{
    StarknetNode,
    crd::StarknetNodeStatusEnum,
    reconcilier::{
        Error, pod::StarknetNodePod, pvc::NodeSystem, snapshot::StarknetNodeSnapshot,
        status::StarknetNodeStatusExt,
    },
};

#[derive(Clone)]
pub struct Context {
    pub client: Client,
    pub recorder: Recorder,
}
/// This is the main reconciliation loop.
///
pub async fn reconcile(obj: Arc<StarknetNode>, ctx: Arc<Context>) -> Result<Action, Error> {
    obj.ensure_status(&ctx).await?;

    // First of all, get the PVC
    let pvc = obj.get_or_create_pvc(&ctx).await?;

    // Check if it has the annotation (pathfinder.runelabs.xyz/snapshotted=true)
    if obj.should_snapshot(&pvc) {
        let job = obj.get_or_create_job(&pvc, &ctx).await?;

        if obj.is_job_finished(&job, &ctx).await {
            obj.mark_snapshot_finished(&ctx).await?;
        } else {
            // If the job is not finished, set the status to DownloadingSnapshot
            obj.set_status(&ctx, StarknetNodeStatusEnum::DownloadingSnapshot)
                .await?;
        }

        return Ok(Action::requeue(Duration::from_secs(10)));
    } else {
        // Make sure to cleanup the Job if it is finished
        obj.ensure_job_cleanup(&ctx).await?;

        // Let's get ready to deploy it
        obj.set_status(&ctx, StarknetNodeStatusEnum::Pending)
            .await?;
    }

    // Everything is ready to create the pod, so let's do it!
    let pod = match obj.get_or_create_pod(&ctx, &pvc).await {
        Ok(pod) => pod,
        // If the pod is being recreated, requeue after 10 seconds.
        Err(Error::RecreatedPod()) => return Ok(Action::requeue(Duration::from_secs(10))),
        Err(e) => return Err(e),
    };

    // TODO: Create the SVC, and we should be good to go!

    // For now, we just requeue after 10 seconds
    Ok(Action::requeue(Duration::from_secs(10)))
}
