package snapshot

import (
	"fmt"
	"time"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/audit"
)

func migrationAnnotationKey(uuid string) string {
	return audit.MigrationRequestAnnotationPrefix + uuid
}

// parsePendingMigration adapts the canonical workload request into Alfred's
// immutable snapshot model. The workload parser owns schema validation;
// Alfred additionally fences the request against the source state it is about
// to reason about.
func parsePendingMigration(uuid string, raw string, workload *Workload) (InFlight, error) {
	if uuid == "" {
		return InFlight{}, fmt.Errorf("migration request UUID must not be empty")
	}
	req, err := audit.ParseMigrationRequest(raw)
	if err != nil {
		return InFlight{}, err
	}
	if req.FromNode == "" {
		return InFlight{}, fmt.Errorf("migration request from_node must not be empty")
	}
	if req.Instance < 0 {
		return InFlight{}, fmt.Errorf("migration request instance must be non-negative: %d", req.Instance)
	}

	component := v1beta1.ComponentType(req.Component)
	switch component {
	case v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent:
	default:
		return InFlight{}, fmt.Errorf("migration request component %q is unknown", req.Component)
	}

	var current bool
	if workload != nil {
		if state := workload.Components[component]; state != nil {
			for _, instance := range state.Instances {
				if instance != nil && instance.Index == req.Instance {
					current = true
					break
				}
			}
		}
	}
	if !current {
		return InFlight{}, fmt.Errorf("migration request has no current instance %s/%d", component, req.Instance)
	}

	var requestedAt time.Time
	if req.RequestedAt != "" {
		requestedAt, err = time.Parse(time.RFC3339, req.RequestedAt)
		if err != nil {
			return InFlight{}, fmt.Errorf("migration request requested_at %q is invalid: %w", req.RequestedAt, err)
		}
	}

	return InFlight{
		UUID:        uuid,
		Component:   component,
		FromNode:    req.FromNode,
		RequestedAt: requestedAt,
		RequestedBy: req.RequestedBy,
	}, nil
}
