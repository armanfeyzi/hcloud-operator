package reconcile

import (
	"context"
	"errors"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha1 "github.com/armanfeyzi/hcloud-operator/api/v1alpha1"
)

const (
	testFinalizer = "infra.hkc.io/test-finalizer"
	testKind      = "HCloudServer"
)

// stubResource is a configurable Resource[*HCloudServer] for exercising the base.
type stubResource struct {
	reconcileFn    func(ctx context.Context, obj *infrav1alpha1.HCloudServer) (ctrl.Result, error)
	deleteFn       func(ctx context.Context, obj *infrav1alpha1.HCloudServer) error
	reconcileCalls int
	deleteCalls    int
}

func (s *stubResource) NewObject() *infrav1alpha1.HCloudServer { return &infrav1alpha1.HCloudServer{} }
func (s *stubResource) FinalizerName() string                  { return testFinalizer }
func (s *stubResource) Kind() string                           { return testKind }

func (s *stubResource) Reconcile(ctx context.Context, obj *infrav1alpha1.HCloudServer) (ctrl.Result, error) {
	s.reconcileCalls++
	if s.reconcileFn != nil {
		return s.reconcileFn(ctx, obj)
	}
	return ctrl.Result{}, nil
}

func (s *stubResource) Delete(ctx context.Context, obj *infrav1alpha1.HCloudServer) error {
	s.deleteCalls++
	if s.deleteFn != nil {
		return s.deleteFn(ctx, obj)
	}
	return nil
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := infrav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return scheme
}

func newBase(t *testing.T, res Resource[*infrav1alpha1.HCloudServer], recorder record.EventRecorder, objs ...client.Object) (*BaseReconciler[*infrav1alpha1.HCloudServer], client.Client) {
	t.Helper()
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithStatusSubresource(&infrav1alpha1.HCloudServer{}).
		WithObjects(objs...).
		Build()
	return &BaseReconciler[*infrav1alpha1.HCloudServer]{Client: c, Recorder: recorder, Resource: res}, c
}

