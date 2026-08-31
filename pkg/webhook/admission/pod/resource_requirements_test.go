package pod

import (
	"strings"
	"testing"
)

// Resource strings come from the inferenceservice-config ConfigMap, so a bad
// or missing value must surface as an admission error rather than a panic.
func TestContainerBuildersRejectBadResourceValues(t *testing.T) {
	tests := map[string]struct {
		cpuLimit, memoryLimit  string
		cpuRequest, memRequest string
		wantErrContains        string
	}{
		"unparsable cpu limit": {
			cpuLimit: "2 cores", memoryLimit: "1Gi", cpuRequest: "1", memRequest: "1Gi",
			wantErrContains: "cpuLimit",
		},
		"unparsable memory request": {
			cpuLimit: "1", memoryLimit: "1Gi", cpuRequest: "1", memRequest: "1 gig",
			wantErrContains: "memoryRequest",
		},
		"missing value": {
			cpuLimit: "1", memoryLimit: "", cpuRequest: "1", memRequest: "1Gi",
			wantErrContains: "memoryLimit is required",
		},
	}

	for name, tt := range tests {
		t.Run(name+"/fine-tuned adapter", func(t *testing.T) {
			_, err := (&FineTunedAdapterInjector{
				CpuLimit: tt.cpuLimit, MemoryLimit: tt.memoryLimit,
				CpuRequest: tt.cpuRequest, MemoryRequest: tt.memRequest,
			}).createInitContainer(nil, nil, nil)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErrContains)
			}
		})

		t.Run(name+"/serving sidecar", func(t *testing.T) {
			_, err := (&ServingSidecarInjector{
				CpuLimit: tt.cpuLimit, MemoryLimit: tt.memoryLimit,
				CpuRequest: tt.cpuRequest, MemoryRequest: tt.memRequest,
			}).createServingSidecarContainer(nil, nil, nil)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErrContains)
			}
		})
	}
}

func TestContainerBuildersAcceptValidResourceValues(t *testing.T) {
	c, err := (&FineTunedAdapterInjector{
		CpuLimit: "1", MemoryLimit: "1Gi", CpuRequest: "100m", MemoryRequest: "100Mi",
	}).createInitContainer(nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.Resources.Limits.Cpu().String(); got != "1" {
		t.Fatalf("cpu limit = %s, want 1", got)
	}
	if got := c.Resources.Requests.Memory().String(); got != "100Mi" {
		t.Fatalf("memory request = %s, want 100Mi", got)
	}
}
