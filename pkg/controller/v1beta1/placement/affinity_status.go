package placement

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/placement/affinity"
)

type placementInputContextKey struct{}

type placementInputSnapshot struct {
	source     types.NamespacedName
	uid        types.UID
	generation int64
	condition  policyCondition
}

// Match diagnostics describe registry membership, independently of member
// health and admission. An unmatched OR term does not invalidate the intent;
// components that match different clusters do.
func withPlacementInputCondition(ctx context.Context, source *v1beta1.InferenceService, clusters []v1beta1.WorkloadCluster) context.Context {
	condition := placementInputCondition(source)
	if condition.cond.Status == corev1.ConditionTrue && source.Spec.Placement.UsesAffinityPolicy() {
		if err := componentClustersDiffer(source, clusters); err != nil {
			condition.cond.Status = corev1.ConditionFalse
			condition.cond.Reason = "ComponentClustersDiffer"
			condition.cond.Message = err.Error()
		} else if message, err := unmatchedTermsMessage(source, clusters); err != nil {
			condition.cond.Status = corev1.ConditionFalse
			condition.cond.Reason = "InvalidPlacementIntent"
			condition.cond.Message = err.Error()
		} else {
			condition.cond.Message += message
		}
	}
	return context.WithValue(ctx, placementInputContextKey{}, placementInputSnapshot{
		source: client.ObjectKeyFromObject(source), uid: source.UID, generation: source.Generation, condition: condition,
	})
}

// unmatchedTermsMessage names the terms of each affinity list that match no
// registered cluster.
func unmatchedTermsMessage(source *v1beta1.InferenceService, clusters []v1beta1.WorkloadCluster) (string, error) {
	p := source.Spec.Placement
	type list struct {
		name  string
		terms []v1beta1.ClusterAffinityTerm
	}
	var lists []list
	if p.UsesComponentAffinity() {
		for _, component := range placedComponents(source) {
			if terms := componentTerms(p.ComponentAffinity, component); len(terms) > 0 {
				lists = append(lists, list{name: "componentAffinity." + string(component), terms: terms})
			}
		}
	} else if len(p.ClusterAffinity) > 0 {
		lists = append(lists, list{name: "clusterAffinity", terms: p.ClusterAffinity})
	}
	var message string
	for _, l := range lists {
		selector, err := affinity.Compile(l.terms, p.Mode == v1beta1.PlacementModeSplit)
		if err != nil {
			return "", fmt.Errorf("spec.placement.%s: %w", l.name, err)
		}
		matched := make([]bool, len(l.terms))
		for i := range clusters {
			if !clusters[i].DeletionTimestamp.IsZero() {
				continue
			}
			match, _ := selector.Match(&clusters[i])
			for _, index := range match.TermIndexes {
				matched[index] = true
			}
		}
		var unmatched []int
		for index, found := range matched {
			if !found {
				unmatched = append(unmatched, index)
			}
		}
		if len(unmatched) > 0 {
			message += fmt.Sprintf("; %s terms %v match no registered WorkloadCluster (zero-based indexes)", l.name, unmatched)
		}
	}
	return message, nil
}

func placementInputForWrite(ctx context.Context, source *v1beta1.InferenceService) policyCondition {
	input, ok := ctx.Value(placementInputContextKey{}).(placementInputSnapshot)
	if ok && input.source == client.ObjectKeyFromObject(source) && input.uid == source.UID && input.generation == source.Generation {
		return input.condition
	}
	return placementInputCondition(source)
}
