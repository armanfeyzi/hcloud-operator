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

func newLoadBalancer(name string, selector map[string]string) *infrav1alpha1.HCloudLoadBalancer {
	lb := &infrav1alpha1.HCloudLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: infrav1alpha1.HCloudLoadBalancerSpec{
			LoadBalancerType: "lb11",
			Location:         "fsn1",
			Algorithm:        "round_robin",
		},
	}
	if selector != nil {
		lb.Spec.ServerSelector = &metav1.LabelSelector{MatchLabels: selector}
	}
	return lb
}

var _ = Describe("HCloudLoadBalancerReconciler", func() {
	var lbName string
	var backendServerName string

	BeforeEach(func() {
		lbName = fmt.Sprintf("test-lb-%d", time.Now().UnixNano())
		backendServerName = fmt.Sprintf("test-lb-server-%d", time.Now().UnixNano())
		fakeHCloud.CreateErr = nil
		fakeHCloud.GetErr = nil
		fakeHCloud.DeleteErr = nil
	})

	AfterEach(func() {
		lb := &infrav1alpha1.HCloudLoadBalancer{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, lb); err == nil {
			_ = k8sClient.Delete(ctx, lb)
		}
		server := &infrav1alpha1.HCloudServer{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: backendServerName}, server); err == nil {
			_ = k8sClient.Delete(ctx, server)
		}
	})

	It("creates a load balancer and syncs target servers from serverSelector", func() {
		server := &infrav1alpha1.HCloudServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:   backendServerName,
				Labels: map[string]string{"app": "web"},
			},
			Spec: infrav1alpha1.HCloudServerSpec{
				ServerType: "cx21",
				Image:      "ubuntu-22.04",
				Location:   "fsn1",
			},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudServer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: backendServerName}, obj)
			return obj.Status.ServerID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		Expect(k8sClient.Create(ctx, newLoadBalancer(lbName, map[string]string{"app": "web"}))).To(Succeed())

		Eventually(func() int {
			obj := &infrav1alpha1.HCloudLoadBalancer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)
			return len(obj.Status.AttachedServerIDs)
		}, waitTimeout, pollInterval).Should(Equal(1))

		obj := &infrav1alpha1.HCloudLoadBalancer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)).To(Succeed())
		Expect(obj.Status.LoadBalancerID).To(BeNumerically(">", 0))
		Expect(obj.Status.PublicIPv4).NotTo(BeEmpty())
		Expect(fakeHCloud.LenLoadBalancers()).To(Equal(1))
	})

	It("syncs load balancer services and health checks", func() {
		lb := newLoadBalancer(lbName, nil)
		interval := int32(15)
		timeout := int32(10)
		retries := int32(3)
		path := "/healthz"
		lb.Spec.Services = []infrav1alpha1.HCloudLoadBalancerServiceSpec{
			{
				ListenPort:      80,
				DestinationPort: 8080,
				Protocol:        "http",
				HealthCheck: &infrav1alpha1.HCloudLoadBalancerHealthCheckSpec{
					Protocol:        "http",
					IntervalSeconds: &interval,
					TimeoutSeconds:  &timeout,
					Retries:         &retries,
					HTTP: &infrav1alpha1.HCloudLoadBalancerHealthCheckHTTPSpec{
						Path: &path,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, lb)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudLoadBalancer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)
			return obj.Status.LoadBalancerID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudLoadBalancer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)).To(Succeed())
		info, err := fakeHCloud.GetLoadBalancer(ctx, obj.Status.LoadBalancerID)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Services).To(HaveLen(1))
		Expect(info.Services[0].ListenPort).To(Equal(80))
		Expect(info.Services[0].DestinationPort).To(Equal(8080))
		Expect(info.Services[0].HealthCheck).NotTo(BeNil())
		Expect(info.Services[0].HealthCheck.HTTP).NotTo(BeNil())
		Expect(*info.Services[0].HealthCheck.HTTP.Path).To(Equal("/healthz"))
	})

	It("syncs HTTPS services with referenced certificates", func() {
		certName := fmt.Sprintf("test-lb-cert-%d", time.Now().UnixNano())
		cert := &infrav1alpha1.HCloudCertificate{
			ObjectMeta: metav1.ObjectMeta{Name: certName},
			Spec: infrav1alpha1.HCloudCertificateSpec{
				Type:        "uploaded",
				Certificate: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
				PrivateKey:  "-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n",
			},
		}
		Expect(k8sClient.Create(ctx, cert)).To(Succeed())
		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudCertificate{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: certName}, obj)
			return obj.Status.CertificateID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		lb := newLoadBalancer(lbName, nil)
		lb.Spec.Services = []infrav1alpha1.HCloudLoadBalancerServiceSpec{
			{
				ListenPort:      443,
				DestinationPort: 8443,
				Protocol:        "https",
				CertificateRefs: []corev1.LocalObjectReference{{Name: certName}},
			},
		}
		Expect(k8sClient.Create(ctx, lb)).To(Succeed())

		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudLoadBalancer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)
			return obj.Status.LoadBalancerID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		obj := &infrav1alpha1.HCloudLoadBalancer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)).To(Succeed())
		certObj := &infrav1alpha1.HCloudCertificate{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: certName}, certObj)).To(Succeed())
		info, err := fakeHCloud.GetLoadBalancer(ctx, obj.Status.LoadBalancerID)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Services).To(HaveLen(1))
		Expect(info.Services[0].Protocol).To(Equal("https"))
		Expect(info.Services[0].CertificateIDs).To(Equal([]int64{certObj.Status.CertificateID}))
	})

	It("detaches server targets when labels no longer match selector", func() {
		server := &infrav1alpha1.HCloudServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:   backendServerName,
				Labels: map[string]string{"app": "api"},
			},
			Spec: infrav1alpha1.HCloudServerSpec{
				ServerType: "cx21",
				Image:      "ubuntu-22.04",
				Location:   "fsn1",
			},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Eventually(func() int64 {
			obj := &infrav1alpha1.HCloudServer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: backendServerName}, obj)
			return obj.Status.ServerID
		}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

		Expect(k8sClient.Create(ctx, newLoadBalancer(lbName, map[string]string{"app": "api"}))).To(Succeed())
		Eventually(func() int {
			obj := &infrav1alpha1.HCloudLoadBalancer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, obj)
			return len(obj.Status.AttachedServerIDs)
		}, waitTimeout, pollInterval).Should(Equal(1))

		obj := &infrav1alpha1.HCloudServer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: backendServerName}, obj)).To(Succeed())
		obj.Labels["app"] = "worker"
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())

		Eventually(func() int {
			lb := &infrav1alpha1.HCloudLoadBalancer{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: lbName}, lb)
			return len(lb.Status.AttachedServerIDs)
		}, waitTimeout, pollInterval).Should(Equal(0))
	})
})
