package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudFirewallRule defines one inbound or outbound rule for a Hetzner Cloud Firewall.
type HCloudFirewallRule struct {
	// Direction is "in" or "out" (Hetzner Cloud firewall semantics).
	// +kubebuilder:validation:Enum=in;out
	// +kubebuilder:validation:Required
	Direction string `json:"direction"`

	// Protocol is tcp, udp, icmp, esp, or gre.
	// +kubebuilder:validation:Enum=tcp;udp;icmp;esp;gre
	// +kubebuilder:validation:Required
	Protocol string `json:"protocol"`

	// Port is a single port (e.g. "443"), a range ("8080-8090"), or omitted for ICMP / some protocols.
	// +optional
	Port *string `json:"port,omitempty"`

	// SourceIPs lists IPv4/IPv6 CIDR strings for matching source traffic (e.g. "0.0.0.0/0").
	// +optional
	SourceIPs []string `json:"sourceIPs,omitempty"`

	// DestinationIPs lists IPv4/IPv6 CIDR strings for outbound rules when applicable.
	// +optional
	DestinationIPs []string `json:"destinationIPs,omitempty"`

	// Description is an optional human-readable note stored in Hetzner Cloud.
	// +optional
	Description *string `json:"description,omitempty"`
}

// HCloudFirewallApplyTo selects which Cloud Servers receive this firewall (by Kubernetes refs and/or Hetzner label selector).
type HCloudFirewallApplyTo struct {
	// ServerRefs lists HCloudServer objects by name; the controller resolves status.serverID and applies the firewall to those servers.
	// +optional
	ServerRefs []corev1.LocalObjectReference `json:"serverRefs,omitempty"`

	// LabelSelector is a Hetzner Cloud server label selector string (e.g. "env=prod,app=my-app").
	// It matches labels on servers in Hetzner Cloud, not Kubernetes labels on CRs unless you sync them via HCloudServer.spec.labels.
	// +optional
	LabelSelector string `json:"labelSelector,omitempty"`
}

// HCloudFirewallSpec defines the desired state of an HCloudFirewall.
type HCloudFirewallSpec struct {
	// Labels are attached to the Hetzner Cloud firewall resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Rules is the ordered list of firewall rules (reconciled with Set Rules on change).
	// +optional
	Rules []HCloudFirewallRule `json:"rules,omitempty"`

	// ApplyTo configures server attachment. Omit to manage rules only (attach servers elsewhere / manually).
	// +optional
	ApplyTo *HCloudFirewallApplyTo `json:"applyTo,omitempty"`
}

// HCloudFirewallStatus defines the observed state of an HCloudFirewall.
type HCloudFirewallStatus struct {
	// FirewallID is the Hetzner Cloud firewall ID.
	// +optional
	FirewallID int64 `json:"firewallID,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcfw
// +kubebuilder:printcolumn:name="FirewallID",type=integer,JSONPath=`.status.firewallID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudFirewall is the Schema for the hcloudfirewalls API.
type HCloudFirewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudFirewallSpec   `json:"spec,omitempty"`
	Status HCloudFirewallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudFirewallList contains a list of HCloudFirewall.
type HCloudFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudFirewall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudFirewall{}, &HCloudFirewallList{})
}
