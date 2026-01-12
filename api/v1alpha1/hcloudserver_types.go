package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudServerSpec defines the desired state of an HCloudServer.
// +kubebuilder:validation:XValidation:rule="self.image == oldSelf.image",message="image is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.location == oldSelf.location",message="location is immutable after creation"
type HCloudServerSpec struct {
	// ServerType is the Hetzner Cloud server type (e.g. cx21, cx31, cpx41).
	// May be changed after creation; the controller powers off the server, changes type via the API, then powers it on again.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerType string `json:"serverType"`

	// Image is the OS image to use (e.g. ubuntu-22.04, debian-12).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Location is the Hetzner datacenter location (e.g. fsn1, nbg1, hel1, ash, hil).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Location string `json:"location"`

	// Labels to attach to the Hetzner Cloud server resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// SSHKeys is a list of SSH key names or IDs to inject into the server on creation.
	// +optional
	SSHKeys []string `json:"sshKeys,omitempty"`

	// UserData is cloud-init user-data to pass to the server on first boot.
	// Must be a valid cloud-config or shell script. Applied once at creation time.
	// +optional
	// +kubebuilder:validation:MaxLength=32768
	UserData string `json:"userData,omitempty"`
}

// HCloudServerStatus defines the observed state of an HCloudServer.
type HCloudServerStatus struct {
	// ServerID is the Hetzner Cloud internal server ID.
	// +optional
	ServerID int64 `json:"serverID,omitempty"`

	// State is the current Hetzner server state (e.g. running, stopped, initializing).
	// +optional
	State string `json:"state,omitempty"`

	// AppliedServerType is the server type the controller has fully converged to (API type matches spec and server was running).
	// While spec.serverType differs from the value in Hetzner, or before this field is set after a type change, a resize may be in progress.
	// +optional
	AppliedServerType string `json:"appliedServerType,omitempty"`

	// PublicIPv4 is the server's public IPv4 address.
	// +optional
	PublicIPv4 string `json:"publicIPv4,omitempty"`

	// PublicIPv6 is the server's public IPv6 network prefix.
	// +optional
	PublicIPv6 string `json:"publicIPv6,omitempty"`

	// Conditions represent the latest available observations of the server's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcs
// +kubebuilder:printcolumn:name="ServerType",type=string,JSONPath=`.spec.serverType`
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.publicIPv4`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudServer is the Schema for the hcloudservers API.
// It represents a Hetzner Cloud virtual machine managed by the hcloud-operator.
type HCloudServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudServerSpec   `json:"spec,omitempty"`
	Status HCloudServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudServerList contains a list of HCloudServer.
type HCloudServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudServer{}, &HCloudServerList{})
}
