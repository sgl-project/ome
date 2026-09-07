package runtimegraph

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestProjectKeepsNamespaceFirstResolutionAfterClusterFallback(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("bridge", "base"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "base", ""),
			namespacedRuntime("team-a", "leaf", "bridge"),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf",
	})

	require.NoError(t, err)
	assert.Equal(t, Projection{
		Target: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
		Contexts: []ContextProjection{{
			Context: namespacedContext("team-a"),
			Paths: []ResolutionPath{{
				Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
				Runtimes: []Runtime{
					{Identity: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "base"}},
					{
						Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
						ParentName: "base",
						ResolvedParent: identityPointer(Identity{
							Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
						}),
					},
					{
						Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
						ParentName: "bridge",
						ResolvedParent: identityPointer(Identity{
							Kind: KindClusterServingRuntime, Name: "bridge",
						}),
					},
				},
			}},
		}},
	}, projection)
}

func TestProjectKeepsDirectClusterResolutionSeparateFromNamespacedUses(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("bridge", "base"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "base", ""),
			namespacedRuntime("team-a", "leaf", "bridge"),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "bridge"})

	require.NoError(t, err)
	require.Len(t, projection.Contexts, 2)
	assert.Equal(t, []ResolutionContext{
		clusterContext(), namespacedContext("team-a"),
	}, projectionContexts(projection))
	assert.Equal(t, ResolutionPath{
		Subject: Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
		Runtimes: []Runtime{{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "base",
		}},
		Issue: &Issue{
			Code:       IssueParentMissing,
			Subject:    Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "base",
			Path:       []Identity{{Kind: KindClusterServingRuntime, Name: "bridge"}},
		},
	}, projection.Contexts[0].Paths[0])
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "base"}},
		{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "base",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
			}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
			ParentName: "bridge",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "bridge",
			}),
		},
	}, projection.Contexts[1].Paths[0].Runtimes)
}

func TestProjectGivesOneClusterRuntimeDifferentParentsByResolutionContext(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("cluster-root", ""),
			clusterRuntime("base", ""),
			clusterRuntime("bridge", "base"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-b", "leaf", "bridge"),
			namespacedRuntime("team-a", "leaf", "bridge"),
			namespacedRuntime("team-b", "base", "cluster-root"),
			namespacedRuntime("team-a", "base", ""),
		},
	})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "bridge"})

	require.NoError(t, err)
	require.Len(t, projection.Contexts, 3)
	assert.Equal(t, []ResolutionContext{
		clusterContext(), namespacedContext("team-a"), namespacedContext("team-b"),
	}, projectionContexts(projection))
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "base"}},
		{
			Identity:       Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName:     "base",
			ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "base"}),
		},
	}, projection.Contexts[0].Paths[0].Runtimes)
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "base"}},
		{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "base",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
			}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
			ParentName: "bridge",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "bridge",
			}),
		},
	}, projection.Contexts[1].Paths[0].Runtimes)
	assert.Equal(t, []Runtime{
		{Identity: Identity{Kind: KindClusterServingRuntime, Name: "cluster-root"}},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-b", Name: "base"},
			ParentName: "cluster-root",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "cluster-root",
			}),
		},
		{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "base",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-b", Name: "base",
			}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-b", Name: "leaf"},
			ParentName: "bridge",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "bridge",
			}),
		},
	}, projection.Contexts[2].Paths[0].Runtimes)
}

func TestProjectDoesNotCreateFalseDescendantsAcrossResolutionContexts(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("base", ""),
			clusterRuntime("bridge", "base"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("team-a", "base", ""),
			namespacedRuntime("team-a", "leaf", "bridge"),
		},
	})
	require.NoError(t, err)

	clusterBase, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "base"})
	require.NoError(t, err)
	localBase, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
	})
	require.NoError(t, err)

	assert.Equal(t, []ContextProjection{{
		Context: clusterContext(),
		Paths: []ResolutionPath{
			{
				Subject:  Identity{Kind: KindClusterServingRuntime, Name: "base"},
				Runtimes: []Runtime{{Identity: Identity{Kind: KindClusterServingRuntime, Name: "base"}}},
			},
			{
				Subject: Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
				Runtimes: []Runtime{
					{Identity: Identity{Kind: KindClusterServingRuntime, Name: "base"}},
					{
						Identity:       Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
						ParentName:     "base",
						ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: "base"}),
					},
				},
			},
		},
	}}, clusterBase.Contexts, "the team-a leaf is shadowed away from the cluster base")
	assert.Equal(t, []ContextProjection{{
		Context: namespacedContext("team-a"),
		Paths: []ResolutionPath{
			{
				Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "base"},
				Runtimes: []Runtime{{Identity: Identity{
					Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
				}}},
			},
			{
				Subject: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
				Runtimes: []Runtime{
					{Identity: Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "base"}},
					{
						Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
						ParentName: "base",
						ResolvedParent: identityPointer(Identity{
							Kind: KindServingRuntime, Namespace: "team-a", Name: "base",
						}),
					},
					{
						Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "leaf"},
						ParentName: "bridge",
						ResolvedParent: identityPointer(Identity{
							Kind: KindClusterServingRuntime, Name: "bridge",
						}),
					},
				},
			},
		},
	}}, localBase.Contexts)
}

