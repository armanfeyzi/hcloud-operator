package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HCloudCertificateSpec defines the desired state of an HCloudCertificate.
// +kubebuilder:validation:XValidation:rule="self.type == oldSelf.type",message="type is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.type != 'uploaded' || (has(self.certificate) && has(self.privateKey))",message="certificate and privateKey are required for uploaded certificates"
// +kubebuilder:validation:XValidation:rule="self.type != 'managed' || self.domainNames.size() > 0",message="domainNames are required for managed certificates"
type HCloudCertificateSpec struct {
	// Type is the certificate type: uploaded or managed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=uploaded;managed
	Type string `json:"type"`

	// Certificate is the PEM-encoded certificate chain for uploaded certificates.
	// +optional
	Certificate string `json:"certificate,omitempty"`

	// PrivateKey is the PEM-encoded private key for uploaded certificates.
	// +optional
	PrivateKey string `json:"privateKey,omitempty"`

	// DomainNames lists domains for managed certificates.
	// +optional
	// +listType=set
	DomainNames []string `json:"domainNames,omitempty"`

	// Labels to attach to the Hetzner Cloud certificate resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// HCloudCertificateStatus defines the observed state of an HCloudCertificate.
type HCloudCertificateStatus struct {
	// CertificateID is the Hetzner Cloud internal certificate ID.
	// +optional
	CertificateID int64 `json:"certificateID,omitempty"`

	// DomainNames are the domains observed in Hetzner.
	// +optional
	DomainNames []string `json:"domainNames,omitempty"`

	// Fingerprint is the certificate fingerprint observed in Hetzner.
	// +optional
	Fingerprint string `json:"fingerprint,omitempty"`

	// NotValidBefore is when the certificate becomes valid.
	// +optional
	NotValidBefore *metav1.Time `json:"notValidBefore,omitempty"`

	// NotValidAfter is when the certificate expires.
	// +optional
	NotValidAfter *metav1.Time `json:"notValidAfter,omitempty"`

	// IssuanceStatus reflects managed certificate issuance (e.g. pending, completed, failed).
	// +optional
	IssuanceStatus string `json:"issuanceStatus,omitempty"`

	// Conditions represent the latest available observations of the certificate.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hccert
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Issuance",type=string,JSONPath=`.status.issuanceStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HCloudCertificate is the Schema for the hcloudcertificates API.
type HCloudCertificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HCloudCertificateSpec   `json:"spec,omitempty"`
	Status HCloudCertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HCloudCertificateList contains a list of HCloudCertificate.
type HCloudCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HCloudCertificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HCloudCertificate{}, &HCloudCertificateList{})
}
