package runtimegraph

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestProjectRejectsAmbiguousImplicitTarget(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{clusterRuntime("shared", "")},
		ServingRuntimes:        []omev1beta1.ServingRuntime{namespacedRuntime("team-a", "shared", "")},
	}
	graph, err := Build(snapshot)
	require.NoError(t, err)

	_, err = graph.Project(Target{Namespace: "team-a", Name: "shared"})

	assert.ErrorIs(t, err, ErrTargetAmbiguous)
	var ambiguous *AmbiguousTargetError
	require.True(t, errors.As(err, &ambiguous))
	assert.Equal(t, []Identity{
		{Kind: KindClusterServingRuntime, Name: "shared"},
		{Kind: KindServingRuntime, Namespace: "team-a", Name: "shared"},
	}, ambiguous.Candidates)
}

func TestProjectResolvesNamespacedParentsBeforeClusterParents(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{clusterRuntime("shared", "")},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "shared", ""),
			namespacedRuntime("team-a", "child", "shared"),
			namespacedRuntime("team-b", "child", "shared"),
		},
	}
	graph, err := Build(snapshot)
	require.NoError(t, err)

	local, err := graph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"})
	require.NoError(t, err)
	clusterFallback, err := graph.Project(Target{Kind: KindServingRuntime, Namespace: "team-b", Name: "child"})
	require.NoError(t, err)

	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "shared"}},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"},
			ParentName: "shared",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "shared",
			}),
		},
	}, onlyPath(t, local).Runtimes)
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "shared"}},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-b", Name: "child"},
			ParentName: "shared",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "shared",
			}),
		},
	}, onlyPath(t, clusterFallback).Runtimes)
}

func TestProjectAcceptsFiveRuntimeChainAndRejectsSixthLevel(t *testing.T) {
	snapshot := Snapshot{ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
		clusterRuntime("level-1", ""),
		clusterRuntime("level-2", "level-1"),
		clusterRuntime("level-3", "level-2"),
		clusterRuntime("level-4", "level-3"),
		clusterRuntime("level-5", "level-4"),
		clusterRuntime("level-6", "level-5"),
	}}
	graph, err := Build(snapshot)
	require.NoError(t, err)

	five, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "level-5"})
	require.NoError(t, err)
	six, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "level-6"})
	require.NoError(t, err)

	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-1"}},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-2"}, ParentName: "level-1", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-1"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-3"}, ParentName: "level-2", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-2"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-4"}, ParentName: "level-3", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-3"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-5"}, ParentName: "level-4", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-4"})},
	}, pathForSubject(t, five, Identity{Kind: KindClusterServingRuntime, Name: "level-5"}).Runtimes)
	assert.Nil(t, pathForSubject(t, five, Identity{Kind: KindClusterServingRuntime, Name: "level-5"}).Issue)
	sixPath := onlyPath(t, six)
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-2"}, ParentName: "level-1"},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-3"}, ParentName: "level-2", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-2"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-4"}, ParentName: "level-3", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-3"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-5"}, ParentName: "level-4", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-4"})},
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-6"}, ParentName: "level-5", ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "level-5"})},
	}, sixPath.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueMaxDepthExceeded, Subject: Identity{Kind: KindClusterServingRuntime, Name: "level-6"},
		ParentName: "level-1",
		Path: []Identity{
			{Kind: KindClusterServingRuntime, Name: "level-6"},
			{Kind: KindClusterServingRuntime, Name: "level-5"},
			{Kind: KindClusterServingRuntime, Name: "level-4"},
			{Kind: KindClusterServingRuntime, Name: "level-3"},
			{Kind: KindClusterServingRuntime, Name: "level-2"},
		},
	}, sixPath.Issue)
}

func TestProjectReportsMissingParentWithoutCrossNamespaceLookup(t *testing.T) {
	snapshot := Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{
		namespacedRuntime("team-a", "child", "shared"),
		namespacedRuntime("team-b", "shared", ""),
	}}
	graph, err := Build(snapshot)
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"})
	require.NoError(t, err)

	path := onlyPath(t, projection)
	assert.Equal(t, []Runtime{{
		Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"},
		ParentName: "shared",
	}}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueParentMissing, Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"},
		ParentName: "shared",
		Path:       []Identity{{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"}},
	}, path.Issue)
}

func TestProjectReportsSameScopeCycleByFullIdentity(t *testing.T) {
	graph, err := Build(Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{
		namespacedRuntime("team-a", "runtime-a", "runtime-b"),
		namespacedRuntime("team-a", "runtime-b", "runtime-a"),
	}})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a"})
	require.NoError(t, err)

	path := pathForSubject(t, projection, Identity{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a",
	})
	assert.Equal(t, []Runtime{
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-b"},
			ParentName: "runtime-a",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a",
			}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a"},
			ParentName: "runtime-b",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-b",
			}),
		},
	}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueCycleDetected, Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a"},
		ParentName: "runtime-a",
		Path: []Identity{
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a"},
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-b"},
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime-a"},
		},
	}, path.Issue)
}

