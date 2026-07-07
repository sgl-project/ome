package pernode

import (
	"testing"

	"github.com/onsi/gomega"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestCalculateLifecycleState(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name          string
		nodesReady    []string
		nodesFailed   []string
		expectedState v1beta1.LifeCycleState
	}{
		{
			name:          "Ready state with ready nodes",
			nodesReady:    []string{"node1", "node2"},
			nodesFailed:   []string{},
			expectedState: v1beta1.LifeCycleStateReady,
		},
		{
			name:          "Ready state with mixed nodes",
			nodesReady:    []string{"node1"},
			nodesFailed:   []string{"node2"},
			expectedState: v1beta1.LifeCycleStateReady,
		},
		{
			name:          "Failed state with only failed nodes",
			nodesReady:    []string{},
			nodesFailed:   []string{"node1", "node2"},
			expectedState: v1beta1.LifeCycleStateFailed,
		},
		{
			name:          "InTransit state with no nodes",
			nodesReady:    []string{},
			nodesFailed:   []string{},
			expectedState: v1beta1.LifeCycleStateInTransit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := CalculateLifecycleState(tt.nodesReady, tt.nodesFailed)
			g.Expect(state).To(gomega.Equal(tt.expectedState))
		})
	}
}

func TestAddToSlice(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name     string
		slice    []string
		item     string
		expected []string
	}{
		{
			name:     "Add to empty slice",
			slice:    []string{},
			item:     "item1",
			expected: []string{"item1"},
		},
		{
			name:     "Add new item",
			slice:    []string{"item1", "item2"},
			item:     "item3",
			expected: []string{"item1", "item2", "item3"},
		},
		{
			name:     "Don't add existing item",
			slice:    []string{"item1", "item2"},
			item:     "item1",
			expected: []string{"item1", "item2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addToSlice(tt.slice, tt.item)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}
