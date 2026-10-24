package validation

import (
	"encoding/json"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestValidatePlacementIntent_ComponentWeights_OnlyDeclaredComponentsInSplit(t *testing.T) {
	term := func(weights string) string {
		return `[{` + weights + `"matchFields":[{"key":"metadata.name","operator":"In","values":["cluster-a"]}]}]`
	}
	service := func(components, placement string) string {
		return `{"spec":{` + components + `,"placement":{` + placement + `}}}`
	}
	pd := `"engine":{},"decoder":{}`
	for _, tt := range []struct {
		name, payload, wantErr string
	}{
		{
			name:    "Split weights beside the scalar weight are valid",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weight":2,"weights":{"decoder":1},`)),
		},
		{
			name:    "a router weight needs a router minimum",
			payload: service(pd+`,"router":{}`, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weights":{"router":1},`)),
			wantErr: "set a positive router.minReplicas",
		},
		{
			name:    "a router weight with a minimum divides the routers",
			payload: service(pd+`,"router":{"minReplicas":2}`, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weights":{"router":1},`)),
		},
		{
			name:    "a router weight without a router is rejected",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weights":{"router":1},`)),
			wantErr: "clusterAffinity[0].weights.router names a component the service does not declare",
		},
		{
			name:    "an engine weight on a decoder-only service is rejected",
			payload: service(`"decoder":{}`, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weights":{"engine":2},`)),
			wantErr: "clusterAffinity[0].weights.engine names a component the service does not declare",
		},
		{
			name:    "weights are accepted only in static Split",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"SplitByCapacity","clusterAffinity":`+term(`"weights":{"decoder":1},`)),
			wantErr: "weights is permitted only in Split mode",
		},
		{
			name:    "the scalar weight cannot sit beside default",
			payload: service(pd, `"policy":"ClusterAffinity","mode":"Split","clusterAffinity":`+term(`"weight":2,"weights":{"default":2},`)),
			wantErr: "weight means weights.default",
		},
		{
			name:    "the per-component policy is not recognized",
			payload: service(pd, `"policy":"ComponentAffinity","mode":"Split"`),
			wantErr: "spec.placement.policy must be ClusterAffinity; the ComponentAffinity policy was removed",
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