func reqFor(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func drainEvents(rec *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

func containsEvent(events []string, substr string) bool {
	for _, e := range events {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

func TestBaseReconcile_NotFound(t *testing.T) {
	res := &stubResource{}
	base, _ := newBase(t, res, record.NewFakeRecorder(10))

	result, err := base.Reconcile(context.Background(), reqFor("missing"))
	if err != nil {
		t.Fatalf("expected nil error for not-found, got %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got %+v", result)
	}
	if res.reconcileCalls != 0 || res.deleteCalls != 0 {
		t.Fatalf("domain methods should not run for not-found: reconcile=%d delete=%d", res.reconcileCalls, res.deleteCalls)
	}
}

func TestBaseReconcile_AddsFinalizerAndRequeues(t *testing.T) {
	obj := &infrav1alpha1.HCloudServer{ObjectMeta: metav1.ObjectMeta{Name: "s1"}}
	res := &stubResource{}
	base, c := newBase(t, res, record.NewFakeRecorder(10), obj)

	result, err := base.Reconcile(context.Background(), reqFor("s1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected requeue after adding finalizer, got %+v", result)
	}
	if res.reconcileCalls != 0 {
		t.Fatalf("domain Reconcile should not run on the finalizer-add pass")
	}
	got := &infrav1alpha1.HCloudServer{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "s1"}, got); err != nil {
		t.Fatalf("get object: %v", err)
	}
	if !controllerutil.ContainsFinalizer(got, testFinalizer) {
		t.Fatalf("finalizer was not added")
	}
}

func TestBaseReconcile_SuccessSetsSyncedAndPersists(t *testing.T) {
	obj := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{Name: "s2", Finalizers: []string{testFinalizer}},
	}
	res := &stubResource{
		reconcileFn: func(_ context.Context, o *infrav1alpha1.HCloudServer) (ctrl.Result, error) {
			// domain logic sets Ready and a status field
			meta.SetStatusCondition(o.GetConditions(), metav1.Condition{
				Type: ConditionReady, Status: metav1.ConditionTrue, Reason: "ServerRunning", Message: "ok",
			})
			o.Status.ServerID = 42
			return ctrl.Result{}, nil
		},
	}
	rec := record.NewFakeRecorder(10)
	base, c := newBase(t, res, rec, obj)

	if _, err := base.Reconcile(context.Background(), reqFor("s2")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.reconcileCalls != 1 {
		t.Fatalf("expected domain Reconcile to run once, got %d", res.reconcileCalls)
	}

	got := &infrav1alpha1.HCloudServer{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "s2"}, got); err != nil {
		t.Fatalf("get object: %v", err)
	}
	synced := meta.FindStatusCondition(got.Status.Conditions, ConditionSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue {
		t.Fatalf("expected Synced=True, got %+v", synced)
	}
	ready := meta.FindStatusCondition(got.Status.Conditions, ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected domain Ready=True to be preserved, got %+v", ready)
	}
	if got.Status.ServerID != 42 {
		t.Fatalf("expected domain status field persisted, got serverID=%d", got.Status.ServerID)
	}
}

func TestBaseReconcile_ErrorSetsConditionsAndEmitsWarning(t *testing.T) {
	obj := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{Name: "s3", Finalizers: []string{testFinalizer}},
	}
	wantErr := errors.New("quota exceeded")
	res := &stubResource{
		reconcileFn: func(_ context.Context, _ *infrav1alpha1.HCloudServer) (ctrl.Result, error) {
			return ctrl.Result{}, wantErr
		},
	}
	rec := record.NewFakeRecorder(10)
	base, c := newBase(t, res, rec, obj)

	_, err := base.Reconcile(context.Background(), reqFor("s3"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped reconcile error, got %v", err)
	}

	got := &infrav1alpha1.HCloudServer{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "s3"}, got); err != nil {
		t.Fatalf("get object: %v", err)
	}
	synced := meta.FindStatusCondition(got.Status.Conditions, ConditionSynced)
	if synced == nil || synced.Status != metav1.ConditionFalse || synced.Reason != reasonReconcileError {
		t.Fatalf("expected Synced=False/ReconcileError, got %+v", synced)
	}
	ready := meta.FindStatusCondition(got.Status.Conditions, ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != reasonReconcileError {
		t.Fatalf("expected Ready=False/ReconcileError, got %+v", ready)
	}
	events := drainEvents(rec)
	if !containsEvent(events, eventReconcileError) {
		t.Fatalf("expected a ReconcileError warning event, got %v", events)
	}
}

func TestBaseReconcile_DeletionRunsDeleteAndRemovesFinalizer(t *testing.T) {
	now := metav1.Now()
	obj := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "s4",
			Finalizers:        []string{testFinalizer},
			DeletionTimestamp: &now,
		},
	}
	res := &stubResource{}
	rec := record.NewFakeRecorder(10)
	base, c := newBase(t, res, rec, obj)

	if _, err := base.Reconcile(context.Background(), reqFor("s4")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.deleteCalls != 1 {
		t.Fatalf("expected external Delete to run once, got %d", res.deleteCalls)
	}
	// Removing the last finalizer lets the API delete the object entirely.
	got := &infrav1alpha1.HCloudServer{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "s4"}, got)
	if err == nil {
		if controllerutil.ContainsFinalizer(got, testFinalizer) {
			t.Fatalf("finalizer should have been removed")
		}
	} else if !apierrors.IsNotFound(err) {
		t.Fatalf("unexpected get error: %v", err)
	}
	events := drainEvents(rec)
	if !containsEvent(events, eventDeleted) {
		t.Fatalf("expected a Deleted event, got %v", events)
	}
}

func TestBaseReconcile_DeleteFailureEmitsWarningAndKeepsFinalizer(t *testing.T) {
	now := metav1.Now()
	obj := &infrav1alpha1.HCloudServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "s5",
			Finalizers:        []string{testFinalizer},
			DeletionTimestamp: &now,
		},
	}
	res := &stubResource{
		deleteFn: func(_ context.Context, _ *infrav1alpha1.HCloudServer) error {
			return errors.New("hetzner still busy")
		},
	}
	rec := record.NewFakeRecorder(10)
	base, c := newBase(t, res, rec, obj)

	if _, err := base.Reconcile(context.Background(), reqFor("s5")); err == nil {
		t.Fatalf("expected error when external delete fails")
	}
	got := &infrav1alpha1.HCloudServer{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "s5"}, got); err != nil {
		t.Fatalf("object should still exist while finalizer is held: %v", err)
	}
	if !controllerutil.ContainsFinalizer(got, testFinalizer) {
		t.Fatalf("finalizer must be retained when delete fails")
	}
	events := drainEvents(rec)
	if !containsEvent(events, eventDeleteFailed) {
		t.Fatalf("expected a DeleteFailed warning event, got %v", events)
	}
}