package validation

import (
	"encoding/json"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestComponentAffinityValidation(t *testing.T) {
	const (
		terms    = `[{"matchFields":[{"key":"metadata.name","operator":"In","values":["cluster-a"]}]}]`
		weighted = `[{"weight":2,"matchFields":[{"key":"metadata.name","operator":"In","values":["cluster-a"]}]}]`
	)
	service := func(components, placement string) string {
		return `{"spec":{` + components + `,"placement":{` + placement + `}}}`
	}
	pd := `"engine":{},"decoder":{}`
	for _, tt := range []struct {
		name, payload, wantErr string
	}{
		{
			name:    "Split per-component weights are valid",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":`+weighted+`,"decoder":`+terms+`}`),
		},
		{
			name:    "a Split router entry needs a router minimum",
			payload: service(pd+`,"router":{}`, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":`+terms+`,"decoder":`+terms+`,"router":`+terms+`}`),
			wantErr: "set a positive router.minReplicas",
		},
		{
			name:    "a Split router entry with a minimum divides the routers",
			payload: service(pd+`,"router":{"minReplicas":2}`, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":`+terms+`,"decoder":`+terms+`,"router":`+terms+`}`),
		},
		{
			name:    "the policy requires per-component terms",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split"`),
			wantErr: "componentAffinity is required by the ComponentAffinity policy",
		},
		{
			name:    "an explicit null names no component",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":null`),
			wantErr: "must name at least one component",
		},
		{
			name:    "the shared term list is not read",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","clusterAffinity":`+terms+`,"componentAffinity":{"engine":`+terms+`,"decoder":`+terms+`}`),
			wantErr: "clusterAffinity is not read by the ComponentAffinity policy",
		},
		{
			name:    "a declared decoder needs its own terms",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":`+terms+`}`),
			wantErr: "componentAffinity.decoder is required",
		},
		{
			name:    "terms for an undeclared router are rejected",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":`+terms+`,"decoder":`+terms+`,"router":`+terms+`}`),
			wantErr: "componentAffinity.router names a component the service does not declare",
		},
		{
			name:    "weights are accepted only in Split",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Single","componentAffinity":{"engine":`+weighted+`,"decoder":`+terms+`}`),
			wantErr: "weight is permitted only in Split mode",
		},
		{
			name:    "an empty component list is invalid",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split","componentAffinity":{"engine":[],"decoder":`+terms+`}`),
			wantErr: "componentAffinity.engine: clusterAffinity must contain between 1 and",
		},
		{
			name:    "per-component terms require their policy",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"Split","componentAffinity":{"engine":`+terms+`,"decoder":`+terms+`}`),
			wantErr: "componentAffinity requires the ComponentAffinity policy",
		},
		{
			name:    "the shared term list keeps working",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+weighted),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := &v1beta1.InferenceService{}
			if err := json.Unmarshal([]byte(tt.payload), source); err != nil {
				t.Fatal(err)
			}
			err := ValidatePlacementIntent(source)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("error %v does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestComponentAffinitySelectsInSingleAndAll(t *testing.T) {
	const terms = `[{"matchFields":[{"key":"metadata.name","operator":"In","values":["cluster-a"]}]}]`
	for _, mode := range []string{"Single", "All"} {
		t.Run(mode, func(t *testing.T) {
			source := &v1beta1.InferenceService{}
			payload := `{"spec":{"engine":{},"decoder":{},"router":{},"placement":{"policy":"ComponentAffinity","mode":"` + mode + `","componentAffinity":{"engine":` + terms + `,"decoder":` + terms + `,"router":` + terms + `}}}}`
			if err := json.Unmarshal([]byte(payload), source); err != nil {
				t.Fatal(err)
			}
			if err := ValidatePlacementIntent(source); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