func TestProjectReportsCycleAfterCrossingFromNamespaceToCluster(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("cluster-a", "cluster-b"),
			clusterRuntime("cluster-b", "cluster-a"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "entry", "cluster-a"),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "entry"})
	require.NoError(t, err)

	path := onlyPath(t, projection)
	assert.Equal(t, []Runtime{
		{
			Identity:       Identity{Kind: KindClusterServingRuntime, Name: "cluster-b"},
			ParentName:     "cluster-a",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "cluster-a"}),
		},
		{
			Identity:       Identity{Kind: KindClusterServingRuntime, Name: "cluster-a"},
			ParentName:     "cluster-b",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "cluster-b"}),
		},
		{
			Identity:       Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "entry"},
			ParentName:     "cluster-a",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "cluster-a"}),
		},
	}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueCycleDetected, Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "entry"},
		ParentName: "cluster-a",
		Path: []Identity{
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "entry"},
			{Kind: KindClusterServingRuntime, Name: "cluster-a"},
			{Kind: KindClusterServingRuntime, Name: "cluster-b"},
			{Kind: KindClusterServingRuntime, Name: "cluster-a"},
		},
	}, path.Issue)
}

func TestProjectUsesControllerNameCyclesForRepeatedNameAcrossScopes(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("same-name", ""),
			clusterRuntime("bridge", "same-name"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "same-name", "bridge"),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
	})
	require.NoError(t, err)

	path := pathForSubject(t, projection, Identity{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
	})
	assert.Equal(t, []Runtime{
		{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "same-name",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
			}),
		},
		{
			Identity: Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
			},
			ParentName:     "bridge",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "bridge"}),
		},
	}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueCycleDetected,
		Subject: Identity{
			Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
		},
		ParentName: "same-name",
		Path: []Identity{
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
			{Kind: KindClusterServingRuntime, Name: "bridge"},
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
		},
	}, path.Issue)
}

func TestProjectBuildsDeterministicDependentPaths(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("root", ""),
			clusterRuntime("z-branch", "root"),
			clusterRuntime("a-branch", "root"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-b", "leaf", "root"),
			namespacedRuntime("team-a", "leaf", "a-branch"),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "root"})
	require.NoError(t, err)

	assert.Equal(t, []ResolutionContext{
		clusterContext(), namespacedContext("team-a"), namespacedContext("team-b"),
	}, projectionContexts(projection))
	assert.Equal(t, []Identity{
		{Kind: KindClusterServingRuntime, Name: "root"},
		{Kind: KindClusterServingRuntime, Name: "a-branch"},
		{Kind: KindClusterServingRuntime, Name: "z-branch"},
	}, contextPathSubjects(projection.Contexts[0]))
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "root"}},
		{
			Identity:       Identity{Kind: KindClusterServingRuntime, Name: "a-branch"},
			ParentName:     "root",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "root"}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
			ParentName: "a-branch",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "a-branch",
			}),
		},
	}, projection.Contexts[1].Paths[0].Runtimes)
}

func TestClusterRuntimeParentNeverResolvesToNamespacedRuntime(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{clusterRuntime("child", "parent")},
		ServingRuntimes:        []omev1beta1.ServingRuntime{namespacedRuntime("team-a", "parent", "")},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "child"})
	require.NoError(t, err)

	path := onlyPath(t, projection)
	assert.Equal(t, clusterContext(), projection.Contexts[0].Context)
	assert.Equal(t, []Runtime{{
		Identity:   Identity{Kind: KindClusterServingRuntime, Name: "child"},
		ParentName: "parent",
	}}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code: IssueParentMissing, Subject: Identity{Kind: KindClusterServingRuntime, Name: "child"},
		ParentName: "parent",
		Path:       []Identity{{Kind: KindClusterServingRuntime, Name: "child"}},
	}, path.Issue)
}

func TestBuildRejectsDuplicateRuntimeIdentity(t *testing.T) {
	duplicate := namespacedRuntime("team-a", "runtime", "")

	_, err := Build(Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{duplicate, duplicate}})

	assert.ErrorIs(t, err, ErrDuplicateRuntime)
	var duplicateError *DuplicateRuntimeError
	require.True(t, errors.As(err, &duplicateError))
	assert.Equal(t, Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "runtime"}, duplicateError.Identity)
}

func TestBuildRejectsIncompleteRuntimeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		snapshot Snapshot
		want     Identity
	}{
		{
			name: "cluster name required",
			snapshot: Snapshot{ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
				clusterRuntime("", ""),
			}},
			want: Identity{Kind: KindClusterServingRuntime},
		},
		{
			name: "namespaced name required",
			snapshot: Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{
				namespacedRuntime("team-a", "", ""),
			}},
			want: Identity{Kind: KindServingRuntime, Namespace: "team-a"},
		},
		{
			name: "namespace required",
			snapshot: Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{
				namespacedRuntime("", "runtime", ""),
			}},
			want: Identity{Kind: KindServingRuntime, Name: "runtime"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Build(test.snapshot)

			assert.ErrorIs(t, err, ErrInvalidRuntime)
			var invalid *InvalidRuntimeError
			require.True(t, errors.As(err, &invalid))
			assert.Equal(t, test.want, invalid.Identity)
		})
	}
}

