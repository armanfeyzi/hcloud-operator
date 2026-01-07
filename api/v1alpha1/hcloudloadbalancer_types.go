package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudLoadBalancerSpec defines the desired state of an HCloudLoadBalancer.
// +kubebuilder:validation:XValidation:rule="!has(self.location) || !has(self.networkZone)",message="location and networkZone are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="self.loadBalancerType == oldSelf.loadBalancerType",message="loadBalancerType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.location) || !has(self.location) || self.location == oldSelf.location",message="location is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.networkZone) || !has(self.networkZone) || self.networkZone == oldSelf.networkZone",message="networkZone is immutable after creation"
type HCloudLoadBalancerSpec struct {
	// LoadBalancerType is the Hetzner load balancer type (e.g. lb11, lb21, lb31).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	LoadBalancerType string `json:"loadBalancerType"`

	// Location is the Hetzner datacenter location (e.g. fsn1, nbg1, hel1).
	// +optional
	Location string `json:"location,omitempty"`

	// NetworkZone is the Hetzner network zone (e.g. eu-central, us-east).
	// +optional
	NetworkZone string `json:"networkZone,omitempty"`

	// Algorithm selects the balancing algorithm. Supported values are
	// "round_robin" and "least_connections".
	// +optional
	// +kubebuilder:default:=round_robin
	// +kubebuilder:validation:Enum=round_robin;least_connections
	Algorithm string `json:"algorithm,omitempty"`

	// ServerSelector selects HCloudServer resources to attach as targets.
	// This matches the Kubernetes labels on HCloudServer objects.
	// +optional
	ServerSelector *metav1.LabelSelector `json:"serverSelector,omitempty"`

	// Labels to attach to the Hetzner Cloud load balancer resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudLoadBalancerStatus defines the observed state of an HCloudLoadBalancer.
type HCloudLoadBalancerStatus struct {
	// LoadBalancerID is the Hetzner Cloud internal load balancer ID.
	// +optional
	LoadBalancerID int64 `json:"loadBalancerID,omitempty"`

	// PublicIPv4 is the load balancer's public IPv4 address.
	// +optional
	PublicIPv4 string `json:"publicIPv4,omitempty"`

	// PublicIPv6 is the load balancer's public IPv6 address.
	// +optional
	PublicIPv6 string `json:"publicIPv6,omitempty"`

	// AttachedServerIDs are server IDs currently attached as targets.
	// +optional
	AttachedServerIDs []int64 `json:"attachedServerIDs,omitempty"`

	// Conditions represent the latest available observations of the load balancer's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hclb
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.loadBalancerType`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.publicIPv4`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudLoadBalancer is the Schema for the hcloudloadbalancers API.
type HCloudLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudLoadBalancerSpec   `json:"spec,omitempty"`
	Status HCloudLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudLoadBalancerList contains a list of HCloudLoadBalancer.
type HCloudLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudLoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudLoadBalancer{}, &HCloudLoadBalancerList{})
}
