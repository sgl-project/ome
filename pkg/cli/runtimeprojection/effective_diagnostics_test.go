package runtimeprojection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	appstyped "k8s.io/client-go/kubernetes/typed/apps/v1"
	k8stesting "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/effective"
	"sigs.k8s.io/ome/pkg/cli/paging"
	"sigs.k8s.io/ome/pkg/cli/report"
	reportv1alpha1 "sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
	"sigs.k8s.io/ome/pkg/constants"
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
			wantIssues:   []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusUnobserved}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{{Code: reportv1alpha1.WarningPartialData}},
		},
		{
			name: "stale", observed: 6, freshness: effective.StatusFreshnessStale,
			wantIssues: []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusStale}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData}, {Code: reportv1alpha1.WarningStaleEvidence},
			},
		},
		{
			name: "invalid", observed: 8, freshness: effective.StatusFreshnessInconsistent,
			wantIssues: []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueStatusInvalid}},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData}, {Code: reportv1alpha1.WarningStaleEvidence},
			},
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
				setMalformedProjectionDrift(isvcCopy)
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

func TestProjectionRejectsFabricatedActualScopeWithoutLiveSnapshot(t *testing.T) {
	isvc, baseState := resolveUnavailableLiveFixture(t, nil)
	tests := []struct {
		name   string
		mutate func(*effective.RuntimeState)
	}{
		{
			name: "actual kind without live object",
			mutate: func(state *effective.RuntimeState) {
				state.RuntimeKind = "ServingRuntime"
			},
		},
		{
			name: "actual namespace without live object",
			mutate: func(state *effective.RuntimeState) {
				state.RuntimeNamespace = "workloads"
			},
		},
		{
			name: "live hash without live object",
			mutate: func(state *effective.RuntimeState) {
				state.LiveShortHash = "deadbeef"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := *baseState
			test.mutate(&state)

			_, err := ProjectEffective(isvc, &state, projectionTestClock)
			require.ErrorIs(t, err, ErrInvalidEvidence)
			assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
			_, err = ProjectHistory(isvc, &state, projectionTestClock)
			require.ErrorIs(t, err, ErrInvalidEvidence)
			assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
		})
	}
}

func TestProjectEffectiveRetainsLiveIdentityWhenInheritanceEvidenceIsUnavailable(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-runtime", UID: "runtime-uid", Generation: 5,
			Annotations: map[string]string{constants.RuntimeInheritFromAnnotationKey: "missing-parent"},
		},
		Spec: *projectionRuntimeSpec("private.registry/live:secret"),
	}
	runtimeObject.Spec.SupportedModelFormats = []v1beta1.SupportedModelFormat{{
		ModelFormat: &v1beta1.ModelFormat{Name: "pytorch", Weight: 10}, AutoSelect: pointerTo(true),
	}}
	model := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "model", Namespace: "workloads", UID: "model-uid", Generation: 2},
		Spec:       v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: "pytorch"}},
	}
	liveClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(model, runtimeObject).Build()
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "secret-isvc-resource-version",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Model: &v1beta1.ModelRef{Name: model.Name}, Engine: &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset().AppsV1(), effective.NewRuntimeResolver(liveClient), "ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)
	require.Equal(t, effective.LiveRuntimeAvailable, state.LiveAvailability())
	require.NotNil(t, state.LiveConfiguration())

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.InheritanceStateUnavailable, reportValue.Content.Inheritance.State)
	assert.Equal(t, reportv1alpha1.UnavailableNotFound, reportValue.Content.Inheritance.UnavailableReason)
	assert.Equal(t, reportv1alpha1.ConfigurationStateAvailable, reportValue.Content.Live.State)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueInheritanceUnavailable,
	})
	assert.Equal(t, []reportv1alpha1.RuntimeWarning{
		{Code: reportv1alpha1.WarningPartialData},
		{Code: reportv1alpha1.WarningSourceUnavailable},
	}, reportValue.Warnings)
	assert.Contains(t, reportValue.Sources, reportv1alpha1.RuntimeSourceReference{
		Kind: "ClusterServingRuntime", Name: runtimeObject.Name,
		Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
	})
}

