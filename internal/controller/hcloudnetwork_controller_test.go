package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
)

var _ = Describe("HCloudNetworkReconciler", func() {
	var netName string
	var netKey types.NamespacedName

	BeforeEach(func() {
		netName = fmt.Sprintf("test-network-%d", time.Now().UnixNano())
		netKey = types.NamespacedName{Name: netName}
	})

	AfterEach(func() {
		obj := &infrav1alpha1.HCloudNetwork{}
		if err := k8sClient.Get(ctx, netKey, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	It("creates a private network and optional Cloud subnets", func() {
		net := &infrav1alpha1.HCloudNetwork{
			ObjectMeta: metav1.ObjectMeta{Name: netName},
			Spec: infrav1alpha1.HCloudNetworkSpec{
				IPRange:      "10.50.0.0/16",
				NetworkZones: []string{"eu-central"},
				Labels:       map[string]string{"env": "test"},
			},
		}
		Expect(k8sClient.Create(ctx, net)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudNetwork{}
			_ = k8sClient.Get(ctx, netKey, obj)
			return obj.Status.NetworkID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudNetwork{}
		err := k8sClient.Get(ctx, netKey, obj)
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Status.IPRange).To(Equal("10.50.0.0/16"))
		Expect(obj.Status.SubnetZones).To(Equal([]string{"eu-central"}))
	})
})
