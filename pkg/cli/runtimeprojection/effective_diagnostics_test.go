package runtimeprojection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/effective"
	"sigs.k8s.io/ome/pkg/cli/paging"
	reportv1alpha1 "sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
)

func TestProjectEffectiveSurfacesBoundedStatusDiagnostics(t *testing.T) {
	isvc, baseState := resolveLiveProjectionFixture(t)
	tests := []struct {
		name         string
		observed     int64
		freshness    effective.StatusFreshness
		drift        effective.RuntimeDriftState
		wantIssues   []reportv1alpha1.RuntimeIssue
		wantWarnings []reportv1alpha1.RuntimeWarning
	}{
		{
			name: "unobserved", observed: 0, freshness: effective.StatusFreshnessUnknown,
			wantIssues: []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusUnobserved}},
		},
		{
			name: "stale", observed: 6, freshness: effective.StatusFreshnessStale,
			wantIssues:   []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusStale}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{{Code: reportv1alpha1.WarningStaleEvidence}},
		},
		{
			name: "invalid", observed: 8, freshness: effective.StatusFreshnessInconsistent,
			wantIssues:   []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusInvalid}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{{Code: reportv1alpha1.WarningStaleEvidence}},
		},
		{
			name: "malformed drift", observed: 7, freshness: effective.StatusFreshnessCurrent,
			drift:      effective.RuntimeDriftStateMalformed,
			wantIssues: []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueReportedDriftConflict}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := *baseState
			isvcCopy := isvc.DeepCopy()
			isvcCopy.Status.ObservedGeneration = test.observed
			state.ObservedGeneration = test.observed
			state.StatusFreshness = test.freshness
			if test.drift != "" {
				state.DriftState = test.drift
			}
			if test.wantWarnings == nil {
				test.wantWarnings = []reportv1alpha1.RuntimeWarning{}
			}

			reportValue, err := ProjectEffective(isvcCopy, &state, projectionTestClock)
			require.NoError(t, err)
			assert.Equal(t, test.wantIssues, reportValue.Content.Issues)
			assert.Equal(t, test.wantWarnings, reportValue.Warnings)
		})
	}
}

func TestProjectEffectiveMapsMissingAndDisabledLiveRuntimeAsEvidence(t *testing.T) {
	tests := []struct {
		name       string
		runtime    *v1beta1.ClusterServingRuntime
		wantReason reportv1alpha1.UnavailableReason
	}{
		{name: "missing", wantReason: reportv1alpha1.UnavailableNotFound},
		{
			name: "disabled",
			runtime: &v1beta1.ClusterServingRuntime{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu-runtime", UID: "disabled-runtime-uid", Generation: 4},
				Spec:       v1beta1.ServingRuntimeSpec{Disabled: pointerTo(true)},
			},
			wantReason: reportv1alpha1.UnavailableDisabled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isvc, state := resolveUnavailableLiveFixture(t, test.runtime)

			reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			assert.Equal(t, reportv1alpha1.RuntimeKindClusterServingRuntime, reportValue.Content.Selection.Runtime.Kind)
			assert.Equal(t, test.wantReason, reportValue.Content.Inheritance.UnavailableReason)
			assert.Equal(t, test.wantReason, reportValue.Content.Live.UnavailableReason)
			assert.Equal(t, test.wantReason, reportValue.Content.Active.UnavailableReason)
			assert.Equal(t, []reportv1alpha1.RuntimeIssue{
				{Code: reportv1alpha1.RuntimeIssueInheritanceUnavailable},
				{Code: reportv1alpha1.RuntimeIssueLiveRuntimeUnavailable},
			}, reportValue.Content.Issues)
			assert.Equal(t, []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData},
				{Code: reportv1alpha1.WarningSourceUnavailable},
			}, reportValue.Warnings)
			require.Len(t, reportValue.Sources, 2)
			assert.Equal(t, reportv1alpha1.EvidenceUnavailable, reportValue.Sources[0].Evidence)
			assert.Equal(t, test.wantReason, reportValue.Sources[0].UnavailableReason)
			assert.Equal(t, "gpu-runtime", reportValue.Sources[0].Name)
		})
	}
}

func resolveUnavailableLiveFixture(
	t *testing.T,
	runtimeObject *v1beta1.ClusterServingRuntime,
) (*v1beta1.InferenceService, *effective.RuntimeState) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	builder := ctrlclientfake.NewClientBuilder().WithScheme(scheme)
	if runtimeObject != nil {
		builder = builder.WithObjects(runtimeObject)
	}
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: "gpu-runtime"},
			Engine:  &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset().AppsV1(),
		effective.NewRuntimeResolver(builder.Build()),
		"ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)
	return isvc, state
}
