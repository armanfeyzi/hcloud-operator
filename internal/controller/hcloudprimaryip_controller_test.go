package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
)

var _ = Describe("HCloudPrimaryIPReconciler", func() {
	var pipName string
	var serverName string
	var pipKey types.NamespacedName

	BeforeEach(func() {
		pipName = fmt.Sprintf("test-pip-%d", time.Now().UnixNano())
		serverName = fmt.Sprintf("test-pip-server-%d", time.Now().UnixNano())
		pipKey = types.NamespacedName{Name: pipName}
	})

	AfterEach(func() {
		pip := &infrav1alpha1.HCloudPrimaryIP{}
		if err := k8sClient.Get(ctx, pipKey, pip); err == nil {
			_ = k8sClient.Delete(ctx, pip)
		}
		server := &infrav1alpha1.HCloudServer{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, server); err == nil {
			_ = k8sClient.Delete(ctx, server)
		}
	})

	It("creates a primary IP and assigns it to a referenced server", func() {
		server := &infrav1alpha1.HCloudServer{
			ObjectMeta: metav1.ObjectMeta{Name: serverName},
			Spec: infrav1alpha1.HCloudServerSpec{
				ServerType: "cx21",
				Image:      "ubuntu-22.04",
				Location:   "fsn1",
			},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())

		ptr := "app.example.com"
		autoDelete := true
		pip := &infrav1alpha1.HCloudPrimaryIP{
			ObjectMeta: metav1.ObjectMeta{Name: pipName},
			Spec: infrav1alpha1.HCloudPrimaryIPSpec{
				Type:       "ipv4",
				Datacenter: "fsn1-dc14",
				ServerRef:  &corev1.LocalObjectReference{Name: serverName},
				AutoDelete: &autoDelete,
				DNSPtr:     &ptr,
				Labels:     map[string]string{"role": "public"},
			},
		}
		Expect(k8sClient.Create(ctx, pip)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudPrimaryIP{}
			_ = k8sClient.Get(ctx, pipKey, obj)
			return obj.Status.PrimaryIPID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudPrimaryIP{}
			_ = k8sClient.Get(ctx, pipKey, obj)
			return obj.Status.AppliedAssigneeID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudPrimaryIP{}
		Expect(k8sClient.Get(ctx, pipKey, obj)).To(Succeed())
		Expect(obj.Status.IP).NotTo(BeEmpty())
		Expect(obj.Status.Datacenter).To(Equal("fsn1-dc14"))

		info, err := fakeHCloud.GetPrimaryIP(ctx, obj.Status.PrimaryIPID)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.DNSPtr[info.IP]).To(Equal("app.example.com"))
		Expect(info.AssigneeID).To(Equal(obj.Status.AppliedAssigneeID))
	})
})
