package snapshot

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestVolumePinned(t *testing.T) {
	cases := []struct {
		name  string
		modes []corev1.PersistentVolumeAccessMode
		want  bool
	}{
		{"rwx", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, false},
		{"rox", []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, false},
		{"rwo", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, true},
		{"rwop", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod}, true},
		{"rwo+rwx is migratable", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadWriteMany}, false},
		{"empty pins conservatively", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := volumePinned(tc.modes); got != tc.want {
				t.Fatalf("volumePinned(%v) = %v, want %v", tc.modes, got, tc.want)
			}
		})
	}
}

func TestMatchNodeSelector(t *testing.T) {
	labels := map[string]string{
		"topology.kubernetes.io/zone": "z1",
		"disktype":                    "ssd",
		"gpus":                        "8",
	}
	term := func(exprs ...corev1.NodeSelectorRequirement) *corev1.NodeSelector {
		return &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: exprs}}}
	}

	cases := []struct {
		name     string
		selector *corev1.NodeSelector
		want     bool
	}{
		{"in matches", term(corev1.NodeSelectorRequirement{
			Key: "topology.kubernetes.io/zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"z1", "z2"},
		}), true},
		{"in misses", term(corev1.NodeSelectorRequirement{
			Key: "topology.kubernetes.io/zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"z9"},
		}), false},
		{"NotIn on absent key matches", term(corev1.NodeSelectorRequirement{
			Key: "absent", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"x"},
		}), true},
		{"exists", term(corev1.NodeSelectorRequirement{Key: "disktype", Operator: corev1.NodeSelectorOpExists}), true},
		{"doesnotexist fails on present key", term(corev1.NodeSelectorRequirement{
			Key: "disktype", Operator: corev1.NodeSelectorOpDoesNotExist,
		}), false},
		{"and within term", term(
			corev1.NodeSelectorRequirement{Key: "topology.kubernetes.io/zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"z1"}},
			corev1.NodeSelectorRequirement{Key: "disktype", Operator: corev1.NodeSelectorOpIn, Values: []string{"hdd"}},
		), false},
		{"or across terms", &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "disktype", Operator: corev1.NodeSelectorOpIn, Values: []string{"hdd"}}}},
			{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "disktype", Operator: corev1.NodeSelectorOpIn, Values: []string{"ssd"}}}},
		}}, true},
		{"empty term never matches", &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{}}}, false},
		{"gt matches numerically", term(corev1.NodeSelectorRequirement{
			Key: "gpus", Operator: corev1.NodeSelectorOpGt, Values: []string{"4"},
		}), true},
		{"lt fails when value is larger", term(corev1.NodeSelectorRequirement{
			Key: "gpus", Operator: corev1.NodeSelectorOpLt, Values: []string{"4"},
		}), false},
		{"gt on unparsable value fails closed", term(corev1.NodeSelectorRequirement{
			Key: "disktype", Operator: corev1.NodeSelectorOpGt, Values: []string{"4"},
		}), false},
		{"unsupported match field fails the term", &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
			MatchFields: []corev1.NodeSelectorRequirement{{
				Key: "spec.unschedulable", Operator: corev1.NodeSelectorOpIn, Values: []string{"false"},
			}},
		}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchNodeSelector(labels, "node1", tc.selector); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}

	// metadata.name field selector.
	byName := &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
		MatchFields: []corev1.NodeSelectorRequirement{{
			Key: "metadata.name", Operator: corev1.NodeSelectorOpIn, Values: []string{"node1"},
		}},
	}}}
	if !matchNodeSelector(labels, "node1", byName) {
		t.Fatal("metadata.name In [node1] should match node1")
	}
	if matchNodeSelector(labels, "node2", byName) {
		t.Fatal("metadata.name In [node1] should not match node2")
	}
}
