package effective

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimerevision"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

func TestInspectRuntimeRevisionCanonicalizesCreationTimestampWithoutMutation(t *testing.T) {
	location := time.FixedZone("same-instant", 2*60*60)
	tests := []struct {
		name      string
		timestamp time.Time
		want      metav1.Time
	}{
		{
			name:      "nonzero timestamp uses UTC",
			timestamp: time.Unix(100, 123).In(location),
			want:      metav1.NewTime(time.Unix(100, 123).UTC()),
		},
		{
			name:      "zero timestamp uses canonical zero",
			timestamp: time.Time{}.In(location),
			want:      metav1.Time{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revision := revisionFixture(
				t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("v1"),
			)
			revision.CreationTimestamp = metav1.NewTime(test.timestamp)
			before := revision.DeepCopy()

			observation := inspectRuntimeRevision(revision, "ome", revision.Name, "runtime", runtimeselector.KindClusterServingRuntime, "")

			assert.Equal(t, test.want, observation.CreationTimestamp)
			assert.Equal(t, before, revision, "inspection must not mutate the caller object")
		})
	}
}

func TestInspectRuntimeRevisionChecksWriterContractWithoutChangingReadability(t *testing.T) {
	base := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("v1"))
	expectedName := base.Name

	valid := inspectRuntimeRevision(base.DeepCopy(), "ome", expectedName, "runtime", runtimeselector.KindClusterServingRuntime, "")
	assert.True(t, valid.usable)
	assert.Equal(t, RevisionConsistencyConsistent, valid.Consistency)
	assert.Empty(t, valid.ConsistencyCodes())

	tests := []struct {
		name   string
		mutate func(*appsv1.ControllerRevision)
		code   RevisionConsistencyCode
		usable bool
	}{
		{
			name: "returned name", code: RevisionConsistencyReturnedIdentity, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Name = "returned-other" },
		},
		{
			name: "returned namespace", code: RevisionConsistencyReturnedIdentity, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Namespace = "other" },
		},
		{
			name: "created by", code: RevisionConsistencyCreatedBy, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) {
				rev.Annotations[constants.RuntimeRevisionCreatedByKey] = "other"
			},
		},
		{
			name: "source name", code: RevisionConsistencySourceName, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Labels[constants.RuntimeRevisionOfLabelKey] = "other" },
		},
		{
			name: "source kind", code: RevisionConsistencySourceKind, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) {
				rev.Labels[constants.RuntimeRevisionOfKindLabelKey] = runtimeselector.KindServingRuntime
			},
		},
		{
			name: "source namespace", code: RevisionConsistencySourceNamespace, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) {
				rev.Labels[constants.RuntimeRevisionOfNamespaceLabelKey] = "other"
			},
		},
		{
			name: "hash label invalid", code: RevisionConsistencyHashLabelInvalid, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Labels[constants.RuntimeRevisionHashLabelKey] = "ZZZZZZZZ" },
		},
		{
			name: "hash label mismatch", code: RevisionConsistencyHashLabelMismatch, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Labels[constants.RuntimeRevisionHashLabelKey] = "deadbeef" },
		},
		{
			name: "name hash", code: RevisionConsistencyNameHash, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Name = "cr-runtime-deadbeef" },
		},
		{
			name: "ordinal", code: RevisionConsistencyOrdinal, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Revision = 2 },
		},
		{
			name: "unexpected data object", code: RevisionConsistencyUnexpectedDataObject, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) {
				rev.Data.Object = &appsv1.ControllerRevision{}
			},
		},
		{
			name: "noncanonical bytes", code: RevisionConsistencyPayloadCanonicality, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Data.Raw = append(rev.Data.Raw, '\n') },
		},
		{
			name: "unknown field", code: RevisionConsistencyPayloadCanonicality, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Data.Raw = []byte(`{"unknown":1}`) },
		},
		{
			name: "duplicate field", code: RevisionConsistencyPayloadCanonicality, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) {
				rev.Data.Raw = []byte(`{"disabled":false,"disabled":false}`)
			},
		},
		{
			name: "null", code: RevisionConsistencyPayloadCanonicality, usable: true,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Data.Raw = []byte(`null`) },
		},
		{
			name: "malformed", code: RevisionConsistencyMalformedPayload, usable: false,
			mutate: func(rev *appsv1.ControllerRevision) { rev.Data.Raw = []byte(`{`) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revision := base.DeepCopy()
			test.mutate(revision)
			got := inspectRuntimeRevision(revision, "ome", expectedName, "runtime", runtimeselector.KindClusterServingRuntime, "")
			assert.Equal(t, test.usable, got.usable)
			assert.Equal(t, RevisionConsistencyInconsistent, got.Consistency)
			assert.Contains(t, got.ConsistencyCodes(), test.code)
		})
	}
}

