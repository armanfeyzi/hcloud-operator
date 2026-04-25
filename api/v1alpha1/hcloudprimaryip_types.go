package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudPrimaryIPSpec defines the desired state of an HCloudPrimaryIP.
// +kubebuilder:validation:XValidation:rule="self.type == oldSelf.type",message="type is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.datacenter == oldSelf.datacenter",message="datacenter is immutable after creation"
type HCloudPrimaryIPSpec struct {
	// Type is the primary IP address family: ipv4 or ipv6.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ipv4;ipv6
	Type string `json:"type"`

	// Datacenter is the Hetzner datacenter for this primary IP (e.g. fsn1-dc14).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Datacenter string `json:"datacenter"`

	// ServerRef points to an HCloudServer to assign this primary IP to.
	// +optional
	ServerRef *corev1.LocalObjectReference `json:"serverRef,omitempty"`

	// AutoDelete deletes the primary IP when the assignee is deleted.
	// +optional
	AutoDelete *bool `json:"autoDelete,omitempty"`

	// DNSPtr sets reverse DNS for the primary IP address.
	// +optional
	DNSPtr *string `json:"dnsPtr,omitempty"`

	// Labels to attach to the Hetzner Cloud primary IP resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudPrimaryIPStatus defines the observed state of an HCloudPrimaryIP.
type HCloudPrimaryIPStatus struct {
	// PrimaryIPID is the Hetzner Cloud internal primary IP ID.
	// +optional
	PrimaryIPID int64 `json:"primaryIPID,omitempty"`

	// IP is the allocated address observed in Hetzner.
	// +optional
	IP string `json:"ip,omitempty"`

	// Datacenter is the datacenter observed in Hetzner.
	// +optional
	Datacenter string `json:"datacenter,omitempty"`

	// AppliedAssigneeID is the Hetzner assignee ID currently managed via spec.serverRef.
	// +optional
	AppliedAssigneeID int64 `json:"appliedAssigneeID,omitempty"`

	// Conditions represent the latest available observations of the primary IP.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcpip
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudPrimaryIP is the Schema for the hcloudprimaryips API.
type HCloudPrimaryIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudPrimaryIPSpec   `json:"spec,omitempty"`
	Status HCloudPrimaryIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudPrimaryIPList contains a list of HCloudPrimaryIP.
type HCloudPrimaryIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudPrimaryIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudPrimaryIP{}, &HCloudPrimaryIPList{})
}
