/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StorageTemplate struct {
	// size Is the size of the storage to use for the snapshot restore process.
	// Should be at least the double of the size of the snapshot file.
	// +required
	Size resource.Quantity `json:"size"`

	// storageClass Is the storage class to use for the snapshot restore process.
	//
	// If not set uses the default storage class.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type ArchiveSnapshot struct {
	// enable indicates if the archive restore process should be done or not.
	//
	// It is volountary that you need to set other values, as this is stongly discouraged!
	// (At least until the snapshot system is done)
	// +default true
	Enable bool `json:"enable,omitempty"`
	// fileName Is the name of the snapshot file to restore
	// +required
	FileName string `json:"fileName"` // TODO(Red): Make it optional, and fetch the data from the snapshot service
	// checksum Is the checksum of the snapshot file to restore
	// +required
	Checksum string `json:"checksum"` // TODO(Red): Make it optional, and fetch the data from the snapshot service
	// rsyncConfig Is the configuration for downloading the snapshot file
	//
	// If not set, the operator will use the default configuration as provided by the
	// [snapshot service](https://eqlabs.github.io/pathfinder/database-snapshots#rclone-configuration)
	// +optional
	RsyncConfig *string `json:"rsyncConfig,omitempty"`

	// restoreImage is the image going to be used for the restore process.
	//
	// If not set, the default image as configured by the service will be used
	// +optional
	RestoreImage *string `json:"restoreImage,omitempty"`
	// storage Is the storage configuration for the snapshot restore process
	//
	// Note that this storage is temporary, and will be deleted after the snapshot is restored to the main storage configuration
	Storage StorageTemplate `json:"storage"`
}

// StarknetRPCSpec defines the desired state of StarknetRPC.
type StarknetRPCSpec struct {
	// network The network the node will provide and connect to
	// +kubebuilder:validation:MinLength=0
	// +required
	Network string `json:"network"`

	// restoreArchive The archive snapshot restore information
	//
	// +required
	RestoreArchive ArchiveSnapshot `json:"restoreArchive"`

	// resources is the amount of resources dedicated to the StarknetRPC pod
	//
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// image is the image used for the RPC node itself
	//
	// Otherwise, it defaults to the latest tested version for the controller.
	Image *string `json:"image,omitempty"`

	// storage The storage configuration for the node
	Storage StorageTemplate `json:"storage"`

	// layer1RpcSecret Is the secret containing the Layer 1 RPC secret key
	// for synchronization
	Layer1RpcSecret corev1.SecretKeySelector `json:"layer1RpcSecret"`
}

// StarknetRPCStatus defines the observed state of StarknetRPC.
type StarknetRPCStatus struct {
	// conditions represent the current state of the StarknetRPC resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// StarknetRPC is the Schema for the starknetrpcs API.
type StarknetRPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StarknetRPCSpec   `json:"spec,omitempty"`
	Status StarknetRPCStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StarknetRPCList contains a list of StarknetRPC.
type StarknetRPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StarknetRPC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StarknetRPC{}, &StarknetRPCList{})
}
