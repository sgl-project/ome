package runtimeprojection

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/effective"
	"sigs.k8s.io/ome/pkg/cli/paging"
	reportv1alpha1 "sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimerevision"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

func TestProjectHistoryRejectsNilInputsWithFixedSentinel(t *testing.T) {
	isvc := &v1beta1.InferenceService{}
	state := &effective.RuntimeState{}

	_, err := ProjectHistory(nil, state, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())

	_, err = ProjectHistory(isvc, nil, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
}

func TestProjectHistoryMapsCompleteRetentionBoundedEvidence(t *testing.T) {
	isvc, state, activeRevision, olderRevision := resolveHistoryProjectionFixture(t)

	reportValue, err := ProjectHistory(isvc, state, projectionTestClock)
	require.NoError(t, err)

	activeCreatedAt := activeRevision.CreationTimestamp.Time.UTC()
	olderCreatedAt := olderRevision.CreationTimestamp.Time.UTC()
	assert.Equal(t, reportv1alpha1.RuntimeHistoryContent{
		Runtime: &reportv1alpha1.RuntimeObjectReference{
			APIVersion: v1beta1.SchemeGroupVersion.String(),
			Kind:       reportv1alpha1.RuntimeKindClusterServingRuntime,
			Name:       "gpu-runtime",
			UID:        "runtime-uid",
			Generation: 5,
		},
		Observation:    reportv1alpha1.HistoryObservationStateComplete,
		Completeness:   reportv1alpha1.HistoryCompletenessRetentionBounded,
		RequestedPages: 1,
		ObservedPages:  1,
		Revisions: []reportv1alpha1.RuntimeRevisionEntry{
			{
				Revision: reportv1alpha1.RuntimeRevisionReference{
					Namespace: "ome-system", Name: activeRevision.Name, UID: "active-uid", CreatedAt: &activeCreatedAt,
				},
				Source: &reportv1alpha1.RuntimeObjectReference{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       reportv1alpha1.RuntimeKindClusterServingRuntime,
					Name:       "gpu-runtime",
				},
				Hash: activeRevision.Labels[constants.RuntimeRevisionHashLabelKey],
				Roles: []reportv1alpha1.RuntimeRevisionRole{
					reportv1alpha1.RuntimeRevisionRoleActive,
					reportv1alpha1.RuntimeRevisionRoleReported,
					reportv1alpha1.RuntimeRevisionRoleHistory,
				},
				Consistency:    reportv1alpha1.RevisionConsistencyConsistent,
				RelationToLive: reportv1alpha1.RevisionRelationMatchesLive,
				Issues:         []reportv1alpha1.RuntimeIssueCode{},
			},
			{
				Revision: reportv1alpha1.RuntimeRevisionReference{
					Namespace: "ome-system", Name: olderRevision.Name, UID: "older-uid", CreatedAt: &olderCreatedAt,
				},
				Source: &reportv1alpha1.RuntimeObjectReference{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       reportv1alpha1.RuntimeKindClusterServingRuntime,
					Name:       "gpu-runtime",
				},
				Hash:           olderRevision.Labels[constants.RuntimeRevisionHashLabelKey],
				Roles:          []reportv1alpha1.RuntimeRevisionRole{reportv1alpha1.RuntimeRevisionRoleHistory},
				Consistency:    reportv1alpha1.RevisionConsistencyConsistent,
				RelationToLive: reportv1alpha1.RevisionRelationDiffersFromLive,
				Issues:         []reportv1alpha1.RuntimeIssueCode{},
			},
		},
		Issues: []reportv1alpha1.RuntimeIssue{},
	}, reportValue.Content)
	assert.Equal(t, []reportv1alpha1.RuntimeSourceReference{
		{
			Kind: "ClusterServingRuntime", Name: "gpu-runtime", UID: "runtime-uid", Generation: 5,
			Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
		{
			Kind: "ControllerRevision", Namespace: "ome-system", Name: activeRevision.Name, UID: "active-uid",
			Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
		{
			Kind: "ControllerRevision", Namespace: "ome-system", Name: olderRevision.Name, UID: "older-uid",
			Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
		{
			Kind: "InferenceService", Namespace: "workloads", Name: "chat", UID: "isvc-uid", Generation: 7,
			Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
		},
	}, reportValue.Sources)
	assert.Empty(t, reportValue.Warnings)
}

func TestProjectHistoryMapsCollectionStateWithoutInventingCompleteness(t *testing.T) {
	isvc, baseState := resolveLiveProjectionFixture(t)
	tests := []struct {
		name             string
		requested        bool
		complete         bool
		truncated        bool
		requestedPages   int
		observedPages    int
		wantObservation  reportv1alpha1.HistoryObservationState
		wantCompleteness reportv1alpha1.HistoryCompleteness
		wantIssues       []reportv1alpha1.RuntimeIssue
		wantWarnings     []reportv1alpha1.RuntimeWarning
	}{
		{
			name: "not requested", wantObservation: reportv1alpha1.HistoryObservationStateNotRequested,
			wantCompleteness: reportv1alpha1.HistoryCompletenessNotRequested,
		},
		{
			name: "complete is retention bounded", requested: true, complete: true, requestedPages: 2, observedPages: 2,
			wantObservation:  reportv1alpha1.HistoryObservationStateComplete,
			wantCompleteness: reportv1alpha1.HistoryCompletenessRetentionBounded,
		},
		{
			name: "partial after one page", requested: true, requestedPages: 2, observedPages: 1,
			wantObservation:  reportv1alpha1.HistoryObservationStatePartial,
			wantCompleteness: reportv1alpha1.HistoryCompletenessIncomplete,
			wantIssues:       []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueHistoryUnavailable}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData}, {Code: reportv1alpha1.WarningSourceUnavailable},
			},
		},
		{
			name: "unavailable before first page", requested: true, requestedPages: 1,
			wantObservation:  reportv1alpha1.HistoryObservationStateUnavailable,
			wantCompleteness: reportv1alpha1.HistoryCompletenessIncomplete,
			wantIssues:       []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueHistoryUnavailable}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData}, {Code: reportv1alpha1.WarningSourceUnavailable},
			},
		},
		{
			name: "truncated", requested: true, truncated: true, requestedPages: 2, observedPages: 2,
			wantObservation:  reportv1alpha1.HistoryObservationStatePartial,
			wantCompleteness: reportv1alpha1.HistoryCompletenessIncomplete,
			wantIssues:       []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueHistoryTruncated}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData}, {Code: reportv1alpha1.WarningTruncated},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := *baseState
			state.HistoryRequested = test.requested
			state.HistoryComplete = test.complete
			state.HistoryTruncated = test.truncated
			state.HistoryRequestedPages = test.requestedPages
			state.HistoryObservedPages = test.observedPages

			reportValue, err := ProjectHistory(isvc, &state, projectionTestClock)
			require.NoError(t, err)
			if test.wantIssues == nil {
				test.wantIssues = []reportv1alpha1.RuntimeIssue{}
			}
			if test.wantWarnings == nil {
				test.wantWarnings = []reportv1alpha1.RuntimeWarning{}
			}
			assert.Equal(t, test.wantObservation, reportValue.Content.Observation)
			assert.Equal(t, test.wantCompleteness, reportValue.Content.Completeness)
			assert.Equal(t, test.requestedPages, reportValue.Content.RequestedPages)
			assert.Equal(t, test.observedPages, reportValue.Content.ObservedPages)
			assert.Equal(t, test.wantIssues, reportValue.Content.Issues)
			assert.Equal(t, test.wantWarnings, reportValue.Warnings)
		})
	}
}

