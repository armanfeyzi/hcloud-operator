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

var _ = Describe("HCloudPlacementGroupReconciler", func() {
	var pgName string
	var pgKey types.NamespacedName

	BeforeEach(func() {
		pgName = fmt.Sprintf("test-pg-%d", time.Now().UnixNano())
		pgKey = types.NamespacedName{Name: pgName}
	})

	AfterEach(func() {
		obj := &infrav1alpha1.HCloudPlacementGroup{}
		if err := k8sClient.Get(ctx, pgKey, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	It("creates a spread placement group", func() {
		pg := &infrav1alpha1.HCloudPlacementGroup{
			ObjectMeta: metav1.ObjectMeta{Name: pgName},
			Spec: infrav1alpha1.HCloudPlacementGroupSpec{
				Type:   "spread",
				Labels: map[string]string{"env": "test"},
			},
		}
		Expect(k8sClient.Create(ctx, pg)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudPlacementGroup{}
			_ = k8sClient.Get(ctx, pgKey, obj)
			return obj.Status.PlacementGroupID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudPlacementGroup{}
		Expect(k8sClient.Get(ctx, pgKey, obj)).To(Succeed())
		Expect(obj.Status.Type).To(Equal("spread"))
	})
})
