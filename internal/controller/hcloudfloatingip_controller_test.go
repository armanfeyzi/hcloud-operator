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

var _ = Describe("HCloudFloatingIPReconciler", func() {
	var fipName string
	var serverName string
	var fipKey types.NamespacedName

	BeforeEach(func() {
		fipName = fmt.Sprintf("test-fip-%d", time.Now().UnixNano())
		serverName = fmt.Sprintf("test-fip-server-%d", time.Now().UnixNano())
		fipKey = types.NamespacedName{Name: fipName}
	})

	AfterEach(func() {
		fip := &infrav1alpha1.HCloudFloatingIP{}
		if err := k8sClient.Get(ctx, fipKey, fip); err == nil {
			_ = k8sClient.Delete(ctx, fip)
		}
		server := &infrav1alpha1.HCloudServer{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, server); err == nil {
			_ = k8sClient.Delete(ctx, server)
		}
	})

	It("creates a floating IP and assigns it to a referenced server", func() {
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
		fip := &infrav1alpha1.HCloudFloatingIP{
			ObjectMeta: metav1.ObjectMeta{Name: fipName},
			Spec: infrav1alpha1.HCloudFloatingIPSpec{
				Type:        "ipv4",
				Location:    "fsn1",
				Description: "public ingress",
				ServerRef:   &corev1.LocalObjectReference{Name: serverName},
				DNSPtr:      &ptr,
				Labels:      map[string]string{"role": "public"},
			},
		}
		Expect(k8sClient.Create(ctx, fip)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudFloatingIP{}
			_ = k8sClient.Get(ctx, fipKey, obj)
			return obj.Status.FloatingIPID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudFloatingIP{}
			_ = k8sClient.Get(ctx, fipKey, obj)
			return obj.Status.AppliedServerID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudFloatingIP{}
		Expect(k8sClient.Get(ctx, fipKey, obj)).To(Succeed())
		Expect(obj.Status.IP).NotTo(BeEmpty())
		Expect(obj.Status.Location).To(Equal("fsn1"))

		info, err := fakeHCloud.GetFloatingIP(ctx, obj.Status.FloatingIPID)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.DNSPtr[info.IP]).To(Equal("app.example.com"))
		Expect(info.ServerID).To(Equal(obj.Status.AppliedServerID))
	})
})
