use k8s_openapi::api::core::v1::{
    EphemeralVolumeSource, PersistentVolumeClaimVolumeSource, Volume,
};

pub struct VolumeBuilder {
    inner: Volume,
}

impl VolumeBuilder {
    pub fn new(name: &str) -> Self {
        Self {
            inner: Volume {
                name: name.to_string(),
                ..Default::default()
            },
        }
    }

    pub fn volume_claim(mut self, claim_name: &str) -> Self {
        self.inner.persistent_volume_claim = Some(PersistentVolumeClaimVolumeSource {
            claim_name: claim_name.to_string(),
            ..Default::default()
        });
        self
    }

    pub fn ephemeral(mut self, spec: EphemeralVolumeSource) -> Self {
        self.inner.ephemeral = Some(spec);
        self
    }
}

impl Into<Volume> for VolumeBuilder {
    fn into(self) -> Volume {
        self.inner
    }
}
