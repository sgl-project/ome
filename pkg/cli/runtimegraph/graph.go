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

// Snapshot contains runtime objects collected by a caller. Build treats both
// lists as authoritative: absence means a runtime is not present. Callers with
// truncated or unavailable lists must preserve that incompleteness alongside
// the projection instead of presenting absence-based fallbacks or issues as
// definitive cluster state.
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

// ResolutionMode identifies the lookup policy the controller keeps for an
// entire inheritance walk.
type ResolutionMode string

const (
	// ResolutionModeCluster resolves every parent as a
	// ClusterServingRuntime.
	ResolutionModeCluster ResolutionMode = "Cluster"
	// ResolutionModeNamespaced resolves every parent as a ServingRuntime in
	// Namespace first, then falls back to a ClusterServingRuntime.
	ResolutionModeNamespaced ResolutionMode = "Namespaced"
)

// ResolutionContext is the fixed lookup context for one controller
// inheritance walk. Namespace is empty for cluster-only resolution.
type ResolutionContext struct {
	Mode      ResolutionMode `json:"mode"`
	Namespace string         `json:"namespace,omitempty"`
}

// ResolutionPath is the exact bounded walk for one real runtime head.
// Runtimes are ordered from the observed root or error boundary to Subject.
// An unsuccessful walk retains every runtime visited before it stopped.
type ResolutionPath struct {
	Subject  Identity  `json:"subject"`
	Runtimes []Runtime `json:"runtimes"`
	Issue    *Issue    `json:"issue,omitempty"`
}

// ContextProjection groups real-head paths that visited Target under one
// fixed controller lookup context. Paths are not merged: both namespace
// shadowing and the maximum-depth budget can give the same runtime occurrence
// different visible ancestors in different head walks.
type ContextProjection struct {
	Context ResolutionContext `json:"context"`
	Paths   []ResolutionPath  `json:"paths"`
}

// Projection contains every real-head controller walk that visited Target.
// A ServingRuntime target has one namespaced context. A
// ClusterServingRuntime target always has its direct cluster context and may
// also occur in namespaced contexts reached by ServingRuntime heads.
type Projection struct {
	Target   Identity            `json:"target"`
	Contexts []ContextProjection `json:"contexts"`
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
	heads    []Identity
}

type runtimeNode struct {
	identity   Identity
	parentName string
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
	heads := make([]Identity, 0, len(runtimes))
	for identity := range runtimes {
		heads = append(heads, identity)
	}
	sort.Slice(heads, func(i, j int) bool { return identityLess(heads[i], heads[j]) })
	return &Graph{runtimes: runtimes, heads: heads}, nil
}

// Project resolves target and returns its graph view.
func (g *Graph) Project(target Target) (Projection, error) {
	if target.Name == "" {
		return Projection{}, fmt.Errorf("%w: name is required", ErrInvalidTarget)
	}
	if target.Kind == KindServingRuntime && target.Namespace == "" {
		return Projection{}, fmt.Errorf("%w: namespace is required for ServingRuntime", ErrInvalidTarget)
	}
	if target.Kind == KindClusterServingRuntime && target.Namespace != "" {
		return Projection{}, fmt.Errorf("%w: namespace is forbidden for ClusterServingRuntime", ErrInvalidTarget)
	}
	if target.Kind != "" && target.Kind != KindServingRuntime && target.Kind != KindClusterServingRuntime {
		return Projection{}, fmt.Errorf("%w: unsupported kind %q", ErrInvalidTarget, target.Kind)
	}
	identity, err := g.resolveTarget(target)
	if err != nil {
		return Projection{}, err
	}
	if _, ok := g.runtimes[identity]; !ok {
		return Projection{}, &TargetNotFoundError{Target: target}
	}

	pathsByContext := make(map[ResolutionContext][]ResolutionPath)
	for _, head := range g.heads {
		context := resolutionContextForHead(head)
		walk := g.resolveHead(head, context)
		if !walk.visits(identity) {
			continue
		}
		pathsByContext[context] = append(pathsByContext[context], walk.project())
	}

	contexts := make([]ResolutionContext, 0, len(pathsByContext))
	for context := range pathsByContext {
		contexts = append(contexts, context)
	}
	sort.Slice(contexts, func(i, j int) bool { return resolutionContextLess(contexts[i], contexts[j]) })
	result := Projection{Target: identity, Contexts: make([]ContextProjection, 0, len(contexts))}
	for _, context := range contexts {
		paths := pathsByContext[context]
		sort.Slice(paths, func(i, j int) bool {
			leftTarget := paths[i].Subject == identity
			rightTarget := paths[j].Subject == identity
			if leftTarget != rightTarget {
				return leftTarget
			}
			return identityLess(paths[i].Subject, paths[j].Subject)
		})
		result.Contexts = append(result.Contexts, ContextProjection{Context: context, Paths: paths})
	}
	return result, nil
}

