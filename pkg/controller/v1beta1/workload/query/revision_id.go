package query

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// RevisionID is the identity of a ControllerRevision. It may be FULL
// (name + hash — from a ControllerRevision or a status name) or HASH-ONLY
// (from a pod's revision-hash label). The hash is the content identity;
// the name is for persistence, CR lookup, and display. Always build one
// via a constructor — the zero value is the "no revision" sentinel.
//
// A revision name is always "<isvc>-<component>-<hash>" and the hash is
// %08x FNV-32a (8 hex chars, hyphen-free), so hash extraction by the
// trailing segment is exact.
type RevisionID struct {
	name string
	hash string
}

// RevisionOf builds a full RevisionID from a ControllerRevision.
func RevisionOf(cr *appsv1.ControllerRevision) RevisionID {
	if cr == nil {
		return RevisionID{}
	}
	return RevisionFromName(cr.Name)
}

// RevisionFromName builds a full RevisionID from a ControllerRevision name.
func RevisionFromName(name string) RevisionID {
	if name == "" {
		return RevisionID{}
	}
	return RevisionID{name: name, hash: RevisionHashFromControllerRevisionName(name)}
}

// RevisionFromHash builds a hash-only RevisionID (e.g. from a pod label).
func RevisionFromHash(hash string) RevisionID {
	return RevisionID{hash: hash}
}

// RevisionFromPod builds a hash-only RevisionID from a pod's revision-hash
// label. Missing label → zero.
func RevisionFromPod(pod *corev1.Pod) RevisionID {
	if pod == nil {
		return RevisionID{}
	}
	return RevisionFromHash(pod.Labels[LabelRevisionHash])
}

// IsZero reports the "no revision" sentinel (no hash known).
func (r RevisionID) IsZero() bool { return r.hash == "" }

// Name returns the ControllerRevision name, or "" if hash-only.
func (r RevisionID) Name() string { return r.name }

// Hash returns the revision hash, or "" if zero.
func (r RevisionID) Hash() string { return r.hash }

// Same reports whether r and other identify the same revision. Two zero
// values are never "same". When BOTH sides carry a name, names are
// compared (fully qualified — defends against a cross-component hash
// collision); otherwise the hash is compared (the pod / hash-only case).
func (r RevisionID) Same(other RevisionID) bool {
	if r.hash == "" || other.hash == "" {
		return false
	}
	if r.name != "" && other.name != "" {
		return r.name == other.name
	}
	return r.hash == other.hash
}

// String renders the name when known, else "hash:<hash>", else "<none>".
func (r RevisionID) String() string {
	switch {
	case r.name != "":
		return r.name
	case r.hash != "":
		return "hash:" + r.hash
	default:
		return "<none>"
	}
}
