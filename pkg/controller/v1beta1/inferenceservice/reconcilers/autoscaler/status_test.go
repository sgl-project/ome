package autoscaler

import (
	"context"
	"testing"
	"time"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/onsi/gomega"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

// newStatusTestScheme builds the minimal scheme the status writer needs:
// autoscalingv2 (for HPA), kedav1 (for ScaledObject), v1beta1 (defensive,
// in case future iterations of the writer need to Get an InferenceReplica).
// Reused across every table case so a missing scheme manifest surfaces
// once at suite-setup time rather than burying as a runtime panic per
// case.
func newStatusTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	g := gomega.NewWithT(t)
	g.Expect(autoscalingv2.AddToScheme(s)).To(gomega.Succeed())
	g.Expect(kedav1.AddToScheme(s)).To(gomega.Succeed())
	g.Expect(v1beta1.AddToScheme(s)).To(gomega.Succeed())
	return s
}

// TestWriteAutoscalerStatus_ManagedByMatrix pins the three-way mapping
// Class → ManagedBy:
//
//   - nil resolved block               → ManagedBy=none
//   - Class=hpa | keda                 → ManagedBy=ome
//   - Class=external                   → ManagedBy=external
//   - Class=none                       → ManagedBy=none
//   - unknown Class (defensive guard)  → ManagedBy=none
//
// Empty client (no live HPA / SO) — every case lands on zero counters
// and nil Conditions; the mapping isolation is the assertion target.
func TestWriteAutoscalerStatus_ManagedByMatrix(t *testing.T) {
	cases := []struct {
		name      string
		resolved  *v1beta1.ComponentAutoscaler
		wantClass v1beta1.AutoscalerClass
		wantMB    v1beta1.AutoscalerManagedBy
	}{
		{
			name:      "nil resolved block → none",
			resolved:  nil,
			wantClass: v1beta1.AutoscalerNone,
			wantMB:    v1beta1.AutoscalerManagedByNone,
		},
		{
			name:      "class=hpa → ome",
			resolved:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA},
			wantClass: v1beta1.AutoscalerHPA,
			wantMB:    v1beta1.AutoscalerManagedByOME,
		},
		{
			name:      "class=keda → ome",
			resolved:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA},
			wantClass: v1beta1.AutoscalerKEDA,
			wantMB:    v1beta1.AutoscalerManagedByOME,
		},
		{
			name:      "class=external → external",
			resolved:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerExternal},
			wantClass: v1beta1.AutoscalerExternal,
			wantMB:    v1beta1.AutoscalerManagedByExternal,
		},
		{
			name:      "class=none → none",
			resolved:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerNone},
			wantClass: v1beta1.AutoscalerNone,
			wantMB:    v1beta1.AutoscalerManagedByNone,
		},
		{
			name:      "unrecognized class → none (defensive)",
			resolved:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerClass("mystery")},
			wantClass: v1beta1.AutoscalerClass("mystery"),
			wantMB:    v1beta1.AutoscalerManagedByNone,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).Build()

			ref := v1beta1.ScaleTargetRef{
				APIVersion: "ome.io/v1beta1",
				Kind:       "InferenceReplica",
				Name:       "isvc-engine",
			}
			status, stRef, err := WriteAutoscalerStatus(
				context.Background(), cli, "ns1", "isvc-engine",
				tc.resolved, SpecSourceISVC, ref,
				nil,
			)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(status).NotTo(gomega.BeNil())
			g.Expect(status.Class).To(gomega.Equal(tc.wantClass))
			g.Expect(status.ManagedBy).To(gomega.Equal(tc.wantMB))
			g.Expect(status.SpecSource).To(gomega.Equal(string(SpecSourceISVC)),
				"SpecSource must echo the input verbatim")

			// No live HPA / SO planted → counters stay zero, conditions nil,
			// LastScaleTime nil. Mirrors the "dispatch not yet wired" state.
			g.Expect(status.CurrentReplicas).To(gomega.BeZero())
			g.Expect(status.DesiredReplicas).To(gomega.BeZero())
			g.Expect(status.LastScaleTime).To(gomega.BeNil())
			g.Expect(status.Conditions).To(gomega.BeNil())

			// ScaleTargetRef passes through verbatim — operator visibility
			// MUST survive every Class branch (including none / external).
			g.Expect(stRef).NotTo(gomega.BeNil())
			g.Expect(*stRef).To(gomega.Equal(ref))
		})
	}
}

