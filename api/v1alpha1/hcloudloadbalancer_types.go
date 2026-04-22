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

	// Services defines front-end listeners, back-end ports, and optional health checks.
	// Reconciled by listen port: services not listed are removed from Hetzner.
	// +optional
	// +listType=map
	// +listMapKey=listenPort
	Services []HCloudLoadBalancerServiceSpec `json:"services,omitempty"`
}

// HCloudLoadBalancerServiceSpec defines a load balancer service (listener + target port).
type HCloudLoadBalancerServiceSpec struct {
	// ListenPort is the public port the load balancer listens on.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort int32 `json:"listenPort"`

	// DestinationPort is the port on target servers.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	DestinationPort int32 `json:"destinationPort"`

	// Protocol is tcp, http, or https.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=tcp;http;https
	Protocol string `json:"protocol"`

	// Proxyprotocol enables PROXY protocol for this service.
	// +optional
	Proxyprotocol *bool `json:"proxyprotocol,omitempty"`

	// HealthCheck configures active health checking for this service.
	// +optional
	HealthCheck *HCloudLoadBalancerHealthCheckSpec `json:"healthCheck,omitempty"`
}

// HCloudLoadBalancerHealthCheckSpec configures a load balancer health check.
type HCloudLoadBalancerHealthCheckSpec struct {
	// Protocol is tcp, http, or https.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=tcp;http;https
	Protocol string `json:"protocol"`

	// Port is the health check port. Defaults to destinationPort when unset in Hetzner.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`

	// IntervalSeconds between health checks.
	// +optional
	// +kubebuilder:validation:Minimum=1
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`

	// TimeoutSeconds before a check is considered failed.
	// +optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// Retries before marking a target unhealthy.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Retries *int32 `json:"retries,omitempty"`

	// HTTP configures HTTP(S) health check options.
	// +optional
	HTTP *HCloudLoadBalancerHealthCheckHTTPSpec `json:"http,omitempty"`
}

// HCloudLoadBalancerHealthCheckHTTPSpec configures HTTP(S) health check paths and responses.
type HCloudLoadBalancerHealthCheckHTTPSpec struct {
	// Domain to send in the Host header.
	// +optional
	Domain *string `json:"domain,omitempty"`

	// Path to request.
	// +optional
	Path *string `json:"path,omitempty"`

	// Response substring to match in the response body.
	// +optional
	Response *string `json:"response,omitempty"`

	// StatusCodes lists acceptable HTTP status codes (e.g. "2??", "3??").
	// +optional
	StatusCodes []string `json:"statusCodes,omitempty"`

	// TLS enables TLS for the health check request.
	// +optional
	TLS *bool `json:"tls,omitempty"`
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
