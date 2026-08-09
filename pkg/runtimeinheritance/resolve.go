package runtimeinheritance

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// RuntimeRef is the scope-agnostic slice of a runtime CR the resolver
// needs. Callers project their CR (cluster or namespaced) into this
// shape.
type RuntimeRef struct {
	Name       string
	Spec       *v1beta1.ServingRuntimeSpec
	ParentName string
}

// Fetcher looks up a parent runtime by name. Return ErrParentNotFound
// (or a wrapped error errors.Is matches) when the parent doesn't exist.
type Fetcher func(ctx context.Context, name string) (*RuntimeRef, error)

// ErrParentNotFound signals the named parent doesn't exist; the
// resolver wraps it as ParentNotFoundError.
var ErrParentNotFound = errors.New("parent runtime not found")

// ParentNotFoundError carries the missing parent name + chain walked.
type ParentNotFoundError struct {
	Parent string
	Chain  []string
}

func (e *ParentNotFoundError) Error() string {
	return fmt.Sprintf("inherit-from parent %q not found (chain so far: %v)", e.Parent, e.Chain)
}

// CycleError carries the chain that closed the cycle.
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("inheritance cycle detected: %v", e.Cycle)
}

// MaxDepthExceededError carries the chain walked up to the limit.
type MaxDepthExceededError struct {
	MaxDepth int
	Chain    []string
}

func (e *MaxDepthExceededError) Error() string {
	return fmt.Sprintf("inheritance chain exceeds max depth %d (chain: %v)", e.MaxDepth, e.Chain)
}

// Resolve walks start's inherit-from chain to its root via fetch and
// merges bottom-up. Returns the effective spec and the root-first
// chain of names walked.
func Resolve(ctx context.Context, start *RuntimeRef, fetch Fetcher, maxDepth int) (*v1beta1.ServingRuntimeSpec, []string, error) {
	if start == nil {
		return nil, nil, errors.New("runtimeinheritance.Resolve: start is nil")
	}

	visited := map[string]bool{start.Name: true}
	chainChildFirst := []string{start.Name}
	specsChildFirst := []*v1beta1.ServingRuntimeSpec{start.Spec}

	current := start
	for current.ParentName != "" {
		if len(chainChildFirst) >= maxDepth {
			return nil, nil, &MaxDepthExceededError{MaxDepth: maxDepth, Chain: chainChildFirst}
		}
		if visited[current.ParentName] {
			return nil, nil, &CycleError{Cycle: append(chainChildFirst, current.ParentName)}
		}
		parent, err := fetch(ctx, current.ParentName)
		if err != nil {
			if errors.Is(err, ErrParentNotFound) {
				return nil, nil, &ParentNotFoundError{Parent: current.ParentName, Chain: chainChildFirst}
			}
			return nil, nil, fmt.Errorf("fetch parent %q: %w", current.ParentName, err)
		}
		visited[parent.Name] = true
		chainChildFirst = append(chainChildFirst, parent.Name)
		specsChildFirst = append(specsChildFirst, parent.Spec)
		current = parent
	}

	chain := reverseStrings(chainChildFirst)
	specsRootFirst := reverseSpecs(specsChildFirst)

	effective := specsRootFirst[0].DeepCopy()
	for i := 1; i < len(specsRootFirst); i++ {
		merged, err := Merge(effective, specsRootFirst[i])
		if err != nil {
			return nil, nil, fmt.Errorf("merge %s onto effective: %w", chain[i], err)
		}
		effective = merged
	}
	return effective, chain, nil
}

func reverseStrings(in []string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

func reverseSpecs(in []*v1beta1.ServingRuntimeSpec) []*v1beta1.ServingRuntimeSpec {
	out := make([]*v1beta1.ServingRuntimeSpec, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}