func TestProjectEffectiveProjectsOnlyTheExpectedActiveRevisionIdentity(t *testing.T) {
	isvc, state, activeRevision, _ := resolveHistoryProjectionFixture(t)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	require.NotNil(t, reportValue.Content.Active.Revision)
	activeCreatedAt := activeRevision.CreationTimestamp.Time.UTC()
	assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
		Namespace: "ome-system", Name: activeRevision.Name, UID: "active-uid", CreatedAt: &activeCreatedAt,
	}, reportValue.Content.Active.Revision)
	assert.Equal(t, reportv1alpha1.ConfigurationOriginControllerRevision, reportValue.Content.Active.Origin)
	assert.Equal(t, activeRevision.Labels[constants.RuntimeRevisionHashLabelKey], reportValue.Content.Active.Hash)
	assert.Empty(t, reportValue.Content.Issues)
	var revisionSources []reportv1alpha1.RuntimeSourceReference
	for _, source := range reportValue.Sources {
		if source.Kind == "ControllerRevision" {
			revisionSources = append(revisionSources, source)
		}
	}
	assert.Equal(t, []reportv1alpha1.RuntimeSourceReference{{
		Kind: "ControllerRevision", Namespace: "ome-system", Name: activeRevision.Name, UID: "active-uid",
		Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
	}}, revisionSources, "history-only revisions must not leak into the effective report")
}

func TestProjectEffectiveSeparatesExpectedActiveKeyFromMismatchedReturnedIdentity(t *testing.T) {
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	returned := projectionRevision(t, "returned-uid", "gpu-runtime", liveSpec)
	returned.Name = "returned-other-revision"
	returned.CreationTimestamp = metav1.NewTime(time.Date(2026, time.August, 31, 19, 0, 0, 0, time.UTC))
	clientset := k8sfake.NewSimpleClientset(returned.DeepCopy())
	clientset.PrependReactor("get", "controllerrevisions", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, returned.DeepCopy(), nil
	})
	expectedName := "expected-revision"
	isvc, state := resolveProjectionWithRevisionClient(t, clientset, liveSpec, expectedName, nil)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
		Namespace: "ome-system", Name: expectedName,
	}, reportValue.Content.Active.Revision)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueRevisionIdentityMismatch, Revision: expectedName,
	})
	assert.Contains(t, reportValue.Sources, reportv1alpha1.RuntimeSourceReference{
		Kind: "ControllerRevision", Namespace: "ome-system", Name: returned.Name, UID: "returned-uid",
		Evidence: reportv1alpha1.EvidenceObserved, CollectedAt: projectionTestClock.Now(),
	})
}

func TestProjectEffectivePreservesDeclaredPinScopeAcrossNamespacedLiveFallback(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	runtimeObject := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "same-name", Namespace: "workloads", UID: "namespaced-uid", Generation: 4},
		Spec:       *projectionRuntimeSpec("private.registry/namespaced:secret"),
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(runtimeObject).Build()
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "secret-isvc-resource-version",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: runtimeObject.Name},
			Engine:  &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset().AppsV1(), effective.NewRuntimeResolver(client), "ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(context.Background(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.RuntimeKindServingRuntime, reportValue.Content.Selection.Runtime.Kind)
	assert.Equal(t, "workloads", reportValue.Content.Selection.Runtime.Namespace)
	assert.Equal(t, "namespaced-uid", reportValue.Content.Selection.Runtime.UID)
	assert.Equal(t, reportv1alpha1.RuntimeKindServingRuntime, reportValue.Content.Live.Source.Kind)
	assert.Equal(t, "workloads", reportValue.Content.Live.Source.Namespace)
	assert.Equal(t, "ClusterServingRuntime", state.DeclaredSourceKind)
}

