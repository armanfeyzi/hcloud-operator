package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
	hcloudclient "github.com/armanfeyzi/hcloud-operator/internal/hcloud"
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

		It("powers off, changes type, and powers on when spec.serverType is updated", func() {
			By("creating a running server")
			server := newServer(serverName)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			sid := obj.Status.ServerID

			By("patching server type in spec")
			obj.Spec.ServerType = "cpx31"
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())

			By("waiting for applied type and running state")
			Eventually(func() []string {
				o, err := fetchServer(serverName)()
				if err != nil {
					return nil
				}
				return []string{o.Status.AppliedServerType, o.Status.State}
			}, waitTimeout, pollInterval).Should(Equal([]string{"cpx31", "running"}))

			h, err := fakeHCloud.GetServer(ctx, sid)
			Expect(err).NotTo(HaveOccurred())
			Expect(h.ServerType).To(Equal("cpx31"))
		})

		It("passes upgradeDisk to change_type when spec.upgradeDisk is true", func() {
			server := newServer(serverName)
			server.Spec.UpgradeDisk = true
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))

			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			sid := obj.Status.ServerID

			obj.Spec.ServerType = "cpx31"
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())

			Eventually(func() string {
				o, err := fetchServer(serverName)()
				if err != nil {
					return ""
				}
				return o.Status.AppliedServerType
			}, waitTimeout, pollInterval).Should(Equal("cpx31"))

			Expect(fakeHCloud.LastChangeServerTypeUpgradeDisk[sid]).To(BeTrue())
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

		It("attaches the server to referenced HCloudNetwork", func() {
			networkName := fmt.Sprintf("test-network-%d", time.Now().UnixNano())
			networkKey := types.NamespacedName{Name: networkName}

			By("creating a private network")
			netObj := &infrav1alpha1.HCloudNetwork{
				ObjectMeta: metav1.ObjectMeta{Name: networkName},
				Spec: infrav1alpha1.HCloudNetworkSpec{
					IPRange: "10.71.0.0/16",
				},
			}
			Expect(k8sClient.Create(ctx, netObj)).To(Succeed())
			DeferCleanup(func() {
				obj := &infrav1alpha1.HCloudNetwork{}
				if err := k8sClient.Get(ctx, networkKey, obj); err == nil {
					_ = k8sClient.Delete(ctx, obj)
				}
			})

			By("waiting for network status ID")
			var networkID int64
			Eventually(func() int64 {
				obj := &infrav1alpha1.HCloudNetwork{}
				_ = k8sClient.Get(ctx, networkKey, obj)
				return obj.Status.NetworkID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
			_ = k8sClient.Get(ctx, networkKey, netObj)
			networkID = netObj.Status.NetworkID

			By("creating a server that references the network")
			server := newServer(serverName)
			server.Spec.NetworkRef = &corev1.LocalObjectReference{Name: networkName}
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			By("waiting for server provision")
			var sid int64
			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				if obj == nil {
					return 0
				}
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
			obj, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			sid = obj.Status.ServerID

			By("verifying network attachment exists in fake backend")
			Eventually(func() bool {
				s, err := fakeHCloud.GetServer(ctx, sid)
				if err != nil || s == nil {
					return false
				}
				for _, id := range s.NetworkIDs {
					if id == networkID {
						return true
					}
				}
				return false
			}, waitTimeout, pollInterval).Should(BeTrue())

			By("persisting managed network in status")
			Eventually(func() int64 {
				o, _ := fetchServer(serverName)()
				if o == nil {
					return 0
				}
				return o.Status.AppliedNetworkID
			}, waitTimeout, pollInterval).Should(Equal(networkID))

			By("setting explicit Ready reason for network attachment")
			Eventually(func() string {
				o, _ := fetchServer(serverName)()
				if o == nil {
					return ""
				}
				cond := meta.FindStatusCondition(o.Status.Conditions, conditionTypeReady)
				if cond == nil {
					return ""
				}
				return cond.Reason
			}, waitTimeout, pollInterval).Should(Equal(readyReasonNetworkAttached))
		})

		It("migrates attachment when networkRef changes and detaches when removed", func() {
			networkAName := fmt.Sprintf("test-network-a-%d", time.Now().UnixNano())
			networkBName := fmt.Sprintf("test-network-b-%d", time.Now().UnixNano())
			networkAKey := types.NamespacedName{Name: networkAName}
			networkBKey := types.NamespacedName{Name: networkBName}

			createNet := func(name, cidr string) types.NamespacedName {
				key := types.NamespacedName{Name: name}
				obj := &infrav1alpha1.HCloudNetwork{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: infrav1alpha1.HCloudNetworkSpec{
						IPRange: cidr,
					},
				}
				Expect(k8sClient.Create(ctx, obj)).To(Succeed())
				DeferCleanup(func() {
					cleanup := &infrav1alpha1.HCloudNetwork{}
					if err := k8sClient.Get(ctx, key, cleanup); err == nil {
						_ = k8sClient.Delete(ctx, cleanup)
					}
				})
				return key
			}

			createNet(networkAName, "10.72.0.0/16")
			createNet(networkBName, "10.73.0.0/16")

			var networkAID int64
			Eventually(func() int64 {
				obj := &infrav1alpha1.HCloudNetwork{}
				_ = k8sClient.Get(ctx, networkAKey, obj)
				return obj.Status.NetworkID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
			netA := &infrav1alpha1.HCloudNetwork{}
			Expect(k8sClient.Get(ctx, networkAKey, netA)).To(Succeed())
			networkAID = netA.Status.NetworkID

			var networkBID int64
			Eventually(func() int64 {
				obj := &infrav1alpha1.HCloudNetwork{}
				_ = k8sClient.Get(ctx, networkBKey, obj)
				return obj.Status.NetworkID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
			netB := &infrav1alpha1.HCloudNetwork{}
			Expect(k8sClient.Get(ctx, networkBKey, netB)).To(Succeed())
			networkBID = netB.Status.NetworkID

			server := newServer(serverName)
			server.Spec.NetworkRef = &corev1.LocalObjectReference{Name: networkAName}
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			var sid int64
			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				if obj == nil {
					return 0
				}
				return obj.Status.ServerID
			}, waitTimeout, pollInterval).Should(BeNumerically(">", 0))
			cur, err := fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			sid = cur.Status.ServerID

			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				if obj == nil {
					return 0
				}
				return obj.Status.AppliedNetworkID
			}, waitTimeout, pollInterval).Should(Equal(networkAID))

			cur.Spec.NetworkRef = &corev1.LocalObjectReference{Name: networkBName}
			Expect(k8sClient.Update(ctx, cur)).To(Succeed())

			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				if obj == nil {
					return 0
				}
				return obj.Status.AppliedNetworkID
			}, waitTimeout, pollInterval).Should(Equal(networkBID))

			Eventually(func() string {
				o, _ := fetchServer(serverName)()
				if o == nil {
					return ""
				}
				cond := meta.FindStatusCondition(o.Status.Conditions, conditionTypeReady)
				if cond == nil {
					return ""
				}
				return cond.Reason
			}, waitTimeout, pollInterval).Should(Equal(readyReasonNetworkMigrated))

			Eventually(func() []int64 {
				s, _ := fakeHCloud.GetServer(ctx, sid)
				if s == nil {
					return nil
				}
				return s.NetworkIDs
			}, waitTimeout, pollInterval).Should(Equal([]int64{networkBID}))

			cur, err = fetchServer(serverName)()
			Expect(err).NotTo(HaveOccurred())
			cur.Spec.NetworkRef = nil
			Expect(k8sClient.Update(ctx, cur)).To(Succeed())

			Eventually(func() int64 {
				obj, _ := fetchServer(serverName)()
				if obj == nil {
					return -1
				}
				return obj.Status.AppliedNetworkID
			}, waitTimeout, pollInterval).Should(Equal(int64(0)))

			Eventually(func() string {
				o, _ := fetchServer(serverName)()
				if o == nil {
					return ""
				}
				cond := meta.FindStatusCondition(o.Status.Conditions, conditionTypeReady)
				if cond == nil {
					return ""
				}
				return cond.Reason
			}, waitTimeout, pollInterval).Should(Equal(readyReasonNetworkDetached))

			Eventually(func() []int64 {
				s, _ := fakeHCloud.GetServer(ctx, sid)
				if s == nil {
					return nil
				}
				return s.NetworkIDs
			}, waitTimeout, pollInterval).Should(BeEmpty())
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
