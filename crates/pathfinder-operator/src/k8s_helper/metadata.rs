use std::collections::BTreeMap;

use k8s_openapi::apimachinery::pkg::apis::meta::v1::OwnerReference;
use kube::api::ObjectMeta;

pub struct ObjectMetaBuilder {
    inner: ObjectMeta,
}

impl ObjectMetaBuilder {
    pub fn new() -> Self {
        Self {
            inner: Default::default(),
        }
    }

    pub fn name(mut self, name: String) -> Self {
        self.inner.name = Some(name);
        self
    }

    pub fn namespace<T: ToString>(mut self, namespace: T) -> Self {
        self.inner.namespace = Some(namespace.to_string());
        self
    }

    pub fn owned_by(mut self, oref: OwnerReference) -> Self {
        self.inner.owner_references = Some(vec![oref]);
        self
    }

    pub fn with_label<T: ToString, U: ToString>(mut self, label: T, value: U) -> Self {
        let mut labels = self.inner.labels.unwrap_or_else(|| BTreeMap::new());

        labels.insert(label.to_string(), value.to_string());

        self.inner.labels = Some(labels);
        self
    }

    pub fn with_annotation(mut self, label: &str, value: &str) -> Self {
        let mut annotations = self.inner.annotations.unwrap_or_else(|| BTreeMap::new());

        annotations.insert(label.to_string(), value.to_string());

        self.inner.annotations = Some(annotations);
        self
    }

    pub fn build(&self) -> ObjectMeta {
        self.inner.clone()
    }
}

impl Into<ObjectMeta> for ObjectMetaBuilder {
    fn into(self) -> ObjectMeta {
        self.build()
    }
}

pub trait ObjectMetaBuilderExt: Sized {
    fn get_meta(&mut self) -> &mut ObjectMeta;

    fn name(&mut self, name: String) -> &mut Self {
        self.get_meta().name = Some(name);
        self
    }

    fn namespace<T: ToString>(mut self, namespace: T) -> Self {
        self.get_meta().namespace = Some(namespace.to_string());
        self
    }

    fn owned_by(mut self, oref: OwnerReference) -> Self {
        self.get_meta().owner_references = Some(vec![oref]);
        self
    }

    fn with_label<T: ToString, U: ToString>(mut self, label: T, value: U) -> Self {
        let mut labels = self.get_meta().labels.take().unwrap_or_else(BTreeMap::new);

        labels.insert(label.to_string(), value.to_string());

        self.get_meta().labels = Some(labels);
        self
    }

    fn with_annotation(mut self, label: String, value: String) -> Self {
        let mut annotations = self
            .get_meta()
            .annotations
            .take()
            .unwrap_or_else(BTreeMap::new);

        annotations.insert(label, value);

        self.get_meta().annotations = Some(annotations);
        self
    }
}