// TestWriteAutoscalerStatus_SpecSourcePropagation pins that the SpecSource
// input is echoed verbatim onto status.specSource for all three layers
// (isvc / runtime / default) — operators rely on this to debug which layer
// of the inheritance chain produced the live class.
func TestWriteAutoscalerStatus_SpecSourcePropagation(t *testing.T) {
	sources := []SpecSource{SpecSourceISVC, SpecSourceRuntime, SpecSourceDefault}
	for _, src := range sources {
		src := src
		t.Run(string(src), func(t *testing.T) {
			g := gomega.NewWithT(t)
			cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).Build()
			status, _, err := WriteAutoscalerStatus(
				context.Background(), cli, "ns1", "name1",
				&v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA},
				src, v1beta1.ScaleTargetRef{},
				nil,
			)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(status.SpecSource).To(gomega.Equal(string(src)))
		})
	}
}

// TestWriteAutoscalerStatus_HPAMirror pins the live-HPA mirror path:
// CurrentReplicas / DesiredReplicas / LastScaleTime / Conditions are all
// stamped verbatim from the HPA the writer Gets from the live client.
// Without this guard a refactor of the HPA mirror could silently drop
// fields and the operator would see "scaler exists, but I have no idea
// what it's doing".
func TestWriteAutoscalerStatus_HPAMirror(t *testing.T) {
	g := gomega.NewWithT(t)

	scaleTime := metav1.NewTime(time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC))
	transitionTime := metav1.NewTime(time.Date(2026, 5, 29, 11, 59, 30, 0, time.UTC))
	livehpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "isvc-engine"},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 4,
			DesiredReplicas: 7,
			LastScaleTime:   &scaleTime,
			Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:               autoscalingv2.ScalingActive,
					Status:             corev1.ConditionTrue,
					Reason:             "ValidMetricFound",
					Message:            "the HPA was able to successfully calculate a replica count",
					LastTransitionTime: transitionTime,
				},
				{
					Type:               autoscalingv2.AbleToScale,
					Status:             corev1.ConditionTrue,
					Reason:             "ReadyForNewScale",
					Message:            "recommended size matches current size",
					LastTransitionTime: transitionTime,
				},
				{
					Type:               autoscalingv2.ScalingLimited,
					Status:             corev1.ConditionFalse,
					Reason:             "DesiredWithinRange",
					Message:            "the desired count is within the acceptable range",
					LastTransitionTime: transitionTime,
				},
			},
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(newStatusTestScheme(t)).
		WithObjects(livehpa).
		Build()

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA}
	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceISVC, v1beta1.ScaleTargetRef{
			APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "isvc-engine",
		},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.ManagedBy).To(gomega.Equal(v1beta1.AutoscalerManagedByOME))
	g.Expect(status.CurrentReplicas).To(gomega.Equal(int32(4)))
	g.Expect(status.DesiredReplicas).To(gomega.Equal(int32(7)))
	g.Expect(status.LastScaleTime).NotTo(gomega.BeNil())
	g.Expect(status.LastScaleTime.Time.Equal(scaleTime.Time)).To(gomega.BeTrue(),
		"LastScaleTime must mirror the HPA's value (got %v, want %v)",
		status.LastScaleTime.Time, scaleTime.Time)

	g.Expect(status.Conditions).To(gomega.HaveLen(3))
	g.Expect(status.Conditions[0].Type).To(gomega.Equal("ScalingActive"))
	g.Expect(status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(status.Conditions[0].Reason).To(gomega.Equal("ValidMetricFound"))
	g.Expect(status.Conditions[0].LastTransitionTime.Time.Equal(transitionTime.Time)).To(gomega.BeTrue(),
		"LastTransitionTime must propagate verbatim from the HPA's condition (got %v, want %v)",
		status.Conditions[0].LastTransitionTime.Time, transitionTime.Time)
	g.Expect(status.Conditions[2].Type).To(gomega.Equal("ScalingLimited"))
	g.Expect(status.Conditions[2].Status).To(gomega.Equal(metav1.ConditionFalse))
}