func TestProjectDetectsRepeatedNamesBeforeCrossScopeLookup(t *testing.T) {
	graph, err := Build(Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
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
	require.Len(t, projection.Contexts, 1)
	require.Len(t, projection.Contexts[0].Paths, 1)
	path := projection.Contexts[0].Paths[0]
	assert.Equal(t, []Runtime{
		{
			Identity:   Identity{Kind: KindClusterServingRuntime, Name: "bridge"},
			ParentName: "same-name",
			ResolvedParent: identityPointer(Identity{
				Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name",
			}),
		},
		{
			Identity:   Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
			ParentName: "bridge",
			ResolvedParent: identityPointer(Identity{
				Kind: KindClusterServingRuntime, Name: "bridge",
			}),
		},
	}, path.Runtimes)
	assert.Equal(t, &Issue{
		Code:       IssueCycleDetected,
		Subject:    Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
		ParentName: "same-name",
		Path: []Identity{
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
			{Kind: KindClusterServingRuntime, Name: "bridge"},
			{Kind: KindServingRuntime, Namespace: "team-a", Name: "same-name"},
		},
	}, path.Issue)
}

func TestProjectKeepsMaxDepthResultsSeparatePerHead(t *testing.T) {
	graph, err := Build(Snapshot{ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
		clusterRuntime("level-1", ""),
		clusterRuntime("level-2", "level-1"),
		clusterRuntime("level-3", "level-2"),
		clusterRuntime("level-4", "level-3"),
		clusterRuntime("level-5", "level-4"),
		clusterRuntime("level-6", "level-5"),
	}})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "level-5"})

	require.NoError(t, err)
	require.Len(t, projection.Contexts, 1)
	assert.Equal(t, []ResolutionPath{
		{
			Subject: Identity{Kind: KindClusterServingRuntime, Name: "level-5"},
			Runtimes: []Runtime{
				{Identity: Identity{Kind: KindClusterServingRuntime, Name: "level-1"}},
				resolvedClusterRuntime("level-2", "level-1"),
				resolvedClusterRuntime("level-3", "level-2"),
				resolvedClusterRuntime("level-4", "level-3"),
				resolvedClusterRuntime("level-5", "level-4"),
			},
		},
		{
			Subject: Identity{Kind: KindClusterServingRuntime, Name: "level-6"},
			Runtimes: []Runtime{
				{
					Identity:   Identity{Kind: KindClusterServingRuntime, Name: "level-2"},
					ParentName: "level-1",
				},
				resolvedClusterRuntime("level-3", "level-2"),
				resolvedClusterRuntime("level-4", "level-3"),
				resolvedClusterRuntime("level-5", "level-4"),
				resolvedClusterRuntime("level-6", "level-5"),
			},
			Issue: &Issue{
				Code:       IssueMaxDepthExceeded,
				Subject:    Identity{Kind: KindClusterServingRuntime, Name: "level-6"},
				ParentName: "level-1",
				Path: []Identity{
					{Kind: KindClusterServingRuntime, Name: "level-6"},
					{Kind: KindClusterServingRuntime, Name: "level-5"},
					{Kind: KindClusterServingRuntime, Name: "level-4"},
					{Kind: KindClusterServingRuntime, Name: "level-3"},
					{Kind: KindClusterServingRuntime, Name: "level-2"},
				},
			},
		},
	}, projection.Contexts[0].Paths)

	root, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "level-1"})
	require.NoError(t, err)
	assert.NotContains(t, pathSubjects(root), Identity{
		Kind: KindClusterServingRuntime, Name: "level-6",
	}, "the sixth head stopped before it visited level-1")
}