func TestInspectRuntimeRevisionValidatesObservedWriterShapeWithoutDeclaredScope(t *testing.T) {
	base := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("v1"))
	tests := []struct {
		name   string
		mutate func(*appsv1.ControllerRevision)
		code   RevisionConsistencyCode
	}{
		{name: "empty source name", code: RevisionConsistencySourceName, mutate: func(rev *appsv1.ControllerRevision) {
			rev.Labels[constants.RuntimeRevisionOfLabelKey] = ""
		}},
		{name: "unknown source kind", code: RevisionConsistencySourceKind, mutate: func(rev *appsv1.ControllerRevision) {
			rev.Labels[constants.RuntimeRevisionOfKindLabelKey] = "Bogus"
		}},
		{name: "cluster source with namespace", code: RevisionConsistencySourceNamespace, mutate: func(rev *appsv1.ControllerRevision) {
			rev.Labels[constants.RuntimeRevisionOfNamespaceLabelKey] = "unexpected"
		}},
		{name: "namespaced source without namespace", code: RevisionConsistencySourceNamespace, mutate: func(rev *appsv1.ControllerRevision) {
			rev.Labels[constants.RuntimeRevisionOfKindLabelKey] = runtimeselector.KindServingRuntime
			rev.Labels[constants.RuntimeRevisionOfNamespaceLabelKey] = ""
		}},
		{name: "observed labels imply another name", code: RevisionConsistencyNameHash, mutate: func(rev *appsv1.ControllerRevision) {
			rev.Labels[constants.RuntimeRevisionOfLabelKey] = "other-runtime"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revision := base.DeepCopy()
			test.mutate(revision)
			got := inspectRuntimeRevision(revision, "ome", revision.Name, "", "", "")
			assert.Contains(t, got.ConsistencyCodes(), test.code)
		})
	}
}

func TestRevisionShortHashIsVerifiedAndRelationsUseRecomputedHash(t *testing.T) {
	live := livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false)
	live.Runtime.spec = runtimeSpecFixture("live")
	liveFull, liveShort, err := runtimerevision.Hash(live.Runtime.spec)
	require.NoError(t, err)
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("different"))
	revision.Labels[constants.RuntimeRevisionHashLabelKey] = liveShort

	observation := inspectRuntimeRevision(
		revision, "ome", revision.Name, "runtime", runtimeselector.KindClusterServingRuntime, "",
	)
	setObservationLiveRelation(&observation, liveFull, liveShort)

	assert.Empty(t, observation.ShortHash, "mismatched raw label must not become public provenance")
	assert.Contains(t, observation.ConsistencyCodes(), RevisionConsistencyHashLabelMismatch)
	assert.Equal(t, RuntimeHashRelationDifferent, observation.RelationToLive)

	valid := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("valid"))
	verified := inspectRuntimeRevision(valid, "ome", valid.Name, "runtime", runtimeselector.KindClusterServingRuntime, "")
	assert.Equal(t, valid.Labels[constants.RuntimeRevisionHashLabelKey], verified.ShortHash)
}

func TestResolveManagedPinKeepsInconsistentReadableRevisionActiveButBlocksConsistencyGate(t *testing.T) {
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("v1"))
	revision.Annotations[constants.RuntimeRevisionCreatedByKey] = "unexpected-writer"
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return revision.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, "")
	isvc.Status.PinnedRevisionName = revision.Name

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)
	active, err := state.RequireActive()
	require.NoError(t, err)
	assert.Equal(t, RevisionConsistencyInconsistent, active.Consistency)
	_, err = state.RequireConsistentActive()
	assert.EqualError(t, err, "active runtime configuration is not consistency-safe")
}

func TestResolveExplicitPinRejectsWrongRuntimeOfWithoutFallingBack(t *testing.T) {
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "other-runtime", runtimeSpecFixture("other"))
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return revision.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, revision.Name)

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, RuntimePinStateRevisionInvalid, state.PinState)
	_, err = state.RequireActive()
	assert.EqualError(t, err, "active runtime configuration is unavailable")
}