func TestProjectEffectiveDistinguishesNotConfiguredFromFailedAutoSelection(t *testing.T) {
	tests := []struct {
		name         string
		model        *v1beta1.BaseModel
		modelRef     *v1beta1.ModelRef
		wantReason   reportv1alpha1.UnavailableReason
		wantIssues   []reportv1alpha1.RuntimeIssue
		wantWarnings []reportv1alpha1.RuntimeWarning
	}{
		{
			name: "no runtime or model configured", wantReason: reportv1alpha1.UnavailableNotConfigured,
			wantIssues: []reportv1alpha1.RuntimeIssue{{Code: reportv1alpha1.RuntimeIssueInheritanceUnavailable}},
		},
		{
			name: "configured model has no selectable runtime",
			model: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{Name: "model", Namespace: "workloads", UID: "model-uid", Generation: 2},
			},
			modelRef:   &v1beta1.ModelRef{Name: "model"},
			wantReason: reportv1alpha1.UnavailableUnreadable,
			wantIssues: []reportv1alpha1.RuntimeIssue{
				{Code: reportv1alpha1.RuntimeIssueInheritanceUnavailable},
				{Code: reportv1alpha1.RuntimeIssueLiveRuntimeUnavailable},
			},
			wantWarnings: []reportv1alpha1.RuntimeWarning{
				{Code: reportv1alpha1.WarningPartialData},
				{Code: reportv1alpha1.WarningSourceUnavailable},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			builder := ctrlclientfake.NewClientBuilder().WithScheme(scheme)
			if test.model != nil {
				builder = builder.WithObjects(test.model)
			}
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
					ResourceVersion: "secret-isvc-resource-version",
				},
				Spec: v1beta1.InferenceServiceSpec{Model: test.modelRef, Engine: &v1beta1.EngineSpec{}},
			}
			isvc.Status.ObservedGeneration = 7
			resolver, err := effective.NewRuntimePinResolver(
				k8sfake.NewSimpleClientset().AppsV1(), effective.NewRuntimeResolver(builder.Build()), "ome-system",
				paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
			)
			require.NoError(t, err)
			state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
			require.NoError(t, err)

			reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			assert.Nil(t, reportValue.Content.Selection.Runtime)
			assert.Equal(t, test.wantReason, reportValue.Content.Inheritance.UnavailableReason)
			assert.Equal(t, test.wantReason, reportValue.Content.Live.UnavailableReason)
			assert.Equal(t, test.wantReason, reportValue.Content.Active.UnavailableReason)
			assert.Equal(t, test.wantIssues, reportValue.Content.Issues)
			if test.wantWarnings == nil {
				test.wantWarnings = []reportv1alpha1.RuntimeWarning{}
			}
			assert.Equal(t, test.wantWarnings, reportValue.Warnings)
		})
	}
}

func TestProjectEffectiveClassifiesLiveReadFailuresWithoutCopyingErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason reportv1alpha1.UnavailableReason
	}{
		{
			name: "forbidden",
			err: apierrors.NewForbidden(
				schema.GroupResource{Group: v1beta1.SchemeGroupVersion.Group, Resource: "clusterservingruntimes"},
				"gpu-runtime", errors.New("secret-forbidden-cause"),
			),
			wantReason: reportv1alpha1.UnavailableForbidden,
		},
		{
			name: "unsupported api",
			err: &meta.NoKindMatchError{
				GroupKind:        schema.GroupKind{Group: v1beta1.SchemeGroupVersion.Group, Kind: "ClusterServingRuntime"},
				SearchedVersions: []string{"secret-version"},
			},
			wantReason: reportv1alpha1.UnavailableUnsupportedAPI,
		},
		{name: "unreadable", err: errors.New("secret-unreadable-cause"), wantReason: reportv1alpha1.UnavailableUnreadable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			base := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()
			client := projectionGetErrorClient{Client: base, err: test.err}
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
					ResourceVersion: "secret-isvc-resource-version",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "gpu-runtime"}, Engine: &v1beta1.EngineSpec{},
				},
			}
			isvc.Status.ObservedGeneration = 7
			resolver, err := effective.NewRuntimePinResolver(
				k8sfake.NewSimpleClientset().AppsV1(), effective.NewRuntimeResolver(client), "ome-system",
				paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
			)
			require.NoError(t, err)
			state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
			require.NoError(t, err)

			reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			assert.Equal(t, test.wantReason, reportValue.Content.Live.UnavailableReason)
			assert.Equal(t, test.wantReason, reportValue.Content.Inheritance.UnavailableReason)
			assert.NotContains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueHistoryUnavailable})
		})
	}
}

