package acceleratorquota

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/tree"
)

const rootName = "root"

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return s
}

func budget(nominal string) v1beta1.AcceleratorBudget {
	return v1beta1.AcceleratorBudget{
		ResourceName:   "google.com/tpu",
		ResourceFlavor: "tpu7x",
		Nominal:        resource.MustParse(nominal),
	}
}

func gpuBudget(nominal string) v1beta1.AcceleratorBudget {
	return v1beta1.AcceleratorBudget{
		ResourceName:   "nvidia.com/gpu",
		ResourceFlavor: "gb300",
		Nominal:        resource.MustParse(nominal),
	}
}

func cohort(name, parent string, budgets ...v1beta1.AcceleratorBudget) *v1beta1.AcceleratorQuota {
	q := &v1beta1.AcceleratorQuota{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.AcceleratorQuotaSpec{
			Role:    v1beta1.AcceleratorQuotaRoleCohort,
			Budgets: budgets,
		},
	}
	if parent != "" {
		q.Spec.ParentRef = &v1beta1.AcceleratorQuotaParentRef{Name: parent}
	}
	return q
}

func leaf(name, parent string, budgets ...v1beta1.AcceleratorBudget) *v1beta1.AcceleratorQuota {
	return &v1beta1.AcceleratorQuota{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.AcceleratorQuotaSpec{
			Role:      v1beta1.AcceleratorQuotaRoleClusterQueue,
			ParentRef: &v1beta1.AcceleratorQuotaParentRef{Name: parent},
			Budgets:   budgets,
		},
	}
}

func newValidator(t *testing.T, live ...client.Object) *Validator {
	t.Helper()
	s := testScheme(t)
	return &Validator{
		Client:  fake.NewClientBuilder().WithScheme(s).WithObjects(live...).Build(),
		Decoder: admission.NewDecoder(s),
		Options: tree.Options{RootName: rootName, MaxDepth: 5},
	}
}

func request(t *testing.T, op admissionv1.Operation, obj *v1beta1.AcceleratorQuota) admission.Request {
	t.Helper()
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: op,
		Name:      obj.Name,
	}}
	if op != admissionv1.Delete {
		raw, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req.Object.Raw = raw
	}
	return req
}

