// Package runtimepreset implements a mutating webhook that injects
// engine defaults from a Go-defined preset when ome.io/engine: <name>
// names a known engine. The runtime-inheritance resolver composes on
// top at consumer call sites; $(VAR) placeholders are substituted by
// Kubernetes at pod-create time.
package runtimepreset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

// +kubebuilder:webhook:verbs=create;update,path=/mutate-ome-io-v1beta1-servingruntime-preset,mutating=true,failurePolicy=fail,groups=ome.io,resources=servingruntimes,versions=v1beta1,name=servingruntime.ome-webhook-server.preset,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ome-io-v1beta1-clusterservingruntime-preset,mutating=true,failurePolicy=fail,groups=ome.io,resources=clusterservingruntimes,versions=v1beta1,name=clusterservingruntime.ome-webhook-server.preset,sideEffects=None,admissionReviewVersions=v1

var log = logf.Log.WithName("runtimepreset-mutator")

// presets is the registry — adding a new engine is one preset file +
// one map entry.
var presets = map[string]func() *v1beta1.ServingRuntimeSpec{
	"sglang-pd": sglangPDPreset,
}

type ServingRuntimeMutator struct {
	Decoder admission.Decoder
}

func (m *ServingRuntimeMutator) Handle(_ context.Context, req admission.Request) admission.Response {
	sr := &v1beta1.ServingRuntime{}
	if err := m.Decoder.Decode(req, sr); err != nil {
		log.Error(err, "decode ServingRuntime", "name", sr.Name, "namespace", sr.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}
	merged, resp := applyPreset(sr.Annotations, &sr.Spec)
	if resp != nil {
		return *resp
	}
	if merged == nil {
		return admission.Allowed("no engine preset annotation")
	}
	original, errResp := marshalForPatch(sr)
	if errResp != nil {
		return *errResp
	}
	sr.Spec = *merged
	return patchResponse(original, sr)
}

type ClusterServingRuntimeMutator struct {
	Decoder admission.Decoder
}

func (m *ClusterServingRuntimeMutator) Handle(_ context.Context, req admission.Request) admission.Response {
	csr := &v1beta1.ClusterServingRuntime{}
	if err := m.Decoder.Decode(req, csr); err != nil {
		log.Error(err, "decode ClusterServingRuntime", "name", csr.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}
	merged, resp := applyPreset(csr.Annotations, &csr.Spec)
	if resp != nil {
		return *resp
	}
	if merged == nil {
		return admission.Allowed("no engine preset annotation")
	}
	original, errResp := marshalForPatch(csr)
	if errResp != nil {
		return *errResp
	}
	csr.Spec = *merged
	return patchResponse(original, csr)
}

// applyPreset returns (merged, nil) on success, (nil, &resp) on
// passthrough or rejection. merged==nil && resp==nil means no
// annotation set — caller should pass through.
func applyPreset(annotations map[string]string, spec *v1beta1.ServingRuntimeSpec) (*v1beta1.ServingRuntimeSpec, *admission.Response) {
	engine, ok := annotations[constants.RuntimeEngineAnnotationKey]
	if !ok || engine == "" {
		return nil, nil
	}
	factory, known := presets[engine]
	if !known {
		resp := admission.Denied(fmt.Sprintf(
			"unknown %s value %q (known: %s)",
			constants.RuntimeEngineAnnotationKey, engine, strings.Join(knownPresetNames(), ", "),
		))
		return nil, &resp
	}
	// Runtime (child) wins on collision; preset (parent) fills unset
	// positions. Same contract as the runtime-inheritance resolver, so
	// preset + inheritance compose commutatively for any field set by
	// exactly one source.
	merged, err := runtimeinheritance.Merge(factory(), spec)
	if err != nil {
		resp := admission.Errored(http.StatusInternalServerError, fmt.Errorf("merge %s preset: %w", engine, err))
		return nil, &resp
	}
	return merged, nil
}

// marshalForPatch serializes the decoded (pre-mutation) object as the
// patch base. Diffing against the decoded object, not the raw request,
// keeps fields this build's API types don't know out of the patch — a
// raw-based diff would emit remove ops that destroy them.
func marshalForPatch(obj any) ([]byte, *admission.Response) {
	out, err := json.Marshal(obj)
	if err != nil {
		log.Error(err, "marshal runtime for patch base")
		resp := admission.Errored(http.StatusInternalServerError, err)
		return nil, &resp
	}
	// ServingRuntimePodSpec.Containers (embedded) lacks omitempty, so
	// a nil slice marshals as "null". CRD validation rejects null
	// arrays; strip them so base and target are symmetric.
	out, err = runtimeinheritance.StripJSONNulls(out)
	if err != nil {
		log.Error(err, "strip nulls from runtime patch base")
		resp := admission.Errored(http.StatusInternalServerError, err)
		return nil, &resp
	}
	return out, nil
}

func patchResponse(original []byte, obj any) admission.Response {
	out, err := json.Marshal(obj)
	if err != nil {
		log.Error(err, "marshal mutated runtime")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	// ServingRuntimePodSpec.Containers (embedded) lacks omitempty, so
	// a nil slice marshals as "null". CRD validation rejects null
	// arrays. Strip nulls before computing the patch.
	out, err = runtimeinheritance.StripJSONNulls(out)
	if err != nil {
		log.Error(err, "strip nulls from mutated runtime")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(original, out)
}

func knownPresetNames() []string {
	out := make([]string, 0, len(presets))
	for k := range presets {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