func TestProjectEffectiveProjectsFailedExpectedRevisionWithoutFabricatedMetadata(t *testing.T) {
	tests := []struct {
		name       string
		reactor    func(k8stesting.Action) (bool, runtime.Object, error)
		wantReason reportv1alpha1.UnavailableReason
		wantIssue  reportv1alpha1.RuntimeIssueCode
	}{
		{
			name: "not found",
			reactor: func(action k8stesting.Action) (bool, runtime.Object, error) {
				get := action.(k8stesting.GetAction)
				return true, nil, apierrors.NewNotFound(
					schema.GroupResource{Group: "apps", Resource: "controllerrevisions"}, get.GetName(),
				)
			},
			wantReason: reportv1alpha1.UnavailableNotFound,
			wantIssue:  reportv1alpha1.RuntimeIssueRevisionNotFound,
		},
		{
			name: "forbidden",
			reactor: func(action k8stesting.Action) (bool, runtime.Object, error) {
				get := action.(k8stesting.GetAction)
				return true, nil, apierrors.NewForbidden(
					schema.GroupResource{Group: "apps", Resource: "controllerrevisions"},
					get.GetName(), errors.New("secret-revision-denial"),
				)
			},
			wantReason: reportv1alpha1.UnavailableForbidden,
			wantIssue:  reportv1alpha1.RuntimeIssueRevisionUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientset := k8sfake.NewSimpleClientset()
			clientset.PrependReactor("get", "controllerrevisions", test.reactor)
			liveSpec := projectionRuntimeSpec("private.registry/live:secret")
			expected := "expected-revision"
			isvc, state := resolveProjectionWithRevisionClient(t, clientset, liveSpec, expected, nil)

			reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			assert.Equal(t, reportv1alpha1.ConfigurationStateUnavailable, reportValue.Content.Active.State)
			assert.Equal(t, test.wantReason, reportValue.Content.Active.UnavailableReason)
			assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
				Namespace: "ome-system", Name: expected,
			}, reportValue.Content.Active.Revision)
			assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
				Code: reportv1alpha1.RuntimeIssueActiveRevisionUnavailable,
			})
			assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
				Code: test.wantIssue, Revision: expected,
			})
			assert.Contains(t, reportValue.Sources, reportv1alpha1.RuntimeSourceReference{
				Kind: "ControllerRevision", Namespace: "ome-system", Name: expected,
				Evidence: reportv1alpha1.EvidenceUnavailable, CollectedAt: projectionTestClock.Now(),
				UnavailableReason: test.wantReason,
			})
		})
	}
}

func TestProjectEffectiveClassifiesNilRevisionGetAsUnavailable(t *testing.T) {
	base := k8sfake.NewSimpleClientset().AppsV1()
	revisions := nilGetRevisionsGetter{delegate: base}
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	expected := "expected-revision"
	isvc, state := resolveProjectionWithRevisionGetter(t, revisions, liveSpec, expected, nil)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, effective.RuntimePinStateUnavailable, state.PinState)
	assert.Equal(t, reportv1alpha1.UnavailableUnreadable, reportValue.Content.Active.UnavailableReason)
	assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
		Namespace: "ome-system", Name: expected,
	}, reportValue.Content.Active.Revision)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueRevisionUnavailable, Revision: expected,
	})
	assert.Contains(t, reportValue.Sources, reportv1alpha1.RuntimeSourceReference{
		Kind: "ControllerRevision", Namespace: "ome-system", Name: expected,
		Evidence: reportv1alpha1.EvidenceUnavailable, CollectedAt: projectionTestClock.Now(),
		UnavailableReason: reportv1alpha1.UnavailableUnreadable,
	})
}

type nilGetRevisionsGetter struct {
	delegate appstyped.ControllerRevisionsGetter
}

func (getter nilGetRevisionsGetter) ControllerRevisions(namespace string) appstyped.ControllerRevisionInterface {
	return nilGetRevisionInterface{ControllerRevisionInterface: getter.delegate.ControllerRevisions(namespace)}
}

type nilGetRevisionInterface struct {
	appstyped.ControllerRevisionInterface
}

func (client nilGetRevisionInterface) Get(
	context.Context,
	string,
	metav1.GetOptions,
) (*appsv1.ControllerRevision, error) {
	return nil, nil
}

func TestProjectEffectiveDoesNotActivateRetainedPinWhenLiveRuntimeIsDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-runtime", UID: "disabled-runtime-uid", Generation: 5},
		Spec:       v1beta1.ServingRuntimeSpec{Disabled: pointerTo(true)},
	}
	liveClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(runtimeObject).Build()
	revision := projectionRevision(t, "retained-uid", runtimeObject.Name, projectionRuntimeSpec("private.registry/retained:secret"))
	autoSync := false
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "secret-isvc-resource-version",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: runtimeObject.Name, AutoSync: &autoSync},
			Engine:  &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	isvc.Status.PinnedRevisionName = revision.Name
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset(revision.DeepCopy()).AppsV1(), effective.NewRuntimeResolver(liveClient), "ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, effective.RuntimePinStateUnavailable, state.PinState)
	_, err = state.RequireActive()
	require.ErrorIs(t, err, effective.ErrActiveRuntimeUnavailable)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)
	assert.Equal(t, reportv1alpha1.UnavailableDisabled, reportValue.Content.Live.UnavailableReason)
	assert.Equal(t, reportv1alpha1.UnavailableDisabled, reportValue.Content.Active.UnavailableReason)
	assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
		Namespace: "ome-system", Name: revision.Name, UID: "retained-uid",
	}, reportValue.Content.Active.Revision)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueActiveRevisionUnavailable,
	})
}

