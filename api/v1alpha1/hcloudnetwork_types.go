package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudNetworkSpec defines the desired state of an HCloudNetwork.
// +kubebuilder:validation:XValidation:rule="self.ipRange == oldSelf.ipRange",message="ipRange is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.networkZones) || self.networkZones == oldSelf.networkZones",message="networkZones is immutable after creation"
type HCloudNetworkSpec struct {
	// IPRange is the IPv4 CIDR for the private network (e.g. 10.0.0.0/16).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=9
	IPRange string `json:"ipRange"`

	// NetworkZones lists Hetzner network zones where a Cloud subnet is created (e.g. eu-central, us-east, us-west).
	// Subnets let you attach Cloud Servers in those zones to this network. Leave empty for a network with only the main IP range.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	NetworkZones []string `json:"networkZones,omitempty"`

	// ExposeRoutesToVSwitch exposes routes to an attached vSwitch when applicable.
	// +optional
	ExposeRoutesToVSwitch bool `json:"exposeRoutesToVSwitch,omitempty"`

	// Labels to attach to the Hetzner Cloud network resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudNetworkStatus defines the observed state of an HCloudNetwork.
type HCloudNetworkStatus struct {
	// NetworkID is the Hetzner Cloud internal network ID.
	// +optional
	NetworkID int64 `json:"networkID,omitempty"`

	// IPRange is the CIDR observed in Hetzner (echo of allocation).
	// +optional
	IPRange string `json:"ipRange,omitempty"`

	// SubnetZones lists network zones where a Cloud subnet now exists on this network.
	// +optional
	SubnetZones []string `json:"subnetZones,omitempty"`

	// Conditions represent the latest available observations of the network.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcn
// +kubebuilder:printcolumn:name="IPRange",type=string,JSONPath=`.spec.ipRange`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudNetwork is the Schema for the hcloudnetworks API.
// It represents a Hetzner Cloud private Network managed by the hcloud-operator.
type HCloudNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudNetworkSpec   `json:"spec,omitempty"`
	Status HCloudNetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudNetworkList contains a list of HCloudNetwork.
type HCloudNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudNetwork{}, &HCloudNetworkList{})
}
