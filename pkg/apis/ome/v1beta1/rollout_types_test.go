package v1beta1

import "testing"

func TestGetRolloutGroups(t *testing.T) {
	var nilSpec *InferenceServiceSpec
	if nilSpec.GetRolloutGroups() != nil {
		t.Fatal("nil spec must return nil groups")
	}
	s := &InferenceServiceSpec{}
	if s.GetRolloutGroups() != nil {
		t.Fatal("no rollout must return nil groups")
	}
	s.Rollout = &RolloutSpec{Groups: []RolloutGroup{{Components: []ComponentType{EngineComponent}}}}
	if len(s.GetRolloutGroups()) != 1 {
		t.Fatal("set groups must be returned")
	}
}