// TestWriteAutoscalerStatus_HPAMirror_NotFound pins the graceful-degradation
// requirement: when the live HPA hasn't been created yet (dispatch not
// wired, or the HPA was just submitted and the cache hasn't caught up),
// the writer returns the static fields populated and counters at zero.
// NO error, no crash, no stale state.
func TestWriteAutoscalerStatus_HPAMirror_NotFound(t *testing.T) {
	g := gomega.NewWithT(t)
	cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).Build()

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA}
	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceISVC, v1beta1.ScaleTargetRef{},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred(),
		"NotFound on HPA Get must degrade gracefully — no error")
	g.Expect(status.ManagedBy).To(gomega.Equal(v1beta1.AutoscalerManagedByOME))
	g.Expect(status.CurrentReplicas).To(gomega.BeZero())
	g.Expect(status.DesiredReplicas).To(gomega.BeZero())
	g.Expect(status.LastScaleTime).To(gomega.BeNil())
	g.Expect(status.Conditions).To(gomega.BeNil())
}

// TestWriteAutoscalerStatus_ScaledObjectMirror pins the KEDA mirror path:
// Conditions + LastScaleTime come from the ScaledObject; CurrentReplicas
// + DesiredReplicas come from the derived HPA the SO writes (looked up via
// SO.Status.HpaName when present, falling back to the SO name otherwise).
//
// The SO name itself is the utils.GetScaledObjectName(name) of the IR — we
// plant both objects under that name so the writer's two-step lookup
// resolves cleanly.
func TestWriteAutoscalerStatus_ScaledObjectMirror(t *testing.T) {
	g := gomega.NewWithT(t)

	soName := utils.GetScaledObjectName("isvc-engine")
	derivedHpaName := "keda-hpa-" + soName

	lastActive := metav1.NewTime(time.Date(2026, 5, 29, 9, 30, 0, 0, time.UTC))
	so := &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: soName},
		Status: kedav1.ScaledObjectStatus{
			HpaName:        derivedHpaName,
			LastActiveTime: &lastActive,
			Conditions: kedav1.Conditions{
				{
					Type:    kedav1.ConditionReady,
					Status:  metav1.ConditionTrue,
					Reason:  kedav1.ScaledObjectConditionReadySuccessReason,
					Message: kedav1.ScaledObjectConditionReadySuccessMessage,
				},
				{
					Type:    kedav1.ConditionActive,
					Status:  metav1.ConditionFalse,
					Reason:  "ScalerDeactivated",
					Message: "Scaler deactivated",
				},
			},
		},
	}
	derivedHpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: derivedHpaName},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 2,
			DesiredReplicas: 5,
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(newStatusTestScheme(t)).
		WithObjects(so, derivedHpa).
		Build()

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA}
	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceRuntime, v1beta1.ScaleTargetRef{
			APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "isvc-engine",
		},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.ManagedBy).To(gomega.Equal(v1beta1.AutoscalerManagedByOME))
	g.Expect(status.SpecSource).To(gomega.Equal(string(SpecSourceRuntime)))

	g.Expect(status.LastScaleTime).NotTo(gomega.BeNil())
	g.Expect(status.LastScaleTime.Time.Equal(lastActive.Time)).To(gomega.BeTrue(),
		"LastScaleTime must mirror the SO's LastActiveTime (got %v, want %v)",
		status.LastScaleTime.Time, lastActive.Time)
	g.Expect(status.CurrentReplicas).To(gomega.Equal(int32(2)))
	g.Expect(status.DesiredReplicas).To(gomega.Equal(int32(5)))

	g.Expect(status.Conditions).To(gomega.HaveLen(2))
	g.Expect(status.Conditions[0].Type).To(gomega.Equal("Ready"))
	g.Expect(status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(status.Conditions[1].Type).To(gomega.Equal("Active"))
	g.Expect(status.Conditions[1].Status).To(gomega.Equal(metav1.ConditionFalse))
}