func resolveHistoryProjectionFixture(t *testing.T) (
	*v1beta1.InferenceService,
	*effective.RuntimeState,
	*appsv1.ControllerRevision,
	*appsv1.ControllerRevision,
) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-runtime", UID: "runtime-uid", Generation: 5},
		Spec:       *liveSpec.DeepCopy(),
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(runtimeObject).Build()
	activeRevision := projectionRevision(t, "active-uid", "gpu-runtime", liveSpec)
	activeRevision.CreationTimestamp = metav1.NewTime(time.Date(2026, time.August, 31, 19, 0, 0, 0, time.UTC))
	olderRevision := projectionRevision(t, "older-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/older:secret"))
	olderRevision.CreationTimestamp = metav1.NewTime(time.Date(2026, time.August, 30, 19, 0, 0, 0, time.UTC))
	autoSync := false
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: runtimeObject.Name, AutoSync: &autoSync},
			Engine:  &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	isvc.Status.PinnedRevisionName = activeRevision.Name
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset(activeRevision.DeepCopy(), olderRevision.DeepCopy()).AppsV1(),
		effective.NewRuntimeResolver(client),
		"ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{IncludeHistory: true})
	require.NoError(t, err)
	return isvc, state, activeRevision, olderRevision
}

func projectionRuntimeSpec(image string) *v1beta1.ServingRuntimeSpec {
	return &v1beta1.ServingRuntimeSpec{
		EngineConfig: &v1beta1.EngineSpec{Runner: &v1beta1.RunnerSpec{
			Container: corev1.Container{
				Name: "runner", Image: image, Command: []string{"secret-command"},
				Env: []corev1.EnvVar{{Name: "SECRET_ENV", Value: "secret-value"}},
			},
		}},
	}
}

func projectionRevision(
	t *testing.T,
	uid, runtimeName string,
	spec *v1beta1.ServingRuntimeSpec,
) *appsv1.ControllerRevision {
	t.Helper()
	_, shortHash, err := runtimerevision.Hash(spec)
	require.NoError(t, err)
	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	name := runtimerevision.Name(runtimerevision.KindClusterServingRuntime, "", runtimeName, shortHash)
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "ome-system", UID: typesUID(uid),
			Labels: map[string]string{
				constants.RuntimeRevisionOfLabelKey:          runtimeName,
				constants.RuntimeRevisionOfKindLabelKey:      runtimeselector.KindClusterServingRuntime,
				constants.RuntimeRevisionOfNamespaceLabelKey: "",
				constants.RuntimeRevisionHashLabelKey:        shortHash,
			},
			Annotations: map[string]string{
				constants.RuntimeRevisionCreatedByKey: constants.RuntimeRevisionCreatedByOMEValue,
			},
		},
		Data:     runtime.RawExtension{Raw: raw},
		Revision: 1,
	}
}

func typesUID(value string) types.UID { return types.UID(value) }
