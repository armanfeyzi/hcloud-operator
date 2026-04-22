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

func newVolume(name string, size int, serverRef *string) *infrav1alpha1.HCloudVolume {
	vol := &infrav1alpha1.HCloudVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: infrav1alpha1.HCloudVolumeSpec{
			Size: size,
		},
	}
	if serverRef != nil {
		vol.Spec.ServerRef = &corev1.LocalObjectReference{Name: *serverRef}
	} else {
		vol.Spec.Location = "fsn1"
	}
	return vol
}

func fetchVolume(name string) func() (*infrav1alpha1.HCloudVolume, error) {
	return func() (*infrav1alpha1.HCloudVolume, error) {
		obj := &infrav1alpha1.HCloudVolume{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, obj)
		return obj, err
	}
}

var _ = Describe("HCloudVolumeReconciler", func() {
	var volName string

	BeforeEach(func() {
		volName = fmt.Sprintf("test-volume-%d", time.Now().UnixNano())
		fakeHCloud.CreateErr = nil
		fakeHCloud.GetErr = nil
		fakeHCloud.DeleteErr = nil
	})

	AfterEach(func() {
		obj := &infrav1alpha1.HCloudVolume{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: volName, Namespace: "default"}, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	Context("Volume creation", func() {
		It("creates an unattached volume when no ServerRef is provided", func() {
			vol := newVolume(volName, 20, nil)
			Expect(k8sClient.Create(ctx, vol)).To(Succeed())

			Eventually(func() int64 {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return 0
				}
				return v.Status.VolumeID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			v, _ := fetchVolume(volName)()
			Expect(v.Status.State).To(Equal("available"))
			Expect(v.Status.AttachedServerID).To(BeZero())
		})

		It("creates and attaches a volume when ServerRef points to a ready server", func() {
			serverName := fmt.Sprintf("test-server-for-vol-%d", time.Now().UnixNano())

			// Setup Server
			srv := &infrav1alpha1.HCloudServer{
				ObjectMeta: metav1.ObjectMeta{Name: serverName, Namespace: "default"},
				Spec:       infrav1alpha1.HCloudServerSpec{ServerType: "cx21", Image: "ubuntu-22.04", Location: "fsn1"},
			}
			Expect(k8sClient.Create(ctx, srv)).To(Succeed())

			Eventually(func() int64 {
				s := &infrav1alpha1.HCloudServer{}
				k8sClient.Get(ctx, types.NamespacedName{Name: serverName, Namespace: "default"}, s)
				return s.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			s := &infrav1alpha1.HCloudServer{}
			k8sClient.Get(ctx, types.NamespacedName{Name: serverName, Namespace: "default"}, s)
			serverID := s.Status.ServerID

			// Setup Volume
			vol := newVolume(volName, 20, &serverName)
			Expect(k8sClient.Create(ctx, vol)).To(Succeed())

			Eventually(func() int64 {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return 0
				}
				return v.Status.AttachedServerID
			}, waitTimeout, pollInterval).Should(Equal(serverID))
		})

		It("reattempts attachment promptly when referenced server becomes ready", func() {
			serverName := fmt.Sprintf("test-server-delayed-%d", time.Now().UnixNano())

			vol := newVolume(volName, 20, &serverName)
			Expect(k8sClient.Create(ctx, vol)).To(Succeed())

			Consistently(func() int64 {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return -1
				}
				return v.Status.AttachedServerID
			}, 500*time.Millisecond, pollInterval).Should(BeZero())

			srv := &infrav1alpha1.HCloudServer{
				ObjectMeta: metav1.ObjectMeta{Name: serverName, Namespace: "default"},
				Spec:       infrav1alpha1.HCloudServerSpec{ServerType: "cx21", Image: "ubuntu-22.04", Location: "fsn1"},
			}
			Expect(k8sClient.Create(ctx, srv)).To(Succeed())

			Eventually(func() int64 {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return 0
				}
				return v.Status.AttachedServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
		})
	})

	Context("Volume resize", func() {
		It("increases volume size when spec.size is updated", func() {
			vol := newVolume(volName, 20, nil)
			Expect(k8sClient.Create(ctx, vol)).To(Succeed())

			Eventually(func() int {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return 0
				}
				return v.Status.AppliedSize
			}, waitTimeout, pollInterval).Should(Equal(20))

			obj, err := fetchVolume(volName)()
			Expect(err).NotTo(HaveOccurred())
			obj.Spec.Size = 30
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())

			Eventually(func() int {
				v, _ := fetchVolume(volName)()
				if v == nil {
					return 0
				}
				return v.Status.AppliedSize
			}, waitTimeout, pollInterval).Should(Equal(30))
		})
	})
})