func TestProjectChecksMaxDepthBeforeCycleAtTheBoundary(t *testing.T) {
	graph, err := Build(Snapshot{ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
		clusterRuntime("level-1", "level-5"),
		clusterRuntime("level-2", "level-1"),
		clusterRuntime("level-3", "level-2"),
		clusterRuntime("level-4", "level-3"),
		clusterRuntime("level-5", "level-4"),
	}})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "level-5"})

	require.NoError(t, err)
	path := pathForSubject(t, projection, Identity{
		Kind: KindClusterServingRuntime, Name: "level-5",
	})
	require.NotNil(t, path.Issue)
	assert.Equal(t, IssueMaxDepthExceeded, path.Issue.Code)
	assert.Equal(t, "level-5", path.Issue.ParentName)
	assert.Equal(t, []Identity{
		{Kind: KindClusterServingRuntime, Name: "level-5"},
		{Kind: KindClusterServingRuntime, Name: "level-4"},
		{Kind: KindClusterServingRuntime, Name: "level-3"},
		{Kind: KindClusterServingRuntime, Name: "level-2"},
		{Kind: KindClusterServingRuntime, Name: "level-1"},
	}, path.Issue.Path)
	assert.Nil(t, path.Runtimes[0].ResolvedParent)
}

func TestProjectPreservesMissingParentIssueForEveryPathThatReachedTarget(t *testing.T) {
	graph, err := Build(Snapshot{ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
		clusterRuntime("target", "missing"),
		clusterRuntime("child", "target"),
	}})
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "target"})

	require.NoError(t, err)
	require.Len(t, projection.Contexts, 1)
	assert.Equal(t, []ResolutionPath{
		{
			Subject: Identity{Kind: KindClusterServingRuntime, Name: "target"},
			Runtimes: []Runtime{{
				Identity:   Identity{Kind: KindClusterServingRuntime, Name: "target"},
				ParentName: "missing",
			}},
			Issue: &Issue{
				Code:       IssueParentMissing,
				Subject:    Identity{Kind: KindClusterServingRuntime, Name: "target"},
				ParentName: "missing",
				Path:       []Identity{{Kind: KindClusterServingRuntime, Name: "target"}},
			},
		},
		{
			Subject: Identity{Kind: KindClusterServingRuntime, Name: "child"},
			Runtimes: []Runtime{
				{
					Identity:   Identity{Kind: KindClusterServingRuntime, Name: "target"},
					ParentName: "missing",
				},
				resolvedClusterRuntime("child", "target"),
			},
			Issue: &Issue{
				Code:       IssueParentMissing,
				Subject:    Identity{Kind: KindClusterServingRuntime, Name: "child"},
				ParentName: "missing",
				Path: []Identity{
					{Kind: KindClusterServingRuntime, Name: "child"},
					{Kind: KindClusterServingRuntime, Name: "target"},
				},
			},
		},
	}, projection.Contexts[0].Paths)
}

func TestProjectOrdersContextsAndPathsDeterministically(t *testing.T) {
	snapshot := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			clusterRuntime("target", ""),
			clusterRuntime("a-child", "target"),
			clusterRuntime("z-child", "target"),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			namespacedRuntime("z-team", "leaf", "target"),
			namespacedRuntime("a-team", "z-leaf", "target"),
			namespacedRuntime("a-team", "a-leaf", "target"),
		},
	}
	shuffled := Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{
			*snapshot.ClusterServingRuntimes[2].DeepCopy(),
			*snapshot.ClusterServingRuntimes[0].DeepCopy(),
			*snapshot.ClusterServingRuntimes[1].DeepCopy(),
		},
		ServingRuntimes: []omev1beta1.ServingRuntime{
			*snapshot.ServingRuntimes[1].DeepCopy(),
			*snapshot.ServingRuntimes[2].DeepCopy(),
			*snapshot.ServingRuntimes[0].DeepCopy(),
		},
	}
	graph, err := Build(snapshot)
	require.NoError(t, err)
	shuffledGraph, err := Build(shuffled)
	require.NoError(t, err)

	projection, err := graph.Project(Target{Kind: KindClusterServingRuntime, Name: "target"})
	require.NoError(t, err)
	shuffledProjection, err := shuffledGraph.Project(Target{Kind: KindClusterServingRuntime, Name: "target"})
	require.NoError(t, err)

	assert.Equal(t, []ResolutionContext{
		clusterContext(), namespacedContext("a-team"), namespacedContext("z-team"),
	}, projectionContexts(projection))
	assert.Equal(t, []Identity{
		{Kind: KindClusterServingRuntime, Name: "target"},
		{Kind: KindClusterServingRuntime, Name: "a-child"},
		{Kind: KindClusterServingRuntime, Name: "z-child"},
	}, contextPathSubjects(projection.Contexts[0]))
	assert.Equal(t, []Identity{
		{Kind: KindServingRuntime, Namespace: "a-team", Name: "a-leaf"},
		{Kind: KindServingRuntime, Namespace: "a-team", Name: "z-leaf"},
	}, contextPathSubjects(projection.Contexts[1]))
	assert.Equal(t, projection, shuffledProjection)
	encoded, err := json.Marshal(projection)
	require.NoError(t, err)
	shuffledEncoded, err := json.Marshal(shuffledProjection)
	require.NoError(t, err)
	assert.Equal(t, string(encoded), string(shuffledEncoded))
}