func TestProjectionRejectsRevisionPinStatesWhenLiveReadPreemptsPinResolution(t *testing.T) {
	tests := []struct {
		name       string
		liveClient func(*testing.T) ctrlclient.Client
		explicit   bool
		wantMode   effective.RuntimePinMode
	}{
		{
			name: "managed pin with disabled live", liveClient: disabledProjectionRuntimeClient,
			wantMode: effective.RuntimePinModeManagedPin,
		},
		{
			name: "managed pin with unreadable live", liveClient: unreadableProjectionRuntimeClient,
			wantMode: effective.RuntimePinModeManagedPin,
		},
		{
			name: "explicit pin with disabled live", liveClient: disabledProjectionRuntimeClient,
			explicit: true, wantMode: effective.RuntimePinModeExplicitPin,
		},
		{
			name: "explicit pin with unreadable live", liveClient: unreadableProjectionRuntimeClient,
			explicit: true, wantMode: effective.RuntimePinModeExplicitPin,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			autoSync := false
			revision := projectionRevision(
				t, "revision-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/revision:secret"),
			)
			runtimeReference := &v1beta1.ServingRuntimeRef{Name: "gpu-runtime", AutoSync: &autoSync}
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
					ResourceVersion: "secret-isvc-resource-version",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: runtimeReference, Engine: &v1beta1.EngineSpec{},
				},
			}
			isvc.Status.ObservedGeneration = 7
			if test.explicit {
				runtimeReference.Revision = pointerTo(revision.Name)
			} else {
				isvc.Status.PinnedRevisionName = revision.Name
			}
			resolver, err := effective.NewRuntimePinResolver(
				k8sfake.NewSimpleClientset(revision.DeepCopy()).AppsV1(),
				effective.NewRuntimeResolver(test.liveClient(t)),
				"ome-system",
				paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
			)
			require.NoError(t, err)
			state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
			require.NoError(t, err)
			require.Equal(t, test.wantMode, state.PinMode)
			require.Equal(t, effective.RuntimePinStateUnavailable, state.PinState)
			_, err = state.RequireActive()
			require.ErrorIs(t, err, effective.ErrActiveRuntimeUnavailable)
			_, err = ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			_, err = ProjectHistory(isvc, state, projectionTestClock)
			require.NoError(t, err)

			for _, forgedState := range []effective.RuntimePinState{
				effective.RuntimePinStateRevisionMissing,
				effective.RuntimePinStateRevisionInvalid,
				effective.RuntimePinStateRevisionDisabled,
			} {
				contradictory := *state
				contradictory.PinState = forgedState
				_, err = ProjectEffective(isvc, &contradictory, projectionTestClock)
				require.ErrorIs(t, err, ErrInvalidEvidence, "pin state %s", forgedState)
				assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
				_, err = ProjectHistory(isvc, &contradictory, projectionTestClock)
				require.ErrorIs(t, err, ErrInvalidEvidence, "pin state %s", forgedState)
				assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
			}
		})
	}
}

func TestProjectEffectiveRetainsVerifiedPinWhenLiveRuntimeIsMissing(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	liveClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()
	revision := projectionRevision(
		t, "retained-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/retained:secret"),
	)
	revision.CreationTimestamp = metav1.NewTime(time.Date(2026, time.August, 31, 19, 0, 0, 0, time.UTC))
	autoSync := false
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "secret-isvc-resource-version",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{Name: "gpu-runtime", AutoSync: &autoSync},
			Engine:  &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	isvc.Status.PinnedRevisionName = revision.Name
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset(revision.DeepCopy()).AppsV1(), effective.NewRuntimeResolver(liveClient), "ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, effective.RuntimePinStateResolved, state.PinState)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.UnavailableNotFound, reportValue.Content.Live.UnavailableReason)
	assert.Equal(t, reportv1alpha1.ConfigurationStateAvailable, reportValue.Content.Active.State)
	assert.Equal(t, reportv1alpha1.ConfigurationOriginControllerRevision, reportValue.Content.Active.Origin)
	assert.Equal(t, reportv1alpha1.RuntimeHashRelationUnknown, reportValue.Content.LiveToActive)
	createdAt := revision.CreationTimestamp.Time.UTC()
	assert.Equal(t, &reportv1alpha1.RuntimeRevisionReference{
		Namespace: "ome-system", Name: revision.Name, UID: "retained-uid", CreatedAt: &createdAt,
	}, reportValue.Content.Active.Revision)
	assert.Equal(t, revision.Labels[constants.RuntimeRevisionHashLabelKey], reportValue.Content.Active.Hash)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueLiveRuntimeUnavailable,
	})
}

