// Package runtimegraph indexes an already-collected snapshot of OME serving
// runtimes. It performs no cluster reads.
package runtimegraph

import (
	"errors"
	"fmt"
	"sort"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// Kind identifies a runtime's Kubernetes kind.
type Kind string

const (
	KindServingRuntime        Kind = "ServingRuntime"
	KindClusterServingRuntime Kind = "ClusterServingRuntime"
)

// Identity uniquely identifies one runtime in a snapshot.
type Identity struct {
	Kind      Kind   `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// Target selects the runtime to project. An empty Kind requests discovery.
type Target struct {
	Kind      Kind
	Namespace string
	Name      string
}

// Snapshot contains runtime objects collected by a caller.
type Snapshot struct {
	ClusterServingRuntimes []omev1beta1.ClusterServingRuntime
	ServingRuntimes        []omev1beta1.ServingRuntime
}

// Runtime contains the bounded graph data retained for one runtime.
type Runtime struct {
	Identity       Identity  `json:"identity"`
	ParentName     string    `json:"parentName,omitempty"`
	ResolvedParent *Identity `json:"resolvedParent,omitempty"`
}

// IssueCode classifies an inheritance topology problem.
type IssueCode string

const (
	IssueParentMissing    IssueCode = "ParentMissing"
	IssueCycleDetected    IssueCode = "CycleDetected"
	IssueMaxDepthExceeded IssueCode = "MaxDepthExceeded"
)

// Issue describes why Subject's inheritance chain could not be resolved.
// Path is ordered from Subject toward its ancestors.
type Issue struct {
	Code       IssueCode  `json:"code"`
	Subject    Identity   `json:"subject"`
	ParentName string     `json:"parentName,omitempty"`
	Path       []Identity `json:"path"`
}

// Projection is the graph view rooted at a target runtime. Ancestors are
// ordered root first and exclude Target.
type Projection struct {
	Target      Runtime   `json:"target"`
	Ancestors   []Runtime `json:"ancestors"`
	Descendants []Subtree `json:"descendants"`
	Issues      []Issue   `json:"issues"`
}

// Subtree is one node in the target's descendant tree.
type Subtree struct {
	Runtime  Runtime   `json:"runtime"`
	Children []Subtree `json:"children"`
}

var (
	// ErrInvalidTarget indicates that a target lacks required identity data or
	// names an unsupported kind.
	ErrInvalidTarget = errors.New("runtime target is invalid")
	// ErrTargetNotFound indicates that no runtime matched a target.
	ErrTargetNotFound = errors.New("runtime target was not found")
	// ErrTargetAmbiguous indicates that an implicit target matched multiple
	// runtime identities.
	ErrTargetAmbiguous = errors.New("runtime target is ambiguous")
	// ErrDuplicateRuntime indicates that a snapshot repeats an identity.
	ErrDuplicateRuntime = errors.New("runtime snapshot contains a duplicate identity")
	// ErrInvalidRuntime indicates that a snapshot runtime lacks a complete
	// Kubernetes identity.
	ErrInvalidRuntime = errors.New("runtime snapshot contains an invalid identity")
)

// InvalidRuntimeError identifies an incomplete snapshot identity.
type InvalidRuntimeError struct {
	Identity Identity
}

func (e *InvalidRuntimeError) Error() string {
	return fmt.Sprintf("%v: %s", ErrInvalidRuntime, formatIdentity(e.Identity))
}

func (e *InvalidRuntimeError) Unwrap() error { return ErrInvalidRuntime }

// DuplicateRuntimeError identifies the repeated snapshot identity.
type DuplicateRuntimeError struct {
	Identity Identity
}

func (e *DuplicateRuntimeError) Error() string {
	return fmt.Sprintf("%v: %s", ErrDuplicateRuntime, formatIdentity(e.Identity))
}

func (e *DuplicateRuntimeError) Unwrap() error { return ErrDuplicateRuntime }

// TargetNotFoundError retains the unresolved target for callers that need to
// construct a diagnostic.
type TargetNotFoundError struct {
	Target Target
}

func (e *TargetNotFoundError) Error() string {
	return fmt.Sprintf("%v: %q", ErrTargetNotFound, e.Target.Name)
}

func (e *TargetNotFoundError) Unwrap() error { return ErrTargetNotFound }

// AmbiguousTargetError reports every candidate for an ambiguous target.
type AmbiguousTargetError struct {
	Target     Target
	Candidates []Identity
}

func (e *AmbiguousTargetError) Error() string {
	return fmt.Sprintf("%v: %q matched %d runtimes", ErrTargetAmbiguous, e.Target.Name, len(e.Candidates))
}

func (e *AmbiguousTargetError) Unwrap() error { return ErrTargetAmbiguous }

// Graph is an immutable runtime index built from a snapshot.
type Graph struct {
	runtimes map[Identity]runtimeNode
	children map[Identity][]Identity
}

type runtimeNode struct {
	identity   Identity
	parentName string
	parent     Identity
	hasParent  bool
}

// Build indexes a runtime snapshot.
func Build(snapshot Snapshot) (*Graph, error) {
	runtimes := make(map[Identity]runtimeNode, len(snapshot.ClusterServingRuntimes)+len(snapshot.ServingRuntimes))
	for i := range snapshot.ClusterServingRuntimes {
		object := &snapshot.ClusterServingRuntimes[i]
		identity := Identity{Kind: KindClusterServingRuntime, Name: object.Name}
		if identity.Name == "" {
			return nil, &InvalidRuntimeError{Identity: identity}
		}
		if _, duplicate := runtimes[identity]; duplicate {
			return nil, &DuplicateRuntimeError{Identity: identity}
		}
		runtimes[identity] = runtimeNode{
			identity: identity, parentName: object.Annotations[constants.RuntimeInheritFromAnnotationKey],
		}
	}
	for i := range snapshot.ServingRuntimes {
		object := &snapshot.ServingRuntimes[i]
		identity := Identity{Kind: KindServingRuntime, Namespace: object.Namespace, Name: object.Name}
		if identity.Namespace == "" || identity.Name == "" {
			return nil, &InvalidRuntimeError{Identity: identity}
		}
		if _, duplicate := runtimes[identity]; duplicate {
			return nil, &DuplicateRuntimeError{Identity: identity}
		}
		runtimes[identity] = runtimeNode{
			identity: identity, parentName: object.Annotations[constants.RuntimeInheritFromAnnotationKey],
		}
	}
	graph := &Graph{runtimes: runtimes, children: make(map[Identity][]Identity)}
	graph.resolveParents()
	graph.indexChildren()
	return graph, nil
}

// Project resolves target and returns its graph view.
func (g *Graph) Project(target Target) (Projection, error) {
	if target.Name == "" {
		return Projection{}, fmt.Errorf("%w: name is required", ErrInvalidTarget)
	}
	if target.Kind == KindServingRuntime && target.Namespace == "" {
		return Projection{}, fmt.Errorf("%w: namespace is required for ServingRuntime", ErrInvalidTarget)
	}
	if target.Kind != "" && target.Kind != KindServingRuntime && target.Kind != KindClusterServingRuntime {
		return Projection{}, fmt.Errorf("%w: unsupported kind %q", ErrInvalidTarget, target.Kind)
	}
	identity, err := g.resolveTarget(target)
	if err != nil {
		return Projection{}, err
	}
	node, ok := g.runtimes[identity]
	if !ok {
		return Projection{}, &TargetNotFoundError{Target: target}
	}
	ancestors, issue := g.ancestorSpine(identity)
	issues := make([]Issue, 0, 1)
	if issue != nil {
		issues = append(issues, *issue)
	}
	return Projection{
		Target: node.runtime(), Ancestors: ancestors,
		Descendants: g.descendantSubtrees(identity, map[Identity]struct{}{identity: {}}),
		Issues:      issues,
	}, nil
}

func (g *Graph) ancestorSpine(subject Identity) ([]Runtime, *Issue) {
	path := []Identity{subject}
	visited := map[Identity]struct{}{subject: {}}
	current := g.runtimes[subject]
	for current.parentName != "" {
		if len(path) >= constants.RuntimeInheritMaxDepth {
			return reverseRuntimePath(g, path[1:]), &Issue{
				Code: IssueMaxDepthExceeded, Subject: subject, ParentName: current.parentName,
				Path: append([]Identity{}, path...),
			}
		}
		if !current.hasParent {
			return reverseRuntimePath(g, path[1:]), &Issue{
				Code: IssueParentMissing, Subject: subject, ParentName: current.parentName,
				Path: append([]Identity{}, path...),
			}
		}
		if _, seen := visited[current.parent]; seen {
			cyclePath := append(append([]Identity{}, path...), current.parent)
			return reverseRuntimePath(g, path[1:]), &Issue{
				Code: IssueCycleDetected, Subject: subject, ParentName: current.parentName, Path: cyclePath,
			}
		}
		visited[current.parent] = struct{}{}
		path = append(path, current.parent)
		current = g.runtimes[current.parent]
	}
	return reverseRuntimePath(g, path[1:]), nil
}

func reverseRuntimePath(g *Graph, childFirst []Identity) []Runtime {
	runtimes := make([]Runtime, len(childFirst))
	for i := range childFirst {
		runtimes[len(childFirst)-1-i] = g.runtimes[childFirst[i]].runtime()
	}
	return runtimes
}

func (g *Graph) resolveTarget(target Target) (Identity, error) {
	if target.Kind == KindClusterServingRuntime {
		return Identity{Kind: target.Kind, Name: target.Name}, nil
	}
	if target.Kind == KindServingRuntime {
		return Identity(target), nil
	}
	candidates := make([]Identity, 0, 2)
	cluster := Identity{Kind: KindClusterServingRuntime, Name: target.Name}
	if _, ok := g.runtimes[cluster]; ok {
		candidates = append(candidates, cluster)
	}
	namespaced := Identity{Kind: KindServingRuntime, Namespace: target.Namespace, Name: target.Name}
	if _, ok := g.runtimes[namespaced]; ok {
		candidates = append(candidates, namespaced)
	}
	if len(candidates) > 1 {
		return Identity{}, &AmbiguousTargetError{Target: target, Candidates: candidates}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return Identity{}, &TargetNotFoundError{Target: target}
}

func (g *Graph) resolveParents() {
	for identity, node := range g.runtimes {
		if node.parentName == "" {
			continue
		}
		var parent Identity
		if identity.Kind == KindClusterServingRuntime {
			parent = Identity{Kind: KindClusterServingRuntime, Name: node.parentName}
		} else {
			parent = Identity{Kind: KindServingRuntime, Namespace: identity.Namespace, Name: node.parentName}
			if _, ok := g.runtimes[parent]; !ok {
				parent = Identity{Kind: KindClusterServingRuntime, Name: node.parentName}
			}
		}
		if _, ok := g.runtimes[parent]; !ok {
			continue
		}
		node.parent = parent
		node.hasParent = true
		g.runtimes[identity] = node
	}
}

func (g *Graph) indexChildren() {
	for identity, node := range g.runtimes {
		if node.hasParent {
			g.children[node.parent] = append(g.children[node.parent], identity)
		}
	}
	for parent := range g.children {
		sort.Slice(g.children[parent], func(i, j int) bool {
			return identityLess(g.children[parent][i], g.children[parent][j])
		})
	}
}

func (g *Graph) descendantSubtrees(parent Identity, path map[Identity]struct{}) []Subtree {
	result := make([]Subtree, 0, len(g.children[parent]))
	for _, child := range g.children[parent] {
		if _, seen := path[child]; seen {
			continue
		}
		childPath := make(map[Identity]struct{}, len(path)+1)
		for identity := range path {
			childPath[identity] = struct{}{}
		}
		childPath[child] = struct{}{}
		result = append(result, Subtree{
			Runtime:  g.runtimes[child].runtime(),
			Children: g.descendantSubtrees(child, childPath),
		})
	}
	return result
}

func identityLess(left, right Identity) bool {
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	if left.Namespace != right.Namespace {
		return left.Namespace < right.Namespace
	}
	return left.Name < right.Name
}

func formatIdentity(identity Identity) string {
	if identity.Namespace == "" {
		return fmt.Sprintf("%s/%s", identity.Kind, identity.Name)
	}
	return fmt.Sprintf("%s/%s/%s", identity.Kind, identity.Namespace, identity.Name)
}

func (n runtimeNode) runtime() Runtime {
	result := Runtime{Identity: n.identity, ParentName: n.parentName}
	if n.hasParent {
		parent := n.parent
		result.ResolvedParent = &parent
	}
	return result
}