func TestProjectReturnsDefensiveCopies(t *testing.T) {
	graph, err := Build(Snapshot{ServingRuntimes: []omev1beta1.ServingRuntime{
		namespacedRuntime("team-a", "parent", ""),
		namespacedRuntime("team-a", "child", "parent"),
	}})
	require.NoError(t, err)

	first, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "child",
	})
	require.NoError(t, err)
	first.Contexts[0].Context.Namespace = "mutated"
	first.Contexts[0].Paths[0].Subject.Name = "mutated"
	first.Contexts[0].Paths[0].Runtimes[0].Identity.Name = "mutated"
	first.Contexts[0].Paths[0].Runtimes[1].ResolvedParent.Name = "mutated"

	again, err := graph.Project(Target{
		Kind: KindServingRuntime, Namespace: "team-a", Name: "child",
	})

	require.NoError(t, err)
	assert.Equal(t, namespacedContext("team-a"), again.Contexts[0].Context)
	assert.Equal(t, Identity{Kind: KindServingRuntime, Namespace: "team-a", Name: "child"}, again.Contexts[0].Paths[0].Subject)
	assert.Equal(t, "parent", again.Contexts[0].Paths[0].Runtimes[0].Identity.Name)
	assert.Equal(t, "parent", again.Contexts[0].Paths[0].Runtimes[1].ResolvedParent.Name)
}

func clusterContext() ResolutionContext {
	return ResolutionContext{Mode: ResolutionModeCluster}
}

func namespacedContext(namespace string) ResolutionContext {
	return ResolutionContext{Mode: ResolutionModeNamespaced, Namespace: namespace}
}

func resolvedClusterRuntime(name, parent string) Runtime {
	return Runtime{
		Identity:       Identity{Kind: KindClusterServingRuntime, Name: name},
		ParentName:     parent,
		ResolvedParent: identityPointer(Identity{Kind: KindClusterServingRuntime, Name: parent}),
	}
}

func projectionContexts(projection Projection) []ResolutionContext {
	result := make([]ResolutionContext, len(projection.Contexts))
	for i := range projection.Contexts {
		result[i] = projection.Contexts[i].Context
	}
	return result
}

func contextPathSubjects(context ContextProjection) []Identity {
	result := make([]Identity, len(context.Paths))
	for i := range context.Paths {
		result[i] = context.Paths[i].Subject
	}
	return result
}

func pathSubjects(projection Projection) []Identity {
	var result []Identity
	for _, context := range projection.Contexts {
		result = append(result, contextPathSubjects(context)...)
	}
	return result
}

func onlyPath(t *testing.T, projection Projection) ResolutionPath {
	t.Helper()
	require.Len(t, projection.Contexts, 1)
	require.Len(t, projection.Contexts[0].Paths, 1)
	return projection.Contexts[0].Paths[0]
}

func pathForSubject(t *testing.T, projection Projection, subject Identity) ResolutionPath {
	t.Helper()
	for _, context := range projection.Contexts {
		for _, path := range context.Paths {
			if path.Subject == subject {
				return path
			}
		}
	}
	require.Failf(t, "path not found", "subject: %#v", subject)
	return ResolutionPath{}
}
