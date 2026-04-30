package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudFloatingIPSpec defines the desired state of an HCloudFloatingIP.
// +kubebuilder:validation:XValidation:rule="self.type == oldSelf.type",message="type is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.location == oldSelf.location",message="location is immutable after creation"
type HCloudFloatingIPSpec struct {
	// Type is the floating IP address family: ipv4 or ipv6.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ipv4;ipv6
	Type string `json:"type"`

	// Location is the Hetzner location for this floating IP (e.g. fsn1).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Location string `json:"location"`

	// ServerRef points to an HCloudServer to assign this floating IP to.
	// +optional
	ServerRef *corev1.LocalObjectReference `json:"serverRef,omitempty"`

	// Description is stored on the Hetzner Cloud floating IP resource.
	// +optional
	Description string `json:"description,omitempty"`

	// DNSPtr sets reverse DNS for the floating IP address.
	// +optional
	DNSPtr *string `json:"dnsPtr,omitempty"`

	// Labels to attach to the Hetzner Cloud floating IP resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudFloatingIPStatus defines the observed state of an HCloudFloatingIP.
type HCloudFloatingIPStatus struct {
	// FloatingIPID is the Hetzner Cloud internal floating IP ID.
	// +optional
	FloatingIPID int64 `json:"floatingIPID,omitempty"`

	// IP is the allocated address observed in Hetzner.
	// +optional
	IP string `json:"ip,omitempty"`

	// Location is the home location observed in Hetzner.
	// +optional
	Location string `json:"location,omitempty"`

	// AppliedServerID is the Hetzner server ID this floating IP is assigned to.
	// +optional
	AppliedServerID int64 `json:"appliedServerID,omitempty"`

	// Conditions represent the latest available observations of the floating IP.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcfip
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.serverRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudFloatingIP is the Schema for the hcloudfloatingips API.
type HCloudFloatingIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudFloatingIPSpec   `json:"spec,omitempty"`
	Status HCloudFloatingIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudFloatingIPList contains a list of HCloudFloatingIP.
type HCloudFloatingIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudFloatingIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudFloatingIP{}, &HCloudFloatingIPList{})
}
