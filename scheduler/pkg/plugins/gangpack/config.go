package gangpack

import (
	"fmt"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

// Args configures behavior that cannot be inferred from an individual gang.
// TopologyKey is the fallback for PodGroups without a per-gang topology
// annotation.
type Args struct {
	TopologyKey                    string `json:"topologyKey,omitempty"`
	PodGroupTopologyKeyAnnotation  string `json:"podGroupTopologyKeyAnnotation,omitempty"`
	UnsupportedPlacementGroupLabel string `json:"unsupportedPlacementGroupLabel,omitempty"`
	DefaultPermitTimeoutSeconds    *int64 `json:"defaultPermitTimeoutSeconds,omitempty"`
	PodGroupSyncTimeoutSeconds     *int64 `json:"podGroupSyncTimeoutSeconds,omitempty"`
	GCIntervalSeconds              *int64 `json:"gcIntervalSeconds,omitempty"`
	// StandaloneDomainPacking enables the domain-level bin-packing Score for
	// unpinned (non-gang) whole-node pods. Defaults to true when unset, and only
	// takes effect when a pool-wide TopologyKey is configured.
	StandaloneDomainPacking *bool `json:"standaloneDomainPacking,omitempty"`
}

// standaloneDomainPackingEnabled reports the effective toggle: on unless the
// operator explicitly disables it.
func (a Args) standaloneDomainPackingEnabled() bool {
	return a.StandaloneDomainPacking == nil || *a.StandaloneDomainPacking
}

func decodeArgs(obj runtime.Object) (Args, error) {
	var args Args
	if err := frameworkruntime.DecodeInto(obj, &args); err != nil {
		return Args{}, fmt.Errorf("decode %s args: %w", Name, err)
	}
	if args.TopologyKey != "" {
		if errs := validation.IsQualifiedName(args.TopologyKey); len(errs) > 0 {
			return Args{}, fmt.Errorf("topologyKey %q is not a valid node label key: %s", args.TopologyKey, errs[0])
		}
	}
	for name, value := range map[string]string{
		"podGroupTopologyKeyAnnotation":  args.PodGroupTopologyKeyAnnotation,
		"unsupportedPlacementGroupLabel": args.UnsupportedPlacementGroupLabel,
	} {
		if value == "" {
			continue
		}
		if errs := validation.IsQualifiedName(value); len(errs) > 0 {
			return Args{}, fmt.Errorf("%s %q is not a valid metadata key: %s", name, value, errs[0])
		}
	}
	if args.DefaultPermitTimeoutSeconds != nil {
		seconds := *args.DefaultPermitTimeoutSeconds
		if seconds <= 0 {
			return Args{}, fmt.Errorf("defaultPermitTimeoutSeconds must be positive")
		}
		if seconds > math.MaxInt64/int64(time.Second) {
			return Args{}, fmt.Errorf("defaultPermitTimeoutSeconds is too large")
		}
	}
	if args.PodGroupSyncTimeoutSeconds != nil {
		seconds := *args.PodGroupSyncTimeoutSeconds
		if seconds <= 0 {
			return Args{}, fmt.Errorf("podGroupSyncTimeoutSeconds must be positive")
		}
		if seconds > math.MaxInt64/int64(time.Second) {
			return Args{}, fmt.Errorf("podGroupSyncTimeoutSeconds is too large")
		}
	}
	if args.GCIntervalSeconds == nil {
		return Args{}, fmt.Errorf("gcIntervalSeconds must be configured")
	}
	seconds := *args.GCIntervalSeconds
	if seconds <= 0 {
		return Args{}, fmt.Errorf("gcIntervalSeconds must be positive")
	}
	if seconds > math.MaxInt64/int64(time.Second) {
		return Args{}, fmt.Errorf("gcIntervalSeconds is too large")
	}
	return args, nil
}

func (a Args) podGroupSyncTimeout() time.Duration {
	if a.PodGroupSyncTimeoutSeconds == nil {
		return 0
	}
	return time.Duration(*a.PodGroupSyncTimeoutSeconds) * time.Second
}

func (a Args) gcInterval() time.Duration {
	if a.GCIntervalSeconds == nil {
		return 0
	}
	return time.Duration(*a.GCIntervalSeconds) * time.Second
}

func (a Args) defaultPermitTimeout() time.Duration {
	if a.DefaultPermitTimeoutSeconds == nil {
		return 0
	}
	return time.Duration(*a.DefaultPermitTimeoutSeconds) * time.Second
}
