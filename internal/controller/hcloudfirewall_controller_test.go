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

var _ = Describe("HCloudFirewallReconciler", func() {
	var fwName string
	var fwKey types.NamespacedName

	BeforeEach(func() {
		fwName = fmt.Sprintf("test-fw-%d", time.Now().UnixNano())
		fwKey = types.NamespacedName{Name: fwName}
	})

	AfterEach(func() {
		obj := &infrav1alpha1.HCloudFirewall{}
		if err := k8sClient.Get(ctx, fwKey, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	It("creates a firewall from rules", func() {
		fw := &infrav1alpha1.HCloudFirewall{
			ObjectMeta: metav1.ObjectMeta{Name: fwName},
			Spec: infrav1alpha1.HCloudFirewallSpec{
				Labels: map[string]string{"env": "test"},
				Rules: []infrav1alpha1.HCloudFirewallRule{
					{
						Direction: "in",
						Protocol:  "tcp",
						SourceIPs: []string{"0.0.0.0/0"},
						Port:      strPtr("22"),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, fw)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudFirewall{}
			_ = k8sClient.Get(ctx, fwKey, obj)
			return obj.Status.FirewallID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
	})
})

func strPtr(s string) *string {
	return &s
}