// TestWriteAutoscalerStatus_ScaledObjectMirror_NoDerivedHPA pins the
// fallback where SO.Status.HpaName is empty (older KEDA versions stamp it
// lazily) AND no HPA exists under the SO's own name — counters land at
// zero, no error. The Conditions + LastScaleTime still mirror because
// those live on the SO directly.
func TestWriteAutoscalerStatus_ScaledObjectMirror_NoDerivedHPA(t *testing.T) {
	g := gomega.NewWithT(t)

	soName := utils.GetScaledObjectName("isvc-engine")
	lastActive := metav1.NewTime(time.Date(2026, 5, 29, 9, 30, 0, 0, time.UTC))
	so := &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: soName},
		Status: kedav1.ScaledObjectStatus{
			LastActiveTime: &lastActive,
			Conditions: kedav1.Conditions{
				{Type: kedav1.ConditionReady, Status: metav1.ConditionTrue},
			},
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(newStatusTestScheme(t)).
		WithObjects(so).
		Build()

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA}
	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceDefault, v1beta1.ScaleTargetRef{},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.CurrentReplicas).To(gomega.BeZero(),
		"no derived HPA found → counters stay zero")
	g.Expect(status.DesiredReplicas).To(gomega.BeZero())
	g.Expect(status.LastScaleTime).NotTo(gomega.BeNil(),
		"LastActiveTime on the SO still mirrors despite missing HPA")
	g.Expect(status.Conditions).To(gomega.HaveLen(1))
}

// TestWriteAutoscalerStatus_ScaleTargetRef_EmptyOmitsField pins that an
// empty ScaleTargetRef returns a nil pointer (so the caller stamps "field
// absent" onto status), not a zero-valued struct. Without this guard the
// rendered status would contain `{"apiVersion":"","kind":"","name":""}`
// which (a) violates the CRD's "always populated when active" comment and
// (b) makes external scalers think there's a target named "" they should
// scale.
func TestWriteAutoscalerStatus_ScaleTargetRef_EmptyOmitsField(t *testing.T) {
	g := gomega.NewWithT(t)
	cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).Build()

	_, stRef, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "name1",
		nil, SpecSourceDefault, v1beta1.ScaleTargetRef{},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(stRef).To(gomega.BeNil(),
		"empty input ref must return nil so the caller omits scaleTargetRef from status")
}

// TestWriteAutoscalerStatus_NilClient pins the "writer must not crash
// when called without a live client" guard — useful for unit tests that
// only want to exercise the static field mapping.
func TestWriteAutoscalerStatus_NilClient(t *testing.T) {
	g := gomega.NewWithT(t)

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA}
	status, stRef, err := WriteAutoscalerStatus(
		context.Background(),
		client.Client(nil),
		"ns1", "name1",
		resolved, SpecSourceISVC, v1beta1.ScaleTargetRef{
			APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "name1",
		},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.ManagedBy).To(gomega.Equal(v1beta1.AutoscalerManagedByOME))
	g.Expect(status.CurrentReplicas).To(gomega.BeZero(),
		"nil client → skip mirror, counters stay zero")
	g.Expect(stRef).NotTo(gomega.BeNil())
}

// TestWriteAutoscalerStatus_ExternalManagedSkipsMirror pins that
// ManagedBy=external does NOT issue a live HPA / SO lookup — even if the
// caller plants one in the fake client, status conditions / counters stay
// empty. ManagedBy=external means OME is hands-off; mirroring would
// misrepresent ownership.
func TestWriteAutoscalerStatus_ExternalManagedSkipsMirror(t *testing.T) {
	g := gomega.NewWithT(t)

	// Plant an HPA that, if naively read, would surface stale counters.
	livehpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "isvc-engine"},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 99,
			DesiredReplicas: 99,
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(newStatusTestScheme(t)).
		WithObjects(livehpa).
		Build()

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerExternal}
	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceISVC, v1beta1.ScaleTargetRef{},
		nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.ManagedBy).To(gomega.Equal(v1beta1.AutoscalerManagedByExternal))
	g.Expect(status.CurrentReplicas).To(gomega.BeZero(),
		"external-managed → MUST NOT mirror; counters stay zero")
	g.Expect(status.Conditions).To(gomega.BeNil())
}

// hpaWithMetricFailure builds an HPA whose ScalingActive=False condition
// carries a FailedGetResourceMetric message ending in "of Pod <podName>" —
// the exact shape the metrics controller emits when a pod lacks a CPU
// request. The pod name is the only thing that rotates between reconciles.
func hpaWithMetricFailure(podName string) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "isvc-decoder"},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 2,
			DesiredReplicas: 2,
			Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:   autoscalingv2.ScalingActive,
					Status: corev1.ConditionFalse,
					Reason: "FailedGetResourceMetric",
					Message: "the HPA was unable to compute the replica count: " +
						"failed to get cpu utilization: missing request for cpu in container " +
						"ome-container of Pod " + podName,
				},
			},
		},
	}
}

// TestWriteAutoscalerStatus_HPAMirror_StableAcrossPodNameChurn is the
// regression guard for the self-reconcile loop. A CPU-utilization HPA on
// pods with no CPU request perpetually emits FailedGetResourceMetric with a
// message whose only volatile component is the sampled pod name. The mirror
// MUST normalize that away so the ISVC status is byte-identical across two
// reconciles that differ ONLY in the rotating pod name — otherwise the
// status write churns and the For() watch re-triggers reconcile forever.
func TestWriteAutoscalerStatus_HPAMirror_StableAcrossPodNameChurn(t *testing.T) {
	g := gomega.NewWithT(t)

	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA}
	ref := v1beta1.ScaleTargetRef{APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "isvc-decoder"}

	mirror := func(podName string) *v1beta1.ComponentAutoscalerStatus {
		cli := fake.NewClientBuilder().
			WithScheme(newStatusTestScheme(t)).
			WithObjects(hpaWithMetricFailure(podName)).
			Build()
		status, _, err := WriteAutoscalerStatus(
			context.Background(), cli, "ns1", "isvc-decoder",
			resolved, SpecSourceISVC, ref,
			nil,
		)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return status
	}

	first := mirror("isvc-decoder-7d9f8-abcde")
	second := mirror("isvc-decoder-7d9f8-zzzzz")

	// The two HPA inputs differ ONLY in the pod name. The mirrored status —
	// conditions included — must be byte-identical, so updateStatus sees no
	// diff and does not write (and thus does not re-trigger reconcile).
	g.Expect(second).To(gomega.Equal(first),
		"mirror must be stable across pod-name churn (got %+v, want %+v)", second, first)

	// The stable signal is preserved: type/status/reason intact, message
	// normalized to a churn-free placeholder.
	g.Expect(first.Conditions).To(gomega.HaveLen(1))
	c := first.Conditions[0]
	g.Expect(c.Type).To(gomega.Equal("ScalingActive"))
	g.Expect(c.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(c.Reason).To(gomega.Equal("FailedGetResourceMetric"))
	g.Expect(c.Message).To(gomega.ContainSubstring("missing request for cpu in container ome-container"))
	g.Expect(c.Message).To(gomega.HaveSuffix("of Pod <pod>"))
	g.Expect(c.Message).NotTo(gomega.ContainSubstring("abcde"))
	g.Expect(c.Message).NotTo(gomega.ContainSubstring("zzzzz"))
}

// findCond returns a pointer to the condition of the given type, or nil.
func findCond(conds []metav1.Condition, t string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == t {
			return &conds[i]
		}
	}
	return nil
}

// kedaSOWithBareConditions builds a ScaledObject whose conditions mirror
// what live KEDA actually writes: Ready/Active/Fallback carry a reason but
// NO lastTransitionTime, and Paused carries NEITHER a reason nor a time.
// Copying these verbatim onto the ISVC status fails the CRD's condition
// schema (lastTransitionTime required, reason min-1-char), which rejects
// every status update and wedges the ISVC reconcile in a permanent error
// loop — the bug the sanitizer guards against.
func kedaSOWithBareConditions(pausedStatus metav1.ConditionStatus) *kedav1.ScaledObject {
	soName := utils.GetScaledObjectName("isvc-engine")
	return &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: soName},
		Status: kedav1.ScaledObjectStatus{
			Conditions: kedav1.Conditions{
				{Type: kedav1.ConditionReady, Status: metav1.ConditionTrue, Reason: "ScaledObjectReady", Message: "ready"},
				{Type: kedav1.ConditionActive, Status: metav1.ConditionTrue, Reason: "ScalerActive", Message: "active"},
				{Type: kedav1.ConditionFallback, Status: metav1.ConditionTrue, Reason: "FallbackExists", Message: "fallback"},
				// The CRD-invalid one: no reason, no lastTransitionTime.
				{Type: kedav1.ConditionPaused, Status: pausedStatus},
			},
		},
	}
}

