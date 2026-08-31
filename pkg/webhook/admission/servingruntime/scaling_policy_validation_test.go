package servingruntime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/validation"
)

func runtimeProportionalPolicy(decoderRatio string) *v1beta1.ScalingPolicy {
	return &v1beta1.ScalingPolicy{
		Mode: v1beta1.ScalingProportional,
		Proportional: &v1beta1.ProportionalPolicy{
			Anchor: v1beta1.EngineComponent,
			Ratios: map[v1beta1.ComponentType]resource.Quantity{
				v1beta1.DecoderComponent: resource.MustParse(decoderRatio),
			},
		},
	}
}

// mkRuntimeSpec builds the minimal valid runtime spec shared by the SR and
// CSR scaling-policy tests.
func mkRuntimeSpec(policy *v1beta1.ScalingPolicy) v1beta1.ServingRuntimeSpec {
	return v1beta1.ServingRuntimeSpec{
		SupportedModelFormats: []v1beta1.SupportedModelFormat{
			{
				Name:       "safetensors",
				Version:    proto.String("1"),
				AutoSelect: proto.Bool(true),
				Priority:   proto.Int32(1),
			},
		},
		Disabled: proto.Bool(false),
		ProtocolVersions: []constants.InferenceServiceProtocol{
			constants.OpenAIProtocol,
		},
		ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
			Containers: []corev1.Container{
				{
					Name:  constants.MainContainerName,
					Image: "ome/vllm:latest",
					Args:  []string{"--model_name={{.Name}}"},
				},
			},
		},
		ScalingPolicy: policy,
	}
}

func mkSRWithPolicy(name string, policy *v1beta1.ScalingPolicy) *v1beta1.ServingRuntime {
	return &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       mkRuntimeSpec(policy),
	}
}

func mkCSRWithPolicy(name string, policy *v1beta1.ScalingPolicy) *v1beta1.ClusterServingRuntime {
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       mkRuntimeSpec(policy),
	}
}

func encodeObj(t *testing.T, obj any) []byte {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func createReq(t *testing.T, obj any) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: encodeObj(t, obj)},
	}}
}

func updateReq(t *testing.T, oldObj, newObj any) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Object:    runtime.RawExtension{Raw: encodeObj(t, newObj)},
		OldObject: runtime.RawExtension{Raw: encodeObj(t, oldObj)},
	}}
}

func newSRValidator(t *testing.T) *ServingRuntimeValidator {
	t.Helper()
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	return &ServingRuntimeValidator{Client: c, Decoder: newCSRDecoder(t)}
}

func newCSRValidator(t *testing.T) *ClusterServingRuntimeValidator {
	t.Helper()
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	return &ClusterServingRuntimeValidator{Client: c, Decoder: newCSRDecoder(t)}
}

func TestServingRuntime_ScalingPolicyCreate(t *testing.T) {
	tests := []struct {
		name        string
		policy      *v1beta1.ScalingPolicy
		wantAllowed bool
	}{
		{name: "nil policy allowed", policy: nil, wantAllowed: true},
		{name: "Independent allowed", policy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, wantAllowed: true},
		{name: "Proportional denied", policy: runtimeProportionalPolicy("1")},
		{name: "Pinned denied", policy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}},
	}
	for _, tt := range tests {
		t.Run("SR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			resp := newSRValidator(t).Handle(context.Background(), createReq(t, mkSRWithPolicy("rt-sp", tt.policy)))
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))
			}
		})
		t.Run("CSR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			resp := newCSRValidator(t).Handle(context.Background(), createReq(t, mkCSRWithPolicy("crt-sp", tt.policy)))
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))
			}
		})
	}
}