type resolvedHead struct {
	subject            Identity
	runtimesChildFirst []Runtime
	issue              *Issue
}

func (g *Graph) resolveHead(subject Identity, context ResolutionContext) resolvedHead {
	start := g.runtimes[subject].runtime()
	result := resolvedHead{subject: subject, runtimesChildFirst: []Runtime{start}}
	visited := map[string]Identity{subject.Name: subject}

	for {
		currentIndex := len(result.runtimesChildFirst) - 1
		current := result.runtimesChildFirst[currentIndex]
		if current.ParentName == "" {
			return result
		}
		path := runtimeIdentities(result.runtimesChildFirst)
		if len(result.runtimesChildFirst) >= constants.RuntimeInheritMaxDepth {
			result.issue = &Issue{
				Code: IssueMaxDepthExceeded, Subject: subject,
				ParentName: current.ParentName, Path: path,
			}
			return result
		}
		if closing, seen := visited[current.ParentName]; seen {
			parent := closing
			result.runtimesChildFirst[currentIndex].ResolvedParent = &parent
			result.issue = &Issue{
				Code: IssueCycleDetected, Subject: subject,
				ParentName: current.ParentName, Path: append(path, closing),
			}
			return result
		}
		parent, found := g.lookupParent(context, current.ParentName)
		if !found {
			result.issue = &Issue{
				Code: IssueParentMissing, Subject: subject,
				ParentName: current.ParentName, Path: path,
			}
			return result
		}
		resolvedParent := parent
		result.runtimesChildFirst[currentIndex].ResolvedParent = &resolvedParent
		visited[parent.Name] = parent
		result.runtimesChildFirst = append(result.runtimesChildFirst, g.runtimes[parent].runtime())
	}
}

func (g *Graph) lookupParent(context ResolutionContext, name string) (Identity, bool) {
	if context.Mode == ResolutionModeNamespaced {
		namespaced := Identity{Kind: KindServingRuntime, Namespace: context.Namespace, Name: name}
		if _, ok := g.runtimes[namespaced]; ok {
			return namespaced, true
		}
	}
	cluster := Identity{Kind: KindClusterServingRuntime, Name: name}
	_, ok := g.runtimes[cluster]
	return cluster, ok
}

func resolutionContextForHead(head Identity) ResolutionContext {
	if head.Kind == KindServingRuntime {
		return ResolutionContext{Mode: ResolutionModeNamespaced, Namespace: head.Namespace}
	}
	return ResolutionContext{Mode: ResolutionModeCluster}
}

func resolutionContextLess(left, right ResolutionContext) bool {
	if left.Mode != right.Mode {
		return left.Mode == ResolutionModeCluster
	}
	return left.Namespace < right.Namespace
}

func runtimeIdentities(runtimes []Runtime) []Identity {
	result := make([]Identity, len(runtimes))
	for i := range runtimes {
		result[i] = runtimes[i].Identity
	}
	return result
}

func (r resolvedHead) visits(target Identity) bool {
	for _, runtime := range r.runtimesChildFirst {
		if runtime.Identity == target {
			return true
		}
	}
	return false
}

func (r resolvedHead) project() ResolutionPath {
	runtimes := make([]Runtime, len(r.runtimesChildFirst))
	for i := range r.runtimesChildFirst {
		runtimes[len(r.runtimesChildFirst)-1-i] = copyRuntime(r.runtimesChildFirst[i])
	}
	return ResolutionPath{Subject: r.subject, Runtimes: runtimes, Issue: copyIssue(r.issue)}
}

func copyRuntime(runtime Runtime) Runtime {
	result := runtime
	if runtime.ResolvedParent != nil {
		parent := *runtime.ResolvedParent
		result.ResolvedParent = &parent
	}
	return result
}

func copyIssue(issue *Issue) *Issue {
	if issue == nil {
		return nil
	}
	result := *issue
	result.Path = append([]Identity{}, issue.Path...)
	return &result
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
	return Runtime{Identity: n.identity, ParentName: n.parentName}
}
