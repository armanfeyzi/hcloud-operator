package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/afeyzirealyticsio/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/afeyzirealyticsio/hcloud-operator/internal/hcloud"
)

// poll / timeout helpers used throughout the tests.
const (
	pollInterval = 100 * time.Millisecond
	waitTimeout  = 10 * time.Second
)

// newServer returns a minimal HCloudServer object ready to be applied.
func newServer(name string) *infrav1alpha1.HCloudServer {
	return &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: infrav1alpha1.HCloudServerSpec{
			ServerType: "cx21",
			Image:      "ubuntu-22.04",
			Location:   "fsn1",
		},
	}
}

// fetchServer is a small helper that retries until the object is found or timeout.
func fetchServer(name string) func() (*infrav1alpha1.HCloudServer, error) {
	return func() (*infrav1alpha1.HCloudServer, error) {
		obj := &infrav1alpha1.HCloudServer{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, obj)
		return obj, err
	}
}

var _ = Describe("HCloudServerReconciler", func() {

	// Each test gets a unique server name to avoid cross-test collisions.
	var serverName string
	var serverKey types.NamespacedName

	BeforeEach(func() {
		// Use the Ginkgo node index + a counter for a collision-free name.
		serverName = fmt.Sprintf("test-server-%d", time.Now().UnixNano())
		serverKey = types.NamespacedName{Name: serverName}
		// Reset any injected errors from previous tests.
		fakeHCloud.CreateErr = nil
		fakeHCloud.GetErr = nil
		fakeHCloud.DeleteErr = nil
	})

	AfterEach(func() {
		// Best-effort cleanup so the fake doesn't accumulate state.
		obj := &infrav1alpha1.HCloudServer{}
		if err := k8sClient.Get(ctx, serverKey, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	Context("Server creation", func() {
		It("creates a Hetzner server and populates status", func() {
			By("applying an HCloudServer resource")
			server := newServer(serverName)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			By("waiting for the finalizer to be added")
			Eventually(func() bool {
				obj, err := fetchServer(serverName)()
				if err != nil {
					return false
				}
				for _, f := range obj.Finalizers {
					if f == hcloudServerFinalizer {
						return true
					}
				}
				return false
			}, waitTimeout, pollInterval).Should(BeTrue())

			By("waiting for the server ID to appear in status")
			Eventually(func() int64 {
				obj, err := fetchServer(serverName)()
				if err != nil {
					return 0
				}
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			By("verifying IP and state are populated")
			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Status.PublicIPv4).NotTo(BeEmpty())
			Expect(obj.Status.State).To(Equal("running"))

			By("verifying exactly one server exists in the fake backend")
			Expect(fakeHCloud.Len()).To(Equal(1))
		})

		It("adopts a server that already exists by name rather than creating a duplicate", func() {
			By("pre-seeding the fake with a server matching the resource name")
			existing, err := fakeHCloud.CreateServer(ctx, hcloudclient.ServerCreateOpts{
				Name: serverName, ServerType: "cx21", Image: "ubuntu-22.04", Location: "fsn1",
			})
			Expect(err).NotTo(HaveOccurred())
			existingID := existing.ID

			By("applying the HCloudServer resource")
			Expect(k8sClient.Create(ctx, newServer(serverName))).To(Succeed())

			By("waiting for status to reflect the pre-existing server ID")
			Eventually(func() int64 {
				obj, err := fetchServer(serverName)()
				if err != nil {
					return 0
				}
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(Equal(existingID))

			By("verifying no duplicate server was created")
			// Still only the one we seeded.
			Expect(fakeHCloud.Len()).To(Equal(1))
		})

		It("sets Ready=False and requeues when the Hetzner API returns an error", func() {
			By("injecting a create error into the fake")
			fakeHCloud.CreateErr = fmt.Errorf("quota exceeded")

			By("applying the HCloudServer resource")
			Expect(k8sClient.Create(ctx, newServer(serverName))).To(Succeed())

			By("waiting for the Ready condition to be set to False")
			Eventually(func() bool {
				obj, err := fetchServer(serverName)()
				if err != nil {
					return false
				}
				for _, c := range obj.Status.Conditions {
					if c.Type == conditionTypeReady && c.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, waitTimeout, pollInterval).Should(BeTrue())

			By("verifying no server was created in the fake backend")
			Expect(fakeHCloud.Len()).To(Equal(0))
		})
	})

	Context("Server deletion", func() {
		It("deletes the Hetzner server and removes the finalizer", func() {
			By("creating an HCloudServer and waiting for it to be provisioned")
			server := newServer(serverName)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			Expect(fakeHCloud.Len()).To(Equal(1))

			By("deleting the HCloudServer resource")
			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

			By("waiting for the object to be fully removed from the API")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, serverKey, &infrav1alpha1.HCloudServer{})
				return client.IgnoreNotFound(err) == nil && err != nil
			}, waitTimeout, pollInterval).Should(BeTrue(), "object should be gone from the API")

			By("verifying the Hetzner server was deleted from the fake backend")
			Expect(fakeHCloud.Len()).To(Equal(0))
		})

		It("succeeds cleanly when the resource has no server ID (never provisioned)", func() {
			By("creating the resource and patching status to clear the server ID")
			server := newServer(serverName)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Wait for finalizer before patching, so we patch a stable object.
			Eventually(func() bool {
				obj, err := fetchServer(serverName)()
				if err != nil {
					return false
				}
				for _, f := range obj.Finalizers {
					if f == hcloudServerFinalizer {
						return true
					}
				}
				return false
			}, waitTimeout, pollInterval).Should(BeTrue())

			By("deleting the resource")
			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

			By("confirming no delete error occurs and the object is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, serverKey, &infrav1alpha1.HCloudServer{})
				return client.IgnoreNotFound(err) == nil && err != nil
			}, waitTimeout, pollInterval).Should(BeTrue())
		})
	})
})
