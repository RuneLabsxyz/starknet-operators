use k8s_openapi::api::core::v1::{
    Container, ContainerPort, EnvFromSource, EnvVar, EnvVarSource, Probe, ResourceRequirements,
    VolumeMount,
};

pub struct ContainerBuilder {
    inner: Container,
}

impl ContainerBuilder {
    pub fn new<T: Into<String>>(name: T) -> Self {
        Self {
            inner: Container {
                name: name.into(),
                ..Default::default()
            },
        }
    }

    pub fn pull_policy(mut self, policy: &str) -> Self {
        self.inner.image_pull_policy = Some(policy.into());
        self
    }

    pub fn image<T: Into<String>>(mut self, image: T) -> Self {
        self.inner.image = Some(image.into());
        self
    }

    pub fn command(mut self, command: Vec<String>) -> Self {
        self.inner.command = Some(command);
        self
    }

    pub fn args(mut self, args: Vec<String>) -> Self {
        self.inner.args = Some(args);
        self
    }

    pub fn with_resources(mut self, resources: ResourceRequirements) -> Self {
        self.inner.resources = Some(resources);
        self
    }

    pub fn with_env_opt<T: ToString, U: ToString>(mut self, name: T, value: &Option<U>) -> Self {
        if value.is_none() {
            return self;
        }

        let env_var = EnvVar {
            name: name.to_string(),
            value: value.as_ref().map(|v| v.to_string()),
            ..Default::default()
        };
        let mut envs = self.inner.env.unwrap_or_else(|| vec![]);

        envs.push(env_var);

        self.inner.env = Some(envs);
        self
    }

    pub fn with_mount<T: ToString, U: ToString>(mut self, name: T, path: U) -> Self {
        let mount = VolumeMount {
            name: name.to_string(),
            mount_path: path.to_string(),

            ..Default::default()
        };
        let mut mounts = self.inner.volume_mounts.unwrap_or_else(|| vec![]);

        mounts.push(mount);

        self.inner.volume_mounts = Some(mounts);
        self
    }

    pub fn with_port(mut self, name: impl AsRef<str>, port: i32) -> Self {
        let port = ContainerPort {
            name: Some(name.as_ref().to_string()),
            container_port: port,
            ..Default::default()
        };
        let mut ports = self.inner.ports.unwrap_or_else(|| vec![]);

        ports.push(port);

        self.inner.ports = Some(ports);
        self
    }

    pub fn with_readiness_probe(mut self, probe: Probe) -> Self {
        self.inner.readiness_probe = Some(probe);

        self
    }

    pub fn with_liveness_probe(mut self, probe: Probe) -> Self {
        self.inner.liveness_probe = Some(probe);
        self
    }

    pub fn with_env<T: ToString, U: ToString>(mut self, name: T, value: U) -> Self {
        let env_var = EnvVar {
            name: name.to_string(),
            value: Some(value.to_string()),
            ..Default::default()
        };
        let mut envs = self.inner.env.unwrap_or_else(|| vec![]);

        envs.push(env_var);

        self.inner.env = Some(envs);
        self
    }

    pub fn with_env_from(mut self, name: impl AsRef<str>, r#ref: EnvVarSource) -> Self {
        let env_var = EnvVar {
            name: name.as_ref().to_string(),
            value_from: Some(r#ref),
            ..Default::default()
        };

        let mut envs = self.inner.env.unwrap_or_else(|| vec![]);

        envs.push(env_var);

        self.inner.env = Some(envs);
        self
    }
}

impl Into<Container> for ContainerBuilder {
    fn into(self) -> Container {
        self.inner
    }
}