func TestProjectEffectiveProjectsManagedAwaitingPin(t *testing.T) {
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	isvc, state := resolveProjectionWithRevisionClient(t, k8sfake.NewSimpleClientset(), liveSpec, "", nil)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.RuntimePinModeManagedPin, reportValue.Content.Pin.Mode)
	assert.Equal(t, reportv1alpha1.RuntimePinStateAwaitingPin, reportValue.Content.Pin.State)
	assert.Equal(t, reportv1alpha1.ConfigurationOriginLiveRuntime, reportValue.Content.Active.Origin)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueActiveRevisionUnreported,
	})
}

func TestProjectEffectiveProjectsExplicitDesiredReportedMismatchWithoutInventingDrift(t *testing.T) {
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	requested := projectionRevision(t, "requested-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/requested:secret"))
	reported := projectionRevision(t, "reported-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/reported:secret"))
	requestedName := requested.Name
	isvc, state := resolveProjectionWithRevisionClient(
		t, k8sfake.NewSimpleClientset(requested.DeepCopy(), reported.DeepCopy()),
		liveSpec, reported.Name, &requestedName,
	)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.RuntimePinModeExplicitPin, reportValue.Content.Pin.Mode)
	assert.Equal(t, reportv1alpha1.RuntimePinStateDesiredReportedMismatch, reportValue.Content.Pin.State)
	assert.Equal(t, requested.Name, reportValue.Content.Pin.RequestedRevision)
	assert.Equal(t, reported.Name, reportValue.Content.Pin.ReportedRevision)
	require.NotNil(t, reportValue.Content.Active.Revision)
	assert.Equal(t, requested.Name, reportValue.Content.Active.Revision.Name)
	assert.Equal(t, reportv1alpha1.RuntimeHashRelationDifferent, reportValue.Content.LiveToActive)
	assert.NotContains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueReportedDriftConflict,
	})
}

func TestProjectionRejectsRevisionRelationContradictingVerifiedShortHashes(t *testing.T) {
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	revision := projectionRevision(
		t, "revision-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/revision:secret"),
	)
	isvc, state := resolveProjectionWithRevisionClient(
		t, k8sfake.NewSimpleClientset(revision.DeepCopy()), liveSpec, revision.Name, nil,
	)
	require.NotEqual(t, state.LiveShortHash, revision.Labels[constants.RuntimeRevisionHashLabelKey])
	require.Equal(t, effective.RuntimeHashRelationDifferent, state.LiveToActive)

	state.LiveToActive = effective.RuntimeHashRelationEqual
	_, err := ProjectEffective(isvc, state, projectionTestClock)
	require.ErrorIs(t, err, ErrInvalidEvidence)
	assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
}

func TestProjectEffectiveProjectsInvalidAndDisabledRevisionEvidence(t *testing.T) {
	liveSpec := projectionRuntimeSpec("private.registry/live:secret")
	disabledSpec := projectionRuntimeSpec("private.registry/disabled:secret")
	disabledSpec.Disabled = pointerTo(true)
	disabled := projectionRevision(t, "disabled-uid", "gpu-runtime", disabledSpec)
	malformed := projectionRevision(t, "malformed-uid", "gpu-runtime", projectionRuntimeSpec("private.registry/malformed:secret"))
	malformed.Data.Raw = []byte(`{"private":"secret-malformed-payload"`)
	tests := []struct {
		name       string
		revision   *appsv1.ControllerRevision
		wantState  reportv1alpha1.RuntimePinState
		wantReason reportv1alpha1.UnavailableReason
		wantIssue  reportv1alpha1.RuntimeIssueCode
	}{
		{
			name: "disabled", revision: disabled, wantState: reportv1alpha1.RuntimePinStateRevisionDisabled,
			wantReason: reportv1alpha1.UnavailableDisabled, wantIssue: reportv1alpha1.RuntimeIssueRevisionDisabled,
		},
		{
			name: "malformed", revision: malformed, wantState: reportv1alpha1.RuntimePinStateRevisionInvalid,
			wantReason: reportv1alpha1.UnavailableMalformedPayload, wantIssue: reportv1alpha1.RuntimeIssueRevisionPayloadMalformed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isvc, state := resolveProjectionWithRevisionClient(
				t, k8sfake.NewSimpleClientset(test.revision.DeepCopy()), liveSpec, test.revision.Name, nil,
			)

			reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			assert.Equal(t, test.wantState, reportValue.Content.Pin.State)
			assert.Equal(t, reportv1alpha1.ConfigurationStateUnavailable, reportValue.Content.Active.State)
			assert.Equal(t, test.wantReason, reportValue.Content.Active.UnavailableReason)
			assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
				Code: test.wantIssue, Revision: test.revision.Name,
			})
		})
	}
}