func TestServingRuntime_ScalingPolicyUpdate(t *testing.T) {
	tests := []struct {
		name        string
		oldPolicy   *v1beta1.ScalingPolicy
		newPolicy   *v1beta1.ScalingPolicy
		wantAllowed bool
	}{
		{name: "nil to nil allowed", wantAllowed: true},
		{name: "nil to Independent allowed", newPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, wantAllowed: true},
		{name: "newly set Proportional denied", newPolicy: runtimeProportionalPolicy("1")},
		{name: "newly set Pinned denied", newPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}},
		{name: "Independent to Proportional denied", oldPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, newPolicy: runtimeProportionalPolicy("1")},
		{name: "unchanged stored Proportional ratchets through", oldPolicy: runtimeProportionalPolicy("1"), newPolicy: runtimeProportionalPolicy("1"), wantAllowed: true},
		{name: "unchanged stored Pinned ratchets through", oldPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, newPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, wantAllowed: true},
		{name: "changed Proportional ratio denied", oldPolicy: runtimeProportionalPolicy("1"), newPolicy: runtimeProportionalPolicy("2")},
		{name: "Proportional to Independent allowed", oldPolicy: runtimeProportionalPolicy("1"), newPolicy: &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, wantAllowed: true},
		{name: "Proportional removed allowed", oldPolicy: runtimeProportionalPolicy("1"), newPolicy: nil, wantAllowed: true},
	}
	for _, tt := range tests {
		t.Run("SR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			req := updateReq(t, mkSRWithPolicy("rt-sp", tt.oldPolicy), mkSRWithPolicy("rt-sp", tt.newPolicy))
			resp := newSRValidator(t).Handle(context.Background(), req)
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))
			}
		})
		t.Run("CSR "+tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			req := updateReq(t, mkCSRWithPolicy("crt-sp", tt.oldPolicy), mkCSRWithPolicy("crt-sp", tt.newPolicy))
			resp := newCSRValidator(t).Handle(context.Background(), req)
			g.Expect(resp.Allowed).To(gomega.Equal(tt.wantAllowed), "resp=%v", resp.Result)
			if !tt.wantAllowed {
				g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))
			}
		})
	}
}

// TestServingRuntime_ScalingPolicyCheckedWhileDisabled pins that the check
// runs before the disabled-shortcut: a disabled runtime cannot store a
// rejected mode that the update ratchet would later treat as pre-existing.
func TestServingRuntime_ScalingPolicyCheckedWhileDisabled(t *testing.T) {
	g := gomega.NewWithT(t)
	sr := mkSRWithPolicy("rt-disabled", runtimeProportionalPolicy("1"))
	sr.Spec.Disabled = proto.Bool(true)
	resp := newSRValidator(t).Handle(context.Background(), createReq(t, sr))
	g.Expect(resp.Allowed).To(gomega.BeFalse(), "resp=%v", resp.Result)
	g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))

	csr := mkCSRWithPolicy("crt-disabled", runtimeProportionalPolicy("1"))
	csr.Spec.Disabled = proto.Bool(true)
	resp = newCSRValidator(t).Handle(context.Background(), createReq(t, csr))
	g.Expect(resp.Allowed).To(gomega.BeFalse(), "resp=%v", resp.Result)
	g.Expect(resp.Result.Message).To(gomega.ContainSubstring(validation.ReasonScalingModeNotImplemented))
}

// TestServingRuntime_ScalingPolicyRatchetAllowsUnrelatedUpdate pins the
// ratchet's purpose on the runtime path: a stored runtime carrying a rejected
// mode still accepts writes that do not touch the policy.
func TestServingRuntime_ScalingPolicyRatchetAllowsUnrelatedUpdate(t *testing.T) {
	g := gomega.NewWithT(t)
	oldSR := mkSRWithPolicy("rt-ratchet", runtimeProportionalPolicy("1"))
	newSR := mkSRWithPolicy("rt-ratchet", runtimeProportionalPolicy("1"))
	newSR.Spec.ServingRuntimePodSpec.Containers[0].Image = "ome/vllm:newer"
	resp := newSRValidator(t).Handle(context.Background(), updateReq(t, oldSR, newSR))
	g.Expect(resp.Allowed).To(gomega.BeTrue(), "resp=%v", resp.Result)
}
