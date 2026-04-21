package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudPlacementGroupSpec defines the desired state of an HCloudPlacementGroup.
// +kubebuilder:validation:XValidation:rule="self.type == oldSelf.type",message="type is immutable after creation"
type HCloudPlacementGroupSpec struct {
	// Type is the Hetzner placement group strategy: spread or cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=spread;cluster
	Type string `json:"type"`

	// Labels to attach to the Hetzner Cloud placement group resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudPlacementGroupStatus defines the observed state of an HCloudPlacementGroup.
type HCloudPlacementGroupStatus struct {
	// PlacementGroupID is the Hetzner Cloud internal placement group ID.
	// +optional
	PlacementGroupID int64 `json:"placementGroupID,omitempty"`

	// Type is the placement group type observed in Hetzner.
	// +optional
	Type string `json:"type,omitempty"`

	// Conditions represent the latest available observations of the placement group.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcpg
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudPlacementGroup is the Schema for the hcloudplacementgroups API.
type HCloudPlacementGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudPlacementGroupSpec   `json:"spec,omitempty"`
	Status HCloudPlacementGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudPlacementGroupList contains a list of HCloudPlacementGroup.
type HCloudPlacementGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudPlacementGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudPlacementGroup{}, &HCloudPlacementGroupList{})
}
