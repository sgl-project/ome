package runtimeprojection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/effective"
	"sigs.k8s.io/ome/pkg/cli/paging"
	"sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
	"sigs.k8s.io/ome/pkg/constants"
)

var projectionTestClock = v1alpha1.ClockFunc(func() time.Time {
	return time.Date(2026, time.August, 31, 20, 15, 0, 0, time.UTC)
})

func TestProjectEffectiveRejectsNilInputsWithFixedSentinel(t *testing.T) {
	isvc := &v1beta1.InferenceService{}
	state := &effective.RuntimeState{}

	_, err := ProjectEffective(nil, state, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())

	_, err = ProjectEffective(isvc, nil, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
}

func TestProjectEffectiveMapsResolvedLiveConfiguration(t *testing.T) {
	isvc, state := resolveLiveProjectionFixture(t)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	wantRuntime := &v1alpha1.RuntimeObjectReference{
		APIVersion: v1beta1.SchemeGroupVersion.String(),
		Kind:       v1alpha1.RuntimeKindClusterServingRuntime,
		Name:       "gpu-runtime",
		UID:        "runtime-uid",
		Generation: 5,
	}
	wantConfiguration := v1alpha1.RuntimeConfiguration{
		State:  v1alpha1.ConfigurationStateAvailable,
		Origin: v1alpha1.ConfigurationOriginLiveRuntime,
		Source: wantRuntime,
		Hash:   state.LiveShortHash,
		Components: []v1alpha1.RuntimeComponent{{
			Type:                 v1alpha1.RuntimeComponentEngine,
			DeploymentMode:       v1alpha1.DeploymentModeOMENative,
			DeploymentModeSource: v1alpha1.DeploymentModeSourceServiceSpec,
		}},
	}
	assert.Equal(t, v1alpha1.APIVersion, reportValue.APIVersion)
	assert.Equal(t, v1alpha1.RuntimeEffectiveReportKind, reportValue.Kind)
	assert.Equal(t, v1alpha1.Metadata{Namespace: "workloads", Name: "chat"}, reportValue.Metadata)
	assert.Equal(t, projectionTestClock.Now(), reportValue.CollectedAt)
	assert.Equal(t, v1alpha1.RuntimeEffectiveContent{
		Selection: v1alpha1.RuntimeSelection{
			Source:  v1alpha1.RuntimeSelectionSourceExplicit,
			Runtime: wantRuntime,
		},
		Inheritance: v1alpha1.RuntimeInheritance{
			State: v1alpha1.InheritanceStateObserved,
			Sources: []v1alpha1.RuntimeObjectReference{
				{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       v1alpha1.RuntimeKindClusterServingRuntime,
					Name:       "base-runtime",
					UID:        "base-uid",
					Generation: 2,
				},
				*wantRuntime,
			},
		},
		Pin: v1alpha1.RuntimePin{
			Mode:  v1alpha1.RuntimePinModeAutoSync,
			State: v1alpha1.RuntimePinStateNotApplicable,
			Status: v1alpha1.RuntimeStatusObservation{
				Generation:         7,
				ObservedGeneration: 7,
				Freshness:          v1alpha1.StatusFreshnessCurrent,
			},
			ReportedDrift: v1alpha1.RuntimeDriftObservation{State: v1alpha1.DriftConditionStateNotReported},
			SyncState:     v1alpha1.RuntimeSyncStateAbsent,
		},
		Live:         wantConfiguration,
		Active:       wantConfiguration,
		LiveToActive: v1alpha1.RuntimeHashRelationEqual,
		Issues:       []v1alpha1.RuntimeIssue{},
	}, reportValue.Content)
	assert.Equal(t, []v1alpha1.RuntimeSourceReference{
		{
			Kind: "ClusterServingRuntime", Name: "base-runtime", UID: "base-uid", Generation: 2,
			Evidence: v1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
		{
			Kind: "ClusterServingRuntime", Name: "gpu-runtime", UID: "runtime-uid", Generation: 5,
			Evidence: v1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
		{
			Kind: "InferenceService", Namespace: "workloads", Name: "chat", UID: "isvc-uid", Generation: 7,
			Evidence: v1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
	}, reportValue.Sources)
	assert.Empty(t, reportValue.Warnings)
}

func TestProjectEffectiveRejectsMismatchedStateGeneration(t *testing.T) {
	isvc, state := resolveLiveProjectionFixture(t)
	state.Generation++

	_, err := ProjectEffective(isvc, state, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
}

func TestProjectEffectiveRejectsCrossAssociatedOrHostileEvidence(t *testing.T) {
	baseISVC, baseState := resolveLiveProjectionFixture(t)
	tests := []struct {
		name   string
		mutate func(*v1beta1.InferenceService, *effective.RuntimeState)
	}{
		{
			name: "empty service name",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Name = ""
			},
		},
		{
			name: "empty service namespace",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Namespace = ""
			},
		},
		{
			name: "observed generation mismatch",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Status.ObservedGeneration++
			},
		},
		{
			name: "requested revision mismatch",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Spec.Runtime.Revision = pointerTo("other-revision")
			},
		},
		{
			name: "reported revision mismatch",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Status.PinnedRevisionName = "other-revision"
			},
		},
		{
			name: "explicit runtime mismatch",
			mutate: func(isvc *v1beta1.InferenceService, _ *effective.RuntimeState) {
				isvc.Spec.Runtime.Name = "other-runtime"
			},
		},
		{
			name: "unknown selection source",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.SelectionSource = effective.RuntimeSelectionSource("secret-selection")
			},
		},
		{
			name: "unknown runtime kind",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.RuntimeKind = "secret-runtime-kind"
			},
		},
		{
			name: "unknown pin mode",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.PinMode = effective.RuntimePinMode("secret-pin-mode")
			},
		},
		{
			name: "unknown pin state",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.PinState = effective.RuntimePinState("secret-pin-state")
			},
		},
		{
			name: "unknown freshness",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.StatusFreshness = effective.StatusFreshness("secret-freshness")
			},
		},
		{
			name: "unknown sync state",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.SyncTokenState = effective.SyncTokenState("secret-sync")
			},
		},
		{
			name: "unknown drift state",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.DriftState = effective.RuntimeDriftState("secret-drift")
			},
		},
		{
			name: "unknown drift cause",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.DriftReason = effective.RuntimeDriftReason("secret-drift-reason")
			},
		},
		{
			name: "cause without reported drift",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.DriftReason = effective.RuntimeDriftReasonRevisionMismatch
			},
		},
		{
			name: "unknown hash relation",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.LiveToActive = effective.RuntimeHashRelation("secret-relation")
			},
		},
		{
			name: "unverified live hash",
			mutate: func(_ *v1beta1.InferenceService, state *effective.RuntimeState) {
				state.LiveShortHash = "secret-unverified-hash"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isvc := baseISVC.DeepCopy()
			stateCopy := *baseState
			test.mutate(isvc, &stateCopy)

			_, err := ProjectEffective(isvc, &stateCopy, projectionTestClock)
			require.ErrorIs(t, err, ErrInvalidEvidence)
			assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func pointerTo[T any](value T) *T {
	return &value
}

func resolveLiveProjectionFixture(t *testing.T) (*v1beta1.InferenceService, *effective.RuntimeState) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	base := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "base-runtime", UID: "base-uid", Generation: 2},
		Spec: v1beta1.ServingRuntimeSpec{EngineConfig: &v1beta1.EngineSpec{
			Runner: &v1beta1.RunnerSpec{Container: corev1.Container{Name: "runner", Image: "secret.example/base:latest"}},
		}},
	}
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-runtime", UID: "runtime-uid", Generation: 5,
			Annotations: map[string]string{constants.RuntimeInheritFromAnnotationKey: base.Name},
		},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(base, runtimeObject).Build()
	mode := constants.OMENative
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
		},
		Spec: v1beta1.InferenceServiceSpec{
			DeploymentMode: &mode,
			Runtime:        &v1beta1.ServingRuntimeRef{Name: runtimeObject.Name},
			Engine:         &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset().AppsV1(),
		effective.NewRuntimeResolver(client),
		"ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)
	_, err = state.RequireActive()
	require.NoError(t, err)
	return isvc, state
}
