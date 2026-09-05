package gangpack

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/kube-scheduler/framework"
)

// siblingAffinityBlocked returns the unplaced gang sibling that one of pod's
// required podAffinity terms can only be satisfied by. Until that sibling is
// assumed on a node the term rejects every node, so the member should wait
// rather than plan a domain. A term that matches pod itself is left to the
// framework (a first pod may satisfy its own term), a term already matched by
// a placed member of the gang imposes no wait, and terms that target pods
// outside the gang are not considered.
func siblingAffinityBlocked(pod *v1.Pod, unplaced []*v1.Pod, placed map[string]*v1.Pod) (*v1.Pod, bool) {
	terms, err := framework.GetAffinityTerms(pod, framework.GetPodAffinityTerms(pod.Spec.Affinity))
	if err != nil || len(terms) == 0 {
		return nil, false
	}
	for i := range terms {
		term := &terms[i]
		if term.Matches(pod, nil) || anyMemberMatches(term, placed) {
			continue
		}
		for _, sibling := range unplaced {
			if sibling != nil && sibling != pod && !samePodIdentity(sibling, pod) && term.Matches(sibling, nil) {
				return sibling, true
			}
		}
	}
	return nil, false
}

func anyMemberMatches(term *framework.AffinityTerm, members map[string]*v1.Pod) bool {
	for _, member := range members {
		if member != nil && term.Matches(member, nil) {
			return true
		}
	}
	return false
}
