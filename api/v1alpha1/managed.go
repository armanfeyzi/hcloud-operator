package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GetConditions returns a pointer to the resource's status conditions slice.
//
// Each HCloud* root type implements this so the shared generic base reconciler
// (internal/reconcile) can own cross-cutting conditions such as "Synced" without
// knowing the concrete type. Domain logic continues to own "Ready".

func (o *HCloudServer) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudVolume) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudLoadBalancer) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudNetwork) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudFirewall) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudPlacementGroup) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudPrimaryIP) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudFloatingIP) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

func (o *HCloudCertificate) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }
