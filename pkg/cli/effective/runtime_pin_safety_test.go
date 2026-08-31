package effective

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigsyaml "sigs.k8s.io/yaml"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

type runtimeSourceCauseCanary struct {
	Text string
}

func (e runtimeSourceCauseCanary) Error() string { return e.Text }

func TestRuntimeStateAccessorsReturnDefensiveCopies(t *testing.T) {
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("pin"))
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				return revision.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			live := livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false)
			live.Runtime.spec = runtimeSpecFixture("live")
			return live, nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, revision.Name)

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)

	active, err := state.RequireActive()
	require.NoError(t, err)
	components := active.Components()
	require.NotEmpty(t, components)
	components[0].engine.Runner.Image = "mutated"
	again, err := state.RequireActive()
	require.NoError(t, err)
	assert.Equal(t, "pin", again.Components()[0].engine.Runner.Image)

	observations := state.RevisionObservations()
	require.Len(t, observations, 1)
	roles := observations[0].Roles()
	roles[0] = RuntimeRevisionRoleHistory
	observations[0].spec.EngineConfig.Runner.Image = "mutated"
	againObservations := state.RevisionObservations()
	assert.Equal(t, RuntimeRevisionRoleRequested, againObservations[0].Roles()[0])
	assert.Equal(t, "pin", againObservations[0].spec.EngineConfig.Runner.Image)

	live := state.LiveConfiguration()
	require.NotNil(t, live)
	live.Runtime.spec.EngineConfig.Runner.Image = "mutated"
	assert.Equal(t, "live", state.LiveConfiguration().Runtime.spec.EngineConfig.Runner.Image)
}

func TestRuntimeStateBindsCollectedInferenceServiceIdentity(t *testing.T) {
	autoSync := true
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace { return revisionNamespaceStub{} },
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, "")
	isvc.UID = "inference-service-uid"
	isvc.ResourceVersion = "17"

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)

	want := InferenceServiceIdentity{
		Name: "chat", Namespace: "workloads", UID: "inference-service-uid",
	}
	assert.Equal(t, want, state.InferenceServiceIdentity())
	assert.Equal(t, InferenceServiceIdentity{}, (*RuntimeState)(nil).InferenceServiceIdentity())
	assert.True(t, state.MatchesInferenceService(isvc.DeepCopy()))
	assert.False(t, (&RuntimeState{}).MatchesInferenceService(&v1beta1.InferenceService{}))
	assert.False(t, state.MatchesInferenceService(nil))
	assert.False(t, (*RuntimeState)(nil).MatchesInferenceService(isvc))

	mutations := []func(*v1beta1.InferenceService){
		func(candidate *v1beta1.InferenceService) { candidate.Name = "other" },
		func(candidate *v1beta1.InferenceService) { candidate.Namespace = "other" },
		func(candidate *v1beta1.InferenceService) { candidate.UID = "other" },
		func(candidate *v1beta1.InferenceService) { candidate.ResourceVersion = "18" },
	}
	for _, mutate := range mutations {
		candidate := isvc.DeepCopy()
		mutate(candidate)
		assert.False(t, state.MatchesInferenceService(candidate))
	}
}

func TestUnsafeRuntimePinTypesRejectJSONAndYAMLSerialization(t *testing.T) {
	values := []any{
		RuntimeState{},
		ActiveConfiguration{spec: runtimeSpecFixture("secret")},
		RuntimeRevisionObservation{spec: runtimeSpecFixture("secret")},
	}
	for _, value := range values {
		_, err := json.Marshal(value)
		assert.ErrorIs(t, err, ErrUnsafeRuntimeSerialization)
		_, err = sigsyaml.Marshal(value)
		assert.True(t, errors.Is(err, ErrUnsafeRuntimeSerialization), "%T: %v", value, err)
	}
}

func TestRequireActiveUsesStableSentinelErrors(t *testing.T) {
	_, err := (*RuntimeState)(nil).RequireActive()
	assert.ErrorIs(t, err, ErrActiveRuntimeUnavailable)

	state := &RuntimeState{active: &ActiveConfiguration{Consistency: RevisionConsistencyUnknown}}
	_, err = state.RequireConsistentActive()
	assert.ErrorIs(t, err, ErrActiveRuntimeInconsistent)
}

func TestRuntimePinTypesHaveFixedRedactedStringFormatting(t *testing.T) {
	const sentinel = "redaction-canary-do-not-emit"
	values := []struct {
		value any
		want  string
	}{
		{RuntimeState{RuntimeName: sentinel}, "<effective.RuntimeState redacted>"},
		{ActiveConfiguration{RuntimeName: sentinel, spec: runtimeSpecFixture(sentinel)}, "<effective.ActiveConfiguration redacted>"},
		{RuntimeRevisionObservation{SourceName: sentinel, spec: runtimeSpecFixture(sentinel)}, "<effective.RuntimeRevisionObservation redacted>"},
	}
	for _, test := range values {
		got := fmt.Sprintf("%v|%+v|%#v", test.value, test.value, test.value)
		assert.NotContains(t, got, sentinel)
		assert.Equal(t, strings.Join([]string{test.want, test.want, test.want}, "|"), got)
	}
}

func TestRuntimeSourceIssueFormattingNeverExposesCause(t *testing.T) {
	const sentinel = "runtime-source-cause-redaction-canary"
	cause := runtimeSourceCauseCanary{Text: sentinel}
	issue := RuntimeSourceIssue{
		Code: RuntimeSourceIssueRevisionGetFailed, RevisionName: "safe-revision", cause: cause,
	}

	formatted := fmt.Sprintf("%v|%+v|%#v", issue, issue, issue)

	assert.NotContains(t, formatted, sentinel)
	assert.Equal(t,
		"runtime revision evidence read failed|runtime revision evidence read failed|<effective.RuntimeSourceIssue redacted>",
		formatted,
	)
	assert.Equal(t, "runtime revision evidence read failed", issue.Error())
	assert.ErrorIs(t, issue, cause)
}