func TestProjectEffectiveProjectsInvalidPinIntentWithoutEchoingDeclaredKind(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-runtime", UID: "runtime-uid", Generation: 5},
		Spec:       *projectionRuntimeSpec("private.registry/live:secret"),
	}
	liveClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(runtimeObject).Build()
	autoSync := false
	hostileKind := "secret-unsupported-runtime-kind"
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "secret-isvc-resource-version",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime: &v1beta1.ServingRuntimeRef{
				Name: runtimeObject.Name, Kind: &hostileKind, AutoSync: &autoSync,
			},
			Engine: &v1beta1.EngineSpec{},
		},
	}
	isvc.Status.ObservedGeneration = 7
	resolver, err := effective.NewRuntimePinResolver(
		k8sfake.NewSimpleClientset().AppsV1(), effective.NewRuntimeResolver(liveClient), "ome-system",
		paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
	)
	require.NoError(t, err)
	state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
	require.NoError(t, err)

	reportValue, err := ProjectEffective(isvc, state, projectionTestClock)
	require.NoError(t, err)

	assert.Equal(t, reportv1alpha1.RuntimePinModeInvalidPin, reportValue.Content.Pin.Mode)
	assert.Equal(t, reportv1alpha1.RuntimePinStateInvalidIntent, reportValue.Content.Pin.State)
	assert.Equal(t, reportv1alpha1.ConfigurationStateUnavailable, reportValue.Content.Active.State)
	assert.Equal(t, reportv1alpha1.UnavailableMalformedPayload, reportValue.Content.Active.UnavailableReason)
	assert.Contains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueInvalidDeclaredKind,
	})
	assert.NotContains(t, reportValue.Content.Issues, reportv1alpha1.RuntimeIssue{
		Code: reportv1alpha1.RuntimeIssueActiveRevisionUnavailable,
	})
	for _, format := range []report.Format{report.FormatTable, report.FormatJSON, report.FormatYAML} {
		assert.NotContains(t, renderProjectionDocument(t, format, reportValue), hostileKind)
	}
}

