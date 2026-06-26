package runtimerevision

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// FindOrCreate looks up a ControllerRevision in omeNamespace that
// matches the (source runtime, content hash) labels. Returns its
// name if found; creates a new one carrying the serialized spec if
// not. Hash dedup means a second ISVC pinning to the same content
// reuses the same revision.
func FindOrCreate(
	ctx context.Context,
	c client.Client,
	omeNamespace string,
	kind SourceKind,
	sourceNamespace, runtimeName string,
	resolvedSpec *v1beta1.ServingRuntimeSpec,
) (string, error) {
	_, shortHash, err := Hash(resolvedSpec)
	if err != nil {
		return "", err
	}

	if existing, err := findByHash(ctx, c, omeNamespace, runtimeName, shortHash); err != nil {
		return "", err
	} else if existing != "" {
		return existing, nil
	}

	name := Name(kind, sourceNamespace, runtimeName, shortHash)
	specBytes, err := json.Marshal(resolvedSpec)
	if err != nil {
		return "", fmt.Errorf("marshal spec for revision %s: %w", name, err)
	}
	rev := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: omeNamespace,
			Labels: map[string]string{
				constants.RuntimeRevisionOfLabelKey:          runtimeName,
				constants.RuntimeRevisionOfKindLabelKey:      string(kind),
				constants.RuntimeRevisionOfNamespaceLabelKey: sourceNamespace,
				constants.RuntimeRevisionHashLabelKey:        shortHash,
			},
			Annotations: map[string]string{
				constants.RuntimeRevisionCreatedByKey: constants.RuntimeRevisionCreatedByOMEValue,
			},
		},
		Data:     runtime.RawExtension{Raw: specBytes},
		Revision: 1, // Phase 3 GC will set this to a monotonic counter; Phase 1 doesn't rely on it.
	}
	if err := c.Create(ctx, rev); err != nil {
		// A concurrent writer raced us; resolve to the existing name.
		if apierrors.IsAlreadyExists(err) {
			return name, nil
		}
		return "", fmt.Errorf("create revision %s: %w", name, err)
	}
	return name, nil
}

// FetchRevision reads a ControllerRevision by name from omeNamespace.
// Returns IsNotFound when the revision has been deleted or GC'd; the
// caller surfaces that as RuntimeDrifted=True / Reason=RevisionMissing.
// Use this when you need to inspect labels/annotations
// (e.g., cross-check ome.io/runtime-of); Fetch is the convenience
// wrapper when only the decoded spec is needed.
func FetchRevision(
	ctx context.Context,
	c client.Client,
	omeNamespace, name string,
) (*appsv1.ControllerRevision, error) {
	rev := &appsv1.ControllerRevision{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: omeNamespace, Name: name}, rev); err != nil {
		return nil, err
	}
	return rev, nil
}

// DecodeSpec unmarshals a revision's Data field into a *ServingRuntimeSpec.
func DecodeSpec(rev *appsv1.ControllerRevision) (*v1beta1.ServingRuntimeSpec, error) {
	if rev == nil {
		return nil, fmt.Errorf("DecodeSpec: nil revision")
	}
	spec := &v1beta1.ServingRuntimeSpec{}
	if err := json.Unmarshal(rev.Data.Raw, spec); err != nil {
		return nil, fmt.Errorf("decode revision %s/%s Data: %w", rev.Namespace, rev.Name, err)
	}
	return spec, nil
}

// Fetch is the convenience wrapper: FetchRevision + DecodeSpec.
func Fetch(
	ctx context.Context,
	c client.Client,
	omeNamespace, name string,
) (*v1beta1.ServingRuntimeSpec, error) {
	rev, err := FetchRevision(ctx, c, omeNamespace, name)
	if err != nil {
		return nil, err
	}
	return DecodeSpec(rev)
}

// findByHash returns the name of an existing revision matching the
// (runtime, shortHash) label pair, or "" if none exist.
func findByHash(
	ctx context.Context,
	c client.Client,
	omeNamespace, runtimeName, shortHash string,
) (string, error) {
	var list appsv1.ControllerRevisionList
	if err := c.List(ctx, &list,
		client.InNamespace(omeNamespace),
		client.MatchingLabels{
			constants.RuntimeRevisionOfLabelKey:   runtimeName,
			constants.RuntimeRevisionHashLabelKey: shortHash,
		},
	); err != nil {
		return "", fmt.Errorf("list ControllerRevisions in %s: %w", omeNamespace, err)
	}
	if len(list.Items) == 0 {
		return "", nil
	}
	return list.Items[0].Name, nil
}
