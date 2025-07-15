use std::collections::BTreeMap;

use k8s_openapi::apimachinery::pkg::api::resource::Quantity;

pub struct ResourceRequirementBuilder {
    inner: BTreeMap<String, Quantity>,
}
impl ResourceRequirementBuilder {
    pub fn new() -> Self {
        Self {
            inner: BTreeMap::new(),
        }
    }

    pub fn with_resource(mut self, name: String, quantity: Quantity) -> Self {
        self.inner.insert(name, quantity);
        self
    }

    pub fn storage(mut self, quantity: Quantity) -> Self {
        self.inner.insert("storage".to_string(), quantity);
        self
    }

    pub fn with_storage(self, quantity: impl ToString) -> Self {
        self.storage(Quantity(quantity.to_string()))
    }

    pub fn build(self) -> BTreeMap<String, Quantity> {
        self.inner
    }
}

impl Into<BTreeMap<String, Quantity>> for ResourceRequirementBuilder {
    fn into(self) -> BTreeMap<String, Quantity> {
        self.build()
    }
}

impl Into<Option<BTreeMap<String, Quantity>>> for ResourceRequirementBuilder {
    fn into(self) -> Option<BTreeMap<String, Quantity>> {
        Some(self.build())
    }
}