func TestProjectionSafelyReportsInvalidDeclaredKindWhenLiveIsUnavailable(t *testing.T) {
	autoSync := false
	tests := []struct {
		name         string
		kind         string
		liveClient   func(*testing.T) ctrlclient.Client
		wantPinState reportv1alpha1.RuntimePinState
		wantReason   reportv1alpha1.UnavailableReason
	}{
		{
			name: "empty kind with missing live runtime", kind: "",
			liveClient: func(t *testing.T) ctrlclient.Client {
				return ctrlclientfake.NewClientBuilder().WithScheme(projectionRuntimeScheme(t)).Build()
			},
			wantPinState: reportv1alpha1.RuntimePinStateInvalidIntent,
			wantReason:   reportv1alpha1.UnavailableNotFound,
		},
		{
			name: "unsupported kind with missing live runtime", kind: "secret-unsupported-kind",
			liveClient: func(t *testing.T) ctrlclient.Client {
				return ctrlclientfake.NewClientBuilder().WithScheme(projectionRuntimeScheme(t)).Build()
			},
			wantPinState: reportv1alpha1.RuntimePinStateInvalidIntent,
			wantReason:   reportv1alpha1.UnavailableNotFound,
		},
		{
			name: "empty kind with disabled live runtime", kind: "",
			liveClient:   disabledProjectionRuntimeClient,
			wantPinState: reportv1alpha1.RuntimePinStateUnavailable,
			wantReason:   reportv1alpha1.UnavailableDisabled,
		},
		{
			name: "unsupported kind with disabled live runtime", kind: "secret-unsupported-kind",
			liveClient:   disabledProjectionRuntimeClient,
			wantPinState: reportv1alpha1.RuntimePinStateUnavailable,
			wantReason:   reportv1alpha1.UnavailableDisabled,
		},
		{
			name: "empty kind with unreadable live runtime", kind: "",
			liveClient:   unreadableProjectionRuntimeClient,
			wantPinState: reportv1alpha1.RuntimePinStateUnavailable,
			wantReason:   reportv1alpha1.UnavailableUnreadable,
		},
		{
			name: "unsupported kind with unreadable live runtime", kind: "secret-unsupported-kind",
			liveClient:   unreadableProjectionRuntimeClient,
			wantPinState: reportv1alpha1.RuntimePinStateUnavailable,
			wantReason:   reportv1alpha1.UnavailableUnreadable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "chat", Namespace: "workloads", UID: "isvc-uid", Generation: 7,
					ResourceVersion: "secret-isvc-resource-version",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name: "gpu-runtime", Kind: &test.kind, AutoSync: &autoSync,
					},
					Engine: &v1beta1.EngineSpec{},
				},
			}
			isvc.Status.ObservedGeneration = 7
			resolver, err := effective.NewRuntimePinResolver(
				k8sfake.NewSimpleClientset().AppsV1(),
				effective.NewRuntimeResolver(test.liveClient(t)),
				"ome-system",
				paging.Limits{PageSize: 10, MaxItems: 20, MaxPages: 2, RequestTimeout: time.Second},
			)
			require.NoError(t, err)
			state, err := resolver.Resolve(t.Context(), isvc, effective.RuntimeResolveOptions{})
			require.NoError(t, err)

			effectiveReport, err := ProjectEffective(isvc, state, projectionTestClock)
			require.NoError(t, err)
			historyReport, err := ProjectHistory(isvc, state, projectionTestClock)
			require.NoError(t, err)

			assert.Equal(t, test.wantPinState, effectiveReport.Content.Pin.State)
			require.NotNil(t, effectiveReport.Content.Selection.Runtime)
			assert.Equal(t, "gpu-runtime", effectiveReport.Content.Selection.Runtime.Name)
			assert.Equal(t, reportv1alpha1.RuntimeKindUnknown, effectiveReport.Content.Selection.Runtime.Kind)
			assert.Nil(t, effectiveReport.Content.Live.Source)
			assert.Nil(t, effectiveReport.Content.Active.Source)
			assert.Equal(t, test.wantReason, effectiveReport.Content.Live.UnavailableReason)
			assert.Contains(t, effectiveReport.Content.Issues, reportv1alpha1.RuntimeIssue{
				Code: reportv1alpha1.RuntimeIssueInvalidDeclaredKind,
			})
			assert.Equal(t, effectiveReport.Content.Selection.Runtime, historyReport.Content.Runtime)
			assert.Contains(t, historyReport.Content.Issues, reportv1alpha1.RuntimeIssue{
				Code: reportv1alpha1.RuntimeIssueInvalidDeclaredKind,
			})
			for _, source := range append(effectiveReport.Sources, historyReport.Sources...) {
				assert.NotEqual(t, "gpu-runtime", source.Name, "unobserved runtime scope must not be fabricated")
			}
			for _, format := range []report.Format{report.FormatTable, report.FormatJSON, report.FormatYAML} {
				if test.kind != "" {
					assert.NotContains(t, renderProjectionDocument(t, format, effectiveReport), test.kind)
					assert.NotContains(t, renderProjectionDocument(t, format, historyReport), test.kind)
				}
			}

			contradictory := *state
			if state.PinState == effective.RuntimePinStateInvalidIntent {
				contradictory.PinState = effective.RuntimePinStateUnavailable
			} else {
				contradictory.PinState = effective.RuntimePinStateInvalidIntent
			}
			_, err = ProjectEffective(isvc, &contradictory, projectionTestClock)
			require.ErrorIs(t, err, ErrInvalidEvidence)
			assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
			_, err = ProjectHistory(isvc, &contradictory, projectionTestClock)
			require.ErrorIs(t, err, ErrInvalidEvidence)
			assert.Equal(t, ErrInvalidEvidence.Error(), err.Error())
		})
	}
}

func projectionRuntimeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	return scheme
}

func disabledProjectionRuntimeClient(t *testing.T) ctrlclient.Client {
	t.Helper()
	runtimeObject := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-runtime"},
		Spec:       *projectionRuntimeSpec("private.registry/disabled:secret"),
	}
	runtimeObject.Spec.Disabled = pointerTo(true)
	return ctrlclientfake.NewClientBuilder().WithScheme(projectionRuntimeScheme(t)).WithObjects(runtimeObject).Build()
}

func unreadableProjectionRuntimeClient(t *testing.T) ctrlclient.Client {
	t.Helper()
	return projectionGetErrorClient{
		Client: ctrlclientfake.NewClientBuilder().WithScheme(projectionRuntimeScheme(t)).Build(),
		err:    errors.New("secret-live-read-failure"),
	}
}

type projectionGetErrorClient struct {
	ctrlclient.Client
	err error
}

func (client projectionGetErrorClient) Get(
	context.Context,
	ctrlclient.ObjectKey,
	ctrlclient.Object,
	...ctrlclient.GetOption,
) error {
	return client.err
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
			ResourceVersion: "secret-isvc-resource-version",
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
