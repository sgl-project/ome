// Package nodehealth implements Alfred Policy #2: reporting-only node-health
// remediation signals and migration findings for workloads observed on
// unhealthy GPU nodes.
package nodehealth

import (
	"sort"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/constants"
)

// PolicyName is the stable policy label used by metrics and reports.
const PolicyName = "nodehealth"

// Policy is Policy #2. It is a pure classifier and planner; this reporting
// phase performs no migration or node writes.
type Policy struct{}

var _ policy.Policy = &Policy{}

// Name implements policy.Policy.
func (*Policy) Name() string { return PolicyName }

// Evaluate emits one typed marker for every non-clear node and, unless
// signalOnly is set, one deduplicated finding per atomic Instance touching an
// actually unhealthy node.
func (*Policy) Evaluate(snap *snapshot.ClusterSnapshot, cfg *config.Config) []policy.Candidate {
	if snap == nil || cfg == nil || cfg.Policies.NodeHealth.Enabled == nil ||
		!*cfg.Policies.NodeHealth.Enabled {
		return nil
	}

	markers := remediationMarkers(snap)
	if cfg.Policies.NodeHealth.SignalOnly {
		return markers
	}

	findings := unhealthyFindings(snap, cfg)
	rankFindings(findings)
	return append(markers, findings...)
}