func TestProjectReturnsTypedTargetErrors(t *testing.T) {
	graph, err := Build(Snapshot{})
	require.NoError(t, err)

	tests := []struct {
		name   string
		target Target
		want   error
	}{
		{name: "name required", target: Target{}, want: ErrInvalidTarget},
		{name: "namespaced namespace required", target: Target{Kind: KindServingRuntime, Name: "runtime"}, want: ErrInvalidTarget},
		{name: "cluster namespace forbidden", target: Target{Kind: KindClusterServingRuntime, Namespace: "team-a", Name: "runtime"}, want: ErrInvalidTarget},
		{name: "kind rejected", target: Target{Kind: Kind("Other"), Name: "runtime"}, want: ErrInvalidTarget},
		{name: "explicit target absent", target: Target{Kind: KindClusterServingRuntime, Name: "runtime"}, want: ErrTargetNotFound},
		{name: "implicit target absent", target: Target{Namespace: "team-a", Name: "runtime"}, want: ErrTargetNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := graph.Project(test.target)
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestBuildIgnoresReportedInheritanceStatus(t *testing.T) {
	stale := namespacedRuntime("team-a", "child", "parent")
	stale.Status.InheritanceChain = []string{"wrong-root", "child"}
	stale.Status.Conditions = []metav1.Condition{{Type: "InheritanceReady", Status: metav1.ConditionFalse}}
	current := stale.DeepCopy()
	current.Status = omev1beta1.ServingRuntimeStatus{}
	parent := namespacedRuntime("team-a", "parent", "")

	staleGraph, err := Build(Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{stale, parent}})
	require.NoError(t, err)
	currentGraph, err := Build(Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{*current, parent}})
	require.NoError(t, err)
	staleProjection, err := staleGraph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"})
	require.NoError(t, err)
	currentProjection, err := currentGraph.Project(Target{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"})
	require.NoError(t, err)

	assert.Equal(t, currentProjection, staleProjection)
}

func TestBuildAndProjectAreDeterministicAndDoNotMutateInput(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("root", ""),
			clusterRuntime("branch-b", "root"),
			clusterRuntime("branch-a", "root"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-b", "leaf", "branch-b"),
			namespacedRuntime("team-a", "leaf", "branch-a"),
		},
	}
	original := copySnapshot(snapshot)
	shuffled := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			*snapshot.ClusterServingRuntimes[2].DeepCopy(),
			*snapshot.ClusterServingRuntimes[0].DeepCopy(),
			*snapshot.ClusterServingRuntimes[1].DeepCopy(),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			*snapshot.ServingRuntimes[1].DeepCopy(),
			*snapshot.ServingRuntimes[0].DeepCopy(),
		},
	}

	graph, err := Build(snapshot)
	require.NoError(t, err)
	shuffledGraph, err := Build(shuffled)
	require.NoError(t, err)
	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "root"})
	require.NoError(t, err)
	shuffledProjection, err := shuffledGraph.Project(Target{Kind: KindClusterServingRuntime, Name: "root"})
	require.NoError(t, err)
	encoded, err := json.Marshal(projection)
	require.NoError(t, err)
	shuffledEncoded, err := json.Marshal(shuffledProjection)
	require.NoError(t, err)

	assert.Equal(t, graph, shuffledGraph)
	assert.Equal(t, projection, shuffledProjection)
	assert.Equal(t, string(encoded), string(shuffledEncoded))
	assert.Equal(t, original, snapshot)
}

func copySnapshot(snapshot Snapshot) Snapshot {
	result := Snapshot{
		ClusterServingRuntimes: make([]omev1beta1.ClusterServingRuntime, len(snapshot.ClusterServingRuntimes)),
		ServingRuntimes:        make([]omev1beta1.ServingRuntime, len(snapshot.ServingRuntimes)),
	}
	for i := range snapshot.ClusterServingRuntimes {
		result.ClusterServingRuntimes[i] = *snapshot.ClusterServingRuntimes[i].DeepCopy()
	}
	for i := range snapshot.ServingRuntimes {
		result.ServingRuntimes[i] = *snapshot.ServingRuntimes[i].DeepCopy()
	}
	return result
}

func identityPointer(identity Identity) *Identity { return &identity }

func clusterRuntime(name, parent string) omev1beta1.ClusterServingRuntime {
	annotations := map[string]string{}
	if parent != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = parent
	}
	return omev1beta1.ClusterServingRuntime{ObjectMeta: metav1.ObjectMeta{
		Name: name, Annotations: annotations,
	}}
}

func namespacedRuntime(namespace, name, parent string) omev1beta1.ServingRuntime {
	annotations := map[string]string{}
	if parent != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = parent
	}
	return omev1beta1.ServingRuntime{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace, Name: name, Annotations: annotations,
	}}
}
