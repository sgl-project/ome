package servingruntime

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// attachRuntimeRef stamps an AutoscalerPolicyRef onto one runtime component
// config slot; empty component leaves the spec ref-free.
func attachRuntimeRef(spec *v1beta1.ServingRuntimeSpec, component string) {
	ref := &v1beta1.AutoscalerPolicyRef{Name: "request-activity-v1"}
	switch component {
	case "engineConfig":
		spec.EngineConfig = &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
		}
	case "decoderConfig":
		spec.DecoderConfig = &v1beta1.DecoderSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
		}
	case "routerConfig":
		spec.RouterConfig = &v1beta1.RouterSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
		}
	}
}

// A policy ref stored on a runtime component config would be admitted and
// completely inert — the resolver reads refs off the InferenceService only —
// so both runtime webhooks must reject it with pointed guidance.
func TestServingRuntime_AutoscalerPolicyRefRejected(t *testing.T) {
	tests := []struct {
		name        string
		component   string
		wantAllowed bool
	}{
		{name: "no ref allowed", component: "", wantAllowed: true},
		{name: "engineConfig ref denied", component: "engineConfig"},
		{name: "decoderConfig ref denied", component: "decoderConfig"},
		{name: "routerConfig ref denied", component: "routerConfig"},
	}
	for _, tt := range tests {
		t.Run("SR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			sr := mkSRWithPolicy("rt-ref", nil)
			attachRuntimeRef(&sr.Spec, tt.component)
			resp := newSRValidator(t).Handle(context.Background(), createReq(t, sr))
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(tt.component))
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring("policy refs attach on the InferenceService only"))
			}
		})
		t.Run("CSR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			csr := mkCSRWithPolicy("crt-ref", nil)
			attachRuntimeRef(&csr.Spec, tt.component)
			resp := newCSRValidator(t).Handle(context.Background(), createReq(t, csr))
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(tt.component))
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring("policy refs attach on the InferenceService only"))
			}
		})
	}
}

// The check runs before the disabled-shortcut: a disabled runtime cannot
// park an inert ref that re-enabling would silently resurface.
func TestServingRuntime_AutoscalerPolicyRefCheckedWhileDisabled(t *testing.T) {
	g := gomega.NewWithT(t)

	sr := mkSRWithPolicy("rt-ref-disabled", nil)
	sr.Spec.Disabled = proto.Bool(true)
	attachRuntimeRef(&sr.Spec, "engineConfig")
	resp := newSRValidator(t).Handle(context.Background(), createReq(t, sr))
	g.Expect(resp.Allowed).To(gomega.BeFalse(), "resp=%v", resp.Result)

	csr := mkCSRWithPolicy("crt-ref-disabled", nil)
	csr.Spec.Disabled = proto.Bool(true)
	attachRuntimeRef(&csr.Spec, "engineConfig")
	resp = newCSRValidator(t).Handle(context.Background(), createReq(t, csr))
	g.Expect(resp.Allowed).To(gomega.BeFalse(), "resp=%v", resp.Result)
}