// TestWriteAutoscalerStatus_ScaledObjectMirror_SanitizesConditionsForCRD is
// the regression guard for the status-update wedge: KEDA conditions
// mirrored onto the ISVC status must satisfy the ISVC CRD (non-empty reason,
// non-zero lastTransitionTime) even though KEDA supplies neither for some.
func TestWriteAutoscalerStatus_ScaledObjectMirror_SanitizesConditionsForCRD(t *testing.T) {
	g := gomega.NewWithT(t)
	cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).
		WithObjects(kedaSOWithBareConditions(metav1.ConditionFalse)).Build()
	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA}
	ref := v1beta1.ScaleTargetRef{APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "isvc-engine"}

	status, _, err := WriteAutoscalerStatus(
		context.Background(), cli, "ns1", "isvc-engine",
		resolved, SpecSourceISVC, ref, nil,
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(status.Conditions).To(gomega.HaveLen(4))
	for _, c := range status.Conditions {
		g.Expect(c.Reason).NotTo(gomega.BeEmpty(),
			"condition %s must have a non-empty reason (CRD requires >=1 char)", c.Type)
		g.Expect(c.LastTransitionTime.IsZero()).To(gomega.BeFalse(),
			"condition %s must have a lastTransitionTime (CRD requires it)", c.Type)
	}
	// The reason-less Paused condition falls back to its Type as the reason.
	paused := findCond(status.Conditions, "Paused")
	g.Expect(paused).NotTo(gomega.BeNil())
	g.Expect(paused.Reason).To(gomega.Equal("Paused"))
}

// TestWriteAutoscalerStatus_ScaledObjectMirror_TimestampStableAndTransitions
// pins that the synthesized KEDA lastTransitionTime is byte-stable across
// reconciles when nothing changed (preserved from prior status, so the
// status writer's DeepEqual diff doesn't storm updates) but re-stamps on a
// real Status transition.
func TestWriteAutoscalerStatus_ScaledObjectMirror_TimestampStableAndTransitions(t *testing.T) {
	g := gomega.NewWithT(t)
	resolved := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA}
	ref := v1beta1.ScaleTargetRef{APIVersion: "ome.io/v1beta1", Kind: "InferenceReplica", Name: "isvc-engine"}
	mirror := func(so *kedav1.ScaledObject, prev *v1beta1.ComponentAutoscalerStatus) *v1beta1.ComponentAutoscalerStatus {
		cli := fake.NewClientBuilder().WithScheme(newStatusTestScheme(t)).WithObjects(so).Build()
		st, _, err := WriteAutoscalerStatus(
			context.Background(), cli, "ns1", "isvc-engine",
			resolved, SpecSourceISVC, ref, prev,
		)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return st
	}

	// Prior status with an old timestamp on every condition so we can prove
	// preservation (unchanged status) vs re-stamp (status flip).
	old := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	prev := &v1beta1.ComponentAutoscalerStatus{Conditions: []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "ScaledObjectReady", LastTransitionTime: old},
		{Type: "Active", Status: metav1.ConditionTrue, Reason: "ScalerActive", LastTransitionTime: old},
		{Type: "Fallback", Status: metav1.ConditionTrue, Reason: "FallbackExists", LastTransitionTime: old},
		{Type: "Paused", Status: metav1.ConditionFalse, Reason: "Paused", LastTransitionTime: old},
	}}

	// Same observed statuses → timestamps preserved (byte-stable, no churn).
	stable := mirror(kedaSOWithBareConditions(metav1.ConditionFalse), prev)
	for _, c := range stable.Conditions {
		g.Expect(c.LastTransitionTime.Time.Equal(old.Time)).To(gomega.BeTrue(),
			"condition %s unchanged → lastTransitionTime must be preserved", c.Type)
	}

	// Paused flips False→True → only its timestamp re-stamps; others preserved.
	flipped := mirror(kedaSOWithBareConditions(metav1.ConditionTrue), prev)
	paused := findCond(flipped.Conditions, "Paused")
	g.Expect(paused).NotTo(gomega.BeNil())
	g.Expect(paused.LastTransitionTime.Time.Equal(old.Time)).To(gomega.BeFalse(),
		"Paused transitioned → lastTransitionTime must advance")
	ready := findCond(flipped.Conditions, "Ready")
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.LastTransitionTime.Time.Equal(old.Time)).To(gomega.BeTrue(),
		"Ready unchanged → lastTransitionTime preserved even when a sibling transitions")
}
