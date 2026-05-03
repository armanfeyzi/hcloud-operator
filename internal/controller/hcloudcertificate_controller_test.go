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

var _ = Describe("HCloudCertificateReconciler", func() {
	var certName string
	var certKey types.NamespacedName

	BeforeEach(func() {
		certName = fmt.Sprintf("test-cert-%d", time.Now().UnixNano())
		certKey = types.NamespacedName{Name: certName}
	})

	AfterEach(func() {
		cert := &infrav1alpha1.HCloudCertificate{}
		if err := k8sClient.Get(ctx, certKey, cert); err == nil {
			_ = k8sClient.Delete(ctx, cert)
		}
	})

	It("creates an uploaded certificate", func() {
		cert := &infrav1alpha1.HCloudCertificate{
			ObjectMeta: metav1.ObjectMeta{Name: certName},
			Spec: infrav1alpha1.HCloudCertificateSpec{
				Type:        "uploaded",
				Certificate: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
				PrivateKey:  "-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n",
				Labels:      map[string]string{"app": "web"},
			},
		}
		Expect(k8sClient.Create(ctx, cert)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudCertificate{}
			_ = k8sClient.Get(ctx, certKey, obj)
			return obj.Status.CertificateID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudCertificate{}
		Expect(k8sClient.Get(ctx, certKey, obj)).To(Succeed())
		Expect(obj.Status.IssuanceStatus).To(Equal("completed"))
		Expect(obj.Status.Fingerprint).NotTo(BeEmpty())

		info, err := fakeHCloud.GetCertificate(ctx, obj.Status.CertificateID)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Labels["app"]).To(Equal("web"))
	})
})
