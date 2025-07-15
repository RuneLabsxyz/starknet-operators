pub mod implementation;
pub mod pod;
pub mod pvc;
pub mod snapshot;
pub mod status;
pub mod svc;

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Error while interracting with the kubernetes API")]
    KubeError(
        #[from]
        #[source]
        kube::Error,
    ),
    #[error("Recreated pod due to drift.")]
    RecreatedPod(),
}
