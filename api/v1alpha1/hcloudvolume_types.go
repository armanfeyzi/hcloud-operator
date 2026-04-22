package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudVolumeSpec defines the desired state of an HCloudVolume.
// +kubebuilder:validation:XValidation:rule="self.size >= 10 && self.size <= 10240",message="size must be between 10 and 10240 GB"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.location) || !has(self.location) || self.location == oldSelf.location",message="location is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.size >= oldSelf.size",message="size can only be increased"
type HCloudVolumeSpec struct {
	// Size of the volume in GB (minimum 10, maximum 10240).
	// +kubebuilder:validation:Required
	Size int `json:"size"`

	// Location of the volume (e.g. fsn1, nbg1, hel1).
	// Required if ServerRef is not provided.
	// +optional
	Location string `json:"location,omitempty"`

	// ServerRef is a reference to an HCloudServer in the same namespace to attach this volume to.
	// +optional
	ServerRef *corev1.LocalObjectReference `json:"serverRef,omitempty"`

	// Format is the filesystem format to create on the volume (e.g., ext4, xfs).
	// If empty, the volume is not formatted.
	// +optional
	Format string `json:"format,omitempty"`

	// Labels to attach to the Hetzner Cloud volume.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudVolumeStatus defines the observed state of an HCloudVolume.
type HCloudVolumeStatus struct {
	// VolumeID is the Hetzner Cloud internal volume ID.
	// +optional
	VolumeID int64 `json:"volumeID,omitempty"`

	// State is the current Hetzner volume state (e.g. creating, available).
	// +optional
	State string `json:"state,omitempty"`

	// AttachedServerID is the Hetzner Cloud server ID this volume is currently attached to.
	// +optional
	AttachedServerID int64 `json:"attachedServerID,omitempty"`

	// LinuxDevice is the path to the device on the Linux server (e.g., /dev/disk/by-id/scsi-0HC_Volume_12345).
	// +optional
	LinuxDevice string `json:"linuxDevice,omitempty"`

	// AppliedSize is the volume size in GB last observed from Hetzner.
	// +optional
	AppliedSize int `json:"appliedSize,omitempty"`

	// Conditions represent the latest available observations of the volume's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcv
// +kubebuilder:printcolumn:name="Size",type=integer,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudVolume is the Schema for the hcloudvolumes API.
// It represents a Hetzner Cloud Volume managed by the hcloud-operator.
type HCloudVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudVolumeSpec   `json:"spec,omitempty"`
	Status HCloudVolumeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudVolumeList contains a list of HCloudVolume.
type HCloudVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudVolume{}, &HCloudVolumeList{})
}