func remediationMarkers(snap *snapshot.ClusterSnapshot) []policy.Candidate {
	nodeNames := make([]string, 0, len(snap.Nodes))
	for name, node := range snap.Nodes {
		if node == nil || node.Health.State == "" || node.Health.State == snapshot.NodeHealthClear {
			continue
		}
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	markers := make([]policy.Candidate, 0, len(nodeNames))
	for _, name := range nodeNames {
		node := snap.Nodes[name]
		workloads, occupantsPresent := nodeOccupancy(node)
		markers = append(markers, policy.Candidate{
			Policy:     PolicyName,
			Reason:     policy.ReasonRemediationSignal,
			FromNode:   name,
			Executable: false,
			Remediation: &policy.NodeRemediation{
				Node:                   name,
				Health:                 copyHealth(node.Health),
				Workloads:              workloads,
				OMEGPUOccupantsPresent: occupantsPresent,
			},
		})
	}
	return markers
}

func copyHealth(in snapshot.NodeHealthObservation) snapshot.NodeHealthObservation {
	out := in
	out.Conditions = append([]snapshot.NodeConditionObservation(nil), in.Conditions...)
	if in.SuspectUntil != nil {
		until := *in.SuspectUntil
		out.SuspectUntil = &until
	}
	return out
}

func nodeOccupancy(node *snapshot.Node) ([]string, bool) {
	seen := make(map[string]struct{})
	occupantsPresent := false
	for i := range node.OMEPods {
		pod := &node.OMEPods[i]
		if pod.GPUs <= 0 {
			continue
		}
		occupantsPresent = true
		if pod.ISVC.Name == "" {
			continue
		}
		seen[pod.ISVC.String()] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, occupantsPresent
}

type findingKey struct {
	workload  string
	component string
	instance  int32
}

func unhealthyFindings(snap *snapshot.ClusterSnapshot, cfg *config.Config) []policy.Candidate {
	seen := make(map[findingKey]struct{})
	var findings []policy.Candidate
	for _, w := range sortedWorkloads(snap) {
		for _, comp := range sortedComponents(w) {
			physical := unhealthyComponentPods(snap, w, comp)
			covered := make(map[componentPodKey]struct{}, len(physical))
			instances := append([]*snapshot.Instance(nil), comp.Instances...)
			sort.Slice(instances, func(i, j int) bool {
				if instances[i] == nil {
					return instances[j] != nil
				}
				if instances[j] == nil {
					return false
				}
				return instances[i].Index < instances[j].Index
			})
			for _, inst := range instances {
				if inst == nil || inst.TotalGPUs <= 0 {
					continue
				}
				from := firstUnhealthyMember(snap, inst)
				if from == "" {
					continue
				}
				key := findingKey{workload: w.NamespacedName.String(), component: string(comp.Type), instance: inst.Index}
				if _, duplicate := seen[key]; duplicate {
					continue
				}
				seen[key] = struct{}{}
				if candidate, ok := classify(snap, cfg, w, comp, inst, from); ok {
					findings = append(findings, candidate)
					coverInstancePods(covered, physical, inst)
				}
			}
			if comp.DeploymentMode == constants.OMENative {
				if from := firstUncoveredComponentNode(physical, covered); from != "" {
					findings = append(findings, unresolvedComponentAdvisory(w, comp, from))
				}
			}
		}
	}
	return findings
}

type componentPodKey struct {
	namespace string
	name      string
	node      string
}

// unhealthyComponentPods retains the physical Pod evidence for which this
// component owes either a resolvable Instance finding or a component-wide
// fallback. The enclosing Node is authoritative for physical placement.
func unhealthyComponentPods(snap *snapshot.ClusterSnapshot, w *snapshot.Workload,
	comp *snapshot.Component) map[componentPodKey]struct{} {
	pods := make(map[componentPodKey]struct{})
	if comp.DeploymentMode != constants.OMENative {
		return pods
	}
	for nodeName, node := range snap.Nodes {
		if node == nil || node.Health.State != snapshot.NodeHealthUnhealthy {
			continue
		}
		for i := range node.OMEPods {
			pod := &node.OMEPods[i]
			if pod.GPUs > 0 && pod.ISVC == w.NamespacedName && pod.Component == comp.Type {
				pods[componentPodKey{namespace: pod.Namespace, name: pod.Name, node: nodeName}] = struct{}{}
			}
		}
	}
	return pods
}

func coverInstancePods(covered, physical map[componentPodKey]struct{}, inst *snapshot.Instance) {
	for i := range inst.Pods {
		pod := &inst.Pods[i]
		if pod.GPUs <= 0 || pod.Node == "" {
			continue
		}
		key := componentPodKey{namespace: pod.Namespace, name: pod.Name, node: pod.Node}
		if _, ok := physical[key]; ok {
			covered[key] = struct{}{}
		}
	}
}

// firstUncoveredComponentNode chooses only a source node proven by physical
// Pod evidence that no emitted Instance finding covered. It deliberately does
// not infer a migration Instance index from rejected IR/Pod identity.
func firstUncoveredComponentNode(physical, covered map[componentPodKey]struct{}) string {
	from := ""
	for pod := range physical {
		if _, ok := covered[pod]; ok {
			continue
		}
		if from == "" || pod.node < from {
			from = pod.node
		}
	}
	return from
}

// unresolvedComponentAdvisory preserves the workload/component identity
// proven by the ISVC and physical Pod evidence, but uses the component-wide
// sentinel and a zero footprint because no stable migration Instance exists.
func unresolvedComponentAdvisory(w *snapshot.Workload, comp *snapshot.Component, from string) policy.Candidate {
	return policy.Candidate{
		Policy:         PolicyName,
		Workload:       w.NamespacedName,
		Component:      comp.Type,
		Instance:       policy.ComponentWideInstance,
		Mode:           comp.DeploymentMode,
		Reason:         policy.ReasonNodeUnhealthy,
		FromNode:       from,
		AdvisoryReason: policy.AdvisoryOMENativeObservationInvalid,
		Score:          w.Priority,
	}
}

func firstUnhealthyMember(snap *snapshot.ClusterSnapshot, inst *snapshot.Instance) string {
	seen := make(map[string]struct{})
	for i := range inst.Pods {
		pod := &inst.Pods[i]
		if pod.GPUs <= 0 || pod.Node == "" {
			continue
		}
		node := snap.Nodes[pod.Node]
		if node != nil && node.Health.State == snapshot.NodeHealthUnhealthy {
			seen[pod.Node] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return ""
	}
	nodes := make([]string, 0, len(seen))
	for node := range seen {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	return nodes[0]
}

func classify(snap *snapshot.ClusterSnapshot, cfg *config.Config, w *snapshot.Workload,
	comp *snapshot.Component, inst *snapshot.Instance, from string) (policy.Candidate, bool) {

	candidate := policy.Candidate{
		Policy:        PolicyName,
		Workload:      w.NamespacedName,
		Component:     comp.Type,
		Instance:      inst.Index,
		Mode:          comp.DeploymentMode,
		Reason:        policy.ReasonNodeUnhealthy,
		FromNode:      from,
		FootprintGPUs: inst.TotalGPUs,
		Score:         w.Priority,
	}

	switch comp.DeploymentMode {
	case constants.RawDeployment:
		candidate.AdvisoryReason = policy.AdvisoryRawDeploymentMigrationUnsupported
		return candidate, true
	case constants.MultiNode:
		if cfg.LWSRecommendationsEnabled == nil || !*cfg.LWSRecommendationsEnabled {
			return policy.Candidate{}, false
		}
		candidate.AdvisoryReason = policy.AdvisoryLWSMigrationUnsupported
		return candidate, true
	case constants.OMENative:
	default:
		return policy.Candidate{}, false
	}

	if reason := policy.ModelAdvisoryReason(snap, w); reason != "" {
		candidate.AdvisoryReason = reason
		return candidate, true
	}
	if cfg.OMENativeMigrationEnabled == nil || !*cfg.OMENativeMigrationEnabled {
		candidate.AdvisoryReason = policy.AdvisoryMigrationSurfaceDisabled
		return candidate, true
	}
	if reason := policy.OMENativeEligibility(snap, w, comp, inst); reason != "" {
		candidate.AdvisoryReason = reason
		return candidate, true
	}

	plan, ok := policy.PlanAtomicSurge(snap, cfg, w, inst)
	candidate.SurgeShaped = true
	if !ok {
		candidate.AdvisoryReason = policy.AdvisoryNoSurgeHeadroom
		return candidate, true
	}
	candidate.Executable = true
	candidate.HintTargetNodes = append([]string(nil), plan.HintTargetNodes...)
	candidate.PlacementTargetNodes = append([]string(nil), plan.PlacementTargetNodes...)
	return candidate, true
}

func rankFindings(findings []policy.Candidate) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := &findings[i], &findings[j]
		if a.Executable != b.Executable {
			return a.Executable
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.FootprintGPUs != b.FootprintGPUs {
			return a.FootprintGPUs < b.FootprintGPUs
		}
		if a.Workload.String() != b.Workload.String() {
			return a.Workload.String() < b.Workload.String()
		}
		if a.Component != b.Component {
			return a.Component < b.Component
		}
		return a.Instance < b.Instance
	})
}

func sortedWorkloads(snap *snapshot.ClusterSnapshot) []*snapshot.Workload {
	out := make([]*snapshot.Workload, 0, len(snap.Workloads))
	for _, w := range snap.Workloads {
		if w != nil {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NamespacedName.String() < out[j].NamespacedName.String()
	})
	return out
}

func sortedComponents(w *snapshot.Workload) []*snapshot.Component {
	out := make([]*snapshot.Component, 0, len(w.Components))
	for _, comp := range w.Components {
		if comp != nil {
			out = append(out, comp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