func TestValidate(t *testing.T) {
	// A tree that is already violating when the write arrives. Concurrent
	// admissions reach this state, so an admin must be able to repair it.
	brokenLive := []client.Object{
		cohort(rootName, "", budget("128")),
		cohort("org", rootName, budget("10")),
		leaf("bad", "org", budget("99")),
	}
	healthyLive := []client.Object{
		cohort(rootName, "", budget("128")),
		cohort("org", rootName, budget("100")),
		leaf("team-a", "org", budget("60")),
	}

	tests := []struct {
		name        string
		live        []client.Object
		op          admissionv1.Operation
		obj         *v1beta1.AcceleratorQuota
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "a sound addition is admitted",
			live:        healthyLive,
			op:          admissionv1.Create,
			obj:         leaf("team-b", "org", budget("40")),
			wantAllowed: true,
		},
		{
			// The violation lands on the PARENT, whose name is nowhere in the
			// request — which is why the handler diffs whole trees instead of
			// filtering violations by the object under review.
			name:        "an addition that busts its parent is denied",
			live:        healthyLive,
			op:          admissionv1.Create,
			obj:         leaf("team-b", "org", budget("80")),
			wantAllowed: false,
			wantMessage: "children total 140",
		},
		{
			name:        "a dangling parentRef is denied",
			live:        healthyLive,
			op:          admissionv1.Create,
			obj:         leaf("orphan", "ghost", budget("1")),
			wantAllowed: false,
			wantMessage: "does not resolve",
		},
		{
			// The whole reason Diff compares on (Node, Reason, Subject) rather
			// than on the violation text: the message carries a running total,
			// so a value-wise diff would read this repair as new breakage and
			// lock the admin out of fixing their own tree.
			name:        "repairing an existing overrun is admitted",
			live:        brokenLive,
			op:          admissionv1.Update,
			obj:         leaf("bad", "org", budget("50")),
			wantAllowed: true,
		},
		{
			// Same parent, same reason, DIFFERENT budget — a keyed diff must
			// still catch this even though the tree was already violating.
			name: "a new overrun on a different budget is denied",
			live: []client.Object{
				cohort(rootName, "", budget("128")),
				cohort("org", rootName, budget("10"), gpuBudget("4")),
				leaf("bad", "org", budget("99")),
			},
			op:          admissionv1.Update,
			obj:         leaf("bad", "org", budget("99"), gpuBudget("40")),
			wantAllowed: false,
			wantMessage: "nvidia.com/gpu on gb300",
		},
		{
			// Deleting a grouping orphans everything under it, and that damage
			// is invisible from the deleted object alone.
			name:        "deleting a node with children is denied",
			live:        healthyLive,
			op:          admissionv1.Delete,
			obj:         cohort("org", rootName),
			wantAllowed: false,
			wantMessage: "does not resolve",
		},
		{
			name:        "deleting a leaf is admitted",
			live:        healthyLive,
			op:          admissionv1.Delete,
			obj:         leaf("team-a", "org"),
			wantAllowed: true,
		},
		{
			// A write unrelated to the broken part of the tree must not be
			// collateral damage.
			name:        "an unrelated addition to an already-broken tree is admitted",
			live:        brokenLive,
			op:          admissionv1.Create,
			obj:         leaf("elsewhere", rootName, budget("5")),
			wantAllowed: true,
		},
		{
			name:        "an operation the webhook does not gate is allowed",
			live:        healthyLive,
			op:          admissionv1.Connect,
			obj:         leaf("team-a", "org", budget("60")),
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := newValidator(t, tc.live...)
			resp := v.Handle(context.Background(), request(t, tc.op, tc.obj))

			if resp.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v (message: %s)",
					resp.Allowed, tc.wantAllowed, responseMessage(resp))
			}
			if tc.wantMessage != "" && !strings.Contains(responseMessage(resp), tc.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", responseMessage(resp), tc.wantMessage)
			}
		})
	}
}

// A LIST failure must not be waved through. The webhook is failurePolicy=Fail,
// so allowing here would admit exactly the tree-breaking write it exists to stop,
// at the moment the apiserver is least healthy.
func TestValidateFailsClosedOnListError(t *testing.T) {
	s := testScheme(t)
	v := &Validator{
		Client: fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return apierrors.NewServiceUnavailable("apiserver down")
			},
		}).Build(),
		Decoder: admission.NewDecoder(s),
		Options: tree.Options{RootName: rootName},
	}

	resp := v.Handle(context.Background(), request(t, admissionv1.Create, leaf("x", rootName, budget("1"))))
	if resp.Allowed {
		t.Fatal("a failed LIST must not be admitted")
	}
	if got := resp.Result.Code; got != 500 {
		t.Errorf("code = %d, want 500", got)
	}
}

// An unset root name is an operator configuration failure, not a property of the
// write, so it must not be reported as if the admin's object were malformed.
func TestValidateRejectsUnconfiguredRoot(t *testing.T) {
	v := newValidator(t, cohort(rootName, "", budget("1")))
	v.Options.RootName = ""

	resp := v.Handle(context.Background(), request(t, admissionv1.Create, leaf("x", rootName, budget("1"))))
	if resp.Allowed {
		t.Fatal("an unconfigured root must not be admitted")
	}
	if !strings.Contains(responseMessage(resp), "root name is not configured") {
		t.Errorf("message = %q, want it to name the config failure", responseMessage(resp))
	}
}

func responseMessage(resp admission.Response) string {
	if resp.Result == nil {
		return ""
	}
	return resp.Result.Message
}
