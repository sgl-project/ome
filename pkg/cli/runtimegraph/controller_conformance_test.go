package runtimegraph

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

func TestProjectPathMatchesRuntimeInheritanceResolverLookupTrace(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("bridge", "base"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "base", ""),
			namespacedRuntime("team-a", "leaf", "bridge"),
		},
	}
	recorder := newIdentityRecordingClient(t, snapshot)

	_, chain, err := runtimeinheritance.ResolveNamespacedRuntime(
		context.Background(), recorder, "team-a", "leaf",
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"base", "bridge", "leaf"}, chain)
	graph, err := Build(snapshot)
	require.NoError(t, err)
	projection, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf",
	})
	require.NoError(t, err)
	assert.Equal(t, reversedIdentities(recorder.successfulGets), runtimePathIdentities(
		onlyPath(t, projection),
	))
}

func TestProjectCycleMatchesResolverNameCheckBeforeLookup(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("bridge", "same-name"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "same-name", "bridge"),
		},
	}
	recorder := newIdentityRecordingClient(t, snapshot)

	_, _, err := runtimeinheritance.ResolveNamespacedRuntime(
		context.Background(), recorder, "team-a", "same-name",
	)

	var cycle *runtimeinheritance.CycleError
	require.True(t, errors.As(err, &cycle))
	assert.Equal(t, []string{"same-name", "bridge", "same-name"}, cycle.Cycle)
	assert.Equal(t, []Identity{
		{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
		{Kind: KindClusterServingRuntime, Name: "bridge"},
	}, recorder.successfulGets, "the resolver detects the repeated name before fetching it again")

	graph, err := Build(snapshot)
	require.NoError(t, err)
	projection, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
	})
	require.NoError(t, err)
	path := onlyPath(t, projection)
	require.NotNil(t, path.Issue)
	assert.Equal(t, IssueCycleDetected, path.Issue.Code)
	assert.Equal(t, []Identity{
		{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
		{Kind: KindClusterServingRuntime, Name: "bridge"},
		{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
	}, path.Issue.Path)
}

type identityRecordingClient struct {
	client.Client
	successfulGets []Identity
}

func (c *identityRecordingClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	options ...client.GetOption,
) error {
	if err := c.Client.Get(ctx, key, object, options...); err != nil {
		return err
	}
	switch value := object.(type) {
	case *omev1beta1.ClusterServingRuntime:
		c.successfulGets = append(c.successfulGets, Identity{
			Kind: KindClusterServingRuntime, Name: value.Name,
		})
	case *omev1beta1.ServingRuntime:
		c.successfulGets = append(c.successfulGets, Identity{
			Kind: KindServingRuntime, Namespace: value.Namespace, Name: value.Name,
		})
	}
	return nil
}

func newIdentityRecordingClient(t *testing.T, snapshot Snapshot) *identityRecordingClient {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, omev1beta1.AddToScheme(scheme))
	objects := make([]client.Object, 0,
		len(snapshot.ClusterServingRuntimes)+len(snapshot.ServingRuntimes))
	for i := range snapshot.ClusterServingRuntimes {
		objects = append(objects, snapshot.ClusterServingRuntimes[i].DeepCopy())
	}
	for i := range snapshot.ServingRuntimes {
		objects = append(objects, snapshot.ServingRuntimes[i].DeepCopy())
	}
	return &identityRecordingClient{Client: ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()}
}

func reversedIdentities(values []Identity) []Identity {
	result := make([]Identity, len(values))
	for i := range values {
		result[len(values)-1-i] = values[i]
	}
	return result
}

func runtimePathIdentities(path ResolutionPath) []Identity {
	result := make([]Identity, len(path.Runtimes))
	for i := range path.Runtimes {
		result[i] = path.Runtimes[i].Identity
	}
	return result
}
