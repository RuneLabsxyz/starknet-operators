use k8s_openapi::{
    api::core::v1::{ResourceRequirements, SecretKeySelector},
    apimachinery::pkg::api::resource::Quantity,
};
use kube::CustomResource;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

/// Our Document custom resource spec
#[derive(CustomResource, Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
#[kube(
    kind = "StarknetNode",
    group = "runelabs.xyz",
    version = "v1alpha1",
    namespaced,
    derive = "Default"
)]
#[kube(status = "StarknetNodeStatus")]
pub struct StarknetNodeSpec {
    /// The network used by the node
    pub network: String,
    /// The snapshot of the node resource (if set)
    pub snapshot: Option<StarknetSnapshot>,
    /// The secret that contains the RPC URL for L1 interactions
    pub l1_rpc_secret: SecretKeySelector,
    /// The allocated resources for the node
    pub resources: ResourceRequirements,
    // Disk size
    #[kube(validation = Rule::new("self.class == oldSelf.class").message("field is immutable"))]
    pub storage: Storage,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct Storage {
    pub size: Quantity,

    pub class: Option<String>,
}

/// The snapshot of the database, used to accelerate the catch-up process.
///
/// You can see the snapshot file name and checksum in the remote snapshot service:
/// [https://rpc.pathfinder.equilibrium.co/snapshots/latest](https://rpc.pathfinder.equilibrium.co/snapshots/latest)
///
/// In the future, we will support loading from a locally made snapshot.
#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct StarknetSnapshot {
    // The name of the snapshot file
    pub file_name: String,
    // The checksum of the snapshot file
    pub checksum: String,
    /// The rsync configuration for downloading the snapshot.
    ///
    /// Defaults to the configuration provided by the [snapshot service](https://eqlabs.github.io/pathfinder/database-snapshots#rclone-configuration)
    pub rsync_config: Option<String>,

    // The storage configuration for the snapshot.
    pub storage: Storage,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub enum StarknetNodeStatusEnum {
    /// The node is pending to be created.
    #[default]
    Pending,
    /// The node is downloading the snapshot from the remote snapshot service.
    DownloadingSnapshot,
    /// The snapshot has been downloaded, preparing to start the main pod.
    SnapshotDownloaded,
    /// The node is catching up with the network.
    CatchingUp,
    /// The node is ready to accept requests.
    Ready,
    /// An error occurred on this node, so it is not ready to accept requests.
    Failed,
}

#[derive(Deserialize, Serialize, Clone, Debug, JsonSchema, Default)]
pub struct StarknetNodeStatus {
    pub status: StarknetNodeStatusEnum,
    pub snapshot_restored: bool,
    pub head: Option<String>,
}