func TestRevisionEvidenceExposesSafeAssociationDisabledStateAndIssueKey(t *testing.T) {
	autoSync := false
	disabled := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", &v1beta1.ServingRuntimeSpec{Disabled: ptrBool(true)})
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return disabled.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, disabled.Name)
	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)
	observation := state.RevisionObservations()[0]
	assert.Equal(t, disabled.Name, observation.ExpectedName())
	assert.Equal(t, disabled.Name, observation.ReturnedName())
	assert.True(t, observation.Disabled)

	missingName := "missing-revision"
	missing := apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "controllerrevisions"}, missingName)
	missingResolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return nil, missing
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	state, err = missingResolver.Resolve(context.Background(), pinISVC("runtime", &autoSync, missingName), RuntimeResolveOptions{})
	require.NoError(t, err)
	issues := state.SourceIssues()
	require.Len(t, issues, 1)
	assert.Equal(t, missingName, issues[0].RevisionName)
}

func TestResolveNilExactRevisionResponseIsUnavailable(t *testing.T) {
	autoSync := false
	const revisionName = "empty-revision-response"
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return nil, nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, "")
	isvc.Status.PinnedRevisionName = revisionName

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})

	require.NoError(t, err)
	assert.Equal(t, RuntimePinStateUnavailable, state.PinState)
	_, err = state.RequireActive()
	assert.ErrorIs(t, err, ErrActiveRuntimeUnavailable)
	observations := state.RevisionObservations()
	require.Len(t, observations, 1)
	assert.False(t, observations[0].ObjectReturned())
	assert.Equal(t, revisionName, observations[0].ExpectedName())
	assert.Equal(t, "ome", observations[0].ExpectedNamespace())
	issues := state.SourceIssues()
	require.Len(t, issues, 1)
	assert.Equal(t, RuntimeSourceIssueRevisionGetFailed, issues[0].Code)
	assert.Equal(t, revisionName, issues[0].RevisionName)
	assert.ErrorContains(t, errors.Unwrap(issues[0]), "empty response")
}

func TestMalformedRevisionStillExposesReturnedIdentity(t *testing.T) {
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("bad"))
	revision.Data.Raw = []byte(`{`)

	observation := inspectRuntimeRevision(
		revision, "ome", revision.Name, "runtime", runtimeselector.KindClusterServingRuntime, "",
	)

	assert.False(t, observation.Available)
	assert.True(t, observation.ObjectReturned())
	assert.Equal(t, revision.Name, observation.ExpectedName())
	assert.Equal(t, revision.Name, observation.ReturnedName())
	assert.Equal(t, "ome", observation.ExpectedNamespace())
	assert.Equal(t, "ome", observation.ReturnedNamespace())
}

func FuzzInspectRuntimeRevisionPayload(f *testing.F) {
	canonical, err := json.Marshal(runtimeSpecFixture("canonical"))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		canonical,
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(`{"unknown":1}`),
		[]byte(`{"disabled":false,"disabled":true}`),
		[]byte(`{`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		revision := &appsv1.ControllerRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: "revision", Namespace: "ome",
				Labels: map[string]string{
					constants.RuntimeRevisionOfLabelKey:          "runtime",
					constants.RuntimeRevisionOfKindLabelKey:      runtimeselector.KindClusterServingRuntime,
					constants.RuntimeRevisionOfNamespaceLabelKey: "",
					constants.RuntimeRevisionHashLabelKey:        "deadbeef",
				},
				Annotations: map[string]string{
					constants.RuntimeRevisionCreatedByKey: constants.RuntimeRevisionCreatedByOMEValue,
				},
			},
			Data: runtimeRaw(string(raw)), Revision: 1,
		}
		observation := inspectRuntimeRevision(
			revision, "ome", "revision", "runtime", runtimeselector.KindClusterServingRuntime, "",
		)
		assert.Equal(t, "revision", observation.ReturnedName())
		assert.Equal(t, "ome", observation.ReturnedNamespace())
		_, marshalErr := json.Marshal(observation)
		assert.True(t, errors.Is(marshalErr, ErrUnsafeRuntimeSerialization))
		assert.Equal(t, "<effective.RuntimeRevisionObservation redacted>", observation.String())
	})
}
