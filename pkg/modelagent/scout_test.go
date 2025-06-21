package modelagent

import (
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the shouldDownloadModel function
func TestShouldDownloadModel(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				constants.NodeInstanceShapeLabel:   "GPU.A10.2",
				"node.kubernetes.io/instance-type": "GPU.A10.2",
				"accelerator":                      "nvidia",
				"gpu-model":                        "a10",
				"region":                           "us-west-2",
			},
			Annotations: map[string]string{
				constants.TargetInstanceShapes: "GPU.A10.2,GPU.A100.8",
			},
		},
	}

	// Create a test scount with minimal fields needed for the test
	scount := &Scount{
		nodeName:       "test-node",
		nodeInfo:       testNode,
		nodeShapeAlias: "a10",
		logger:         sugaredLogger,
	}

	// Define test cases
	testCases := []struct {
		name           string
		storageSpec    *v1beta1.StorageSpec
		expectedResult bool
		description    string
	}{
		{
			name:           "nil storage spec",
			storageSpec:    nil,
			expectedResult: true,
			description:    "Nil StorageSpec should default to true",
		},
		{
			name:           "empty storage spec",
			storageSpec:    &v1beta1.StorageSpec{},
			expectedResult: true,
			description:    "Empty StorageSpec should default to true",
		},
		{
			name: "matching node selector",
			storageSpec: &v1beta1.StorageSpec{
				NodeSelector: map[string]string{
					"gpu-model": "a10",
				},
			},
			expectedResult: true,
			description:    "NodeSelector matches node labels",
		},
		{
			name: "non-matching node selector",
			storageSpec: &v1beta1.StorageSpec{
				NodeSelector: map[string]string{
					"gpu-model": "a100",
				},
			},
			expectedResult: false,
			description:    "NodeSelector doesn't match node labels",
		},
		{
			name: "matching required node affinity - In operator",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "gpu-model",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"a10", "a100"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			description:    "NodeAffinity with In operator matching node labels",
		},
		{
			name: "non-matching required node affinity - In operator",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "gpu-model",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"a100", "h100"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			description:    "NodeAffinity with In operator not matching node labels",
		},
		{
			name: "matching required node affinity - Exists operator",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "accelerator",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			description:    "NodeAffinity with Exists operator matching node labels",
		},
		{
			name: "non-matching required node affinity - Exists operator",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "non-existent-label",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			description:    "NodeAffinity with Exists operator not matching node labels",
		},
		{
			name: "matching multiple node selector terms - OR relationship",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "non-existent-label",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "gpu-model",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"a10"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			description:    "One of multiple NodeSelectorTerms matches (OR relationship)",
		},
		{
			name: "matching multiple match expressions - AND relationship",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "gpu-model",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"a10"},
									},
									{
										Key:      "accelerator",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			description:    "Multiple MatchExpressions in one term all match (AND relationship)",
		},
		{
			name: "non-matching multiple match expressions - AND relationship",
			storageSpec: &v1beta1.StorageSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "gpu-model",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"a10"},
									},
									{
										Key:      "non-existent-label",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			description:    "One of multiple MatchExpressions doesn't match (AND relationship)",
		},
		{
			name: "node selector + node affinity - both match",
			storageSpec: &v1beta1.StorageSpec{
				NodeSelector: map[string]string{
					"gpu-model": "a10",
				},
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "accelerator",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			description:    "Both NodeSelector and NodeAffinity match",
		},
		{
			name: "node selector + node affinity - one doesn't match",
			storageSpec: &v1beta1.StorageSpec{
				NodeSelector: map[string]string{
					"gpu-model": "a100", // This doesn't match
				},
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "accelerator",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			description:    "NodeSelector doesn't match but NodeAffinity does",
		},
		{
			name: "fallback to annotation - matching",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("oci://foo/bar"),
			},
			expectedResult: true,
			description:    "Empty node selectors should fallback to annotation check (which matches)",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scount.shouldDownloadModel(tc.storageSpec)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}

// Helper function to return a string pointer
func stringPtr(s string) *string {
	return &s
}

// Test the nodeMatchesExpression function
func TestNodeMatchesExpression(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"gpu-model": "a10",
				"region":    "us-west-2",
				"memory":    "32Gi",
				"cpu":       "8",
			},
		},
	}

	// Create a test scount with minimal fields needed
	scount := &Scount{
		nodeName: "test-node",
		nodeInfo: testNode,
		logger:   sugaredLogger,
	}

	// Define test cases
	testCases := []struct {
		name           string
		expr           corev1.NodeSelectorRequirement
		expectedResult bool
		description    string
	}{
		{
			name: "In operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"a10", "a100"},
			},
			expectedResult: true,
			description:    "In operator should match when label value is in the values list",
		},
		{
			name: "In operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"a100", "h100"},
			},
			expectedResult: false,
			description:    "In operator should not match when label value is not in the values list",
		},
		{
			name: "NotIn operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"a100", "h100"},
			},
			expectedResult: true,
			description:    "NotIn operator should match when label value is not in the values list",
		},
		{
			name: "NotIn operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"a10", "h100"},
			},
			expectedResult: false,
			description:    "NotIn operator should not match when label value is in the values list",
		},
		{
			name: "Exists operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpExists,
			},
			expectedResult: true,
			description:    "Exists operator should match when label exists",
		},
		{
			name: "Exists operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "non-existent-label",
				Operator: corev1.NodeSelectorOpExists,
			},
			expectedResult: false,
			description:    "Exists operator should not match when label doesn't exist",
		},
		{
			name: "DoesNotExist operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "non-existent-label",
				Operator: corev1.NodeSelectorOpDoesNotExist,
			},
			expectedResult: true,
			description:    "DoesNotExist operator should match when label doesn't exist",
		},
		{
			name: "DoesNotExist operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "gpu-model",
				Operator: corev1.NodeSelectorOpDoesNotExist,
			},
			expectedResult: false,
			description:    "DoesNotExist operator should not match when label exists",
		},
		{
			name: "Gt operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpGt,
				Values:   []string{"4"},
			},
			expectedResult: true,
			description:    "Gt operator should match when label value is greater than the specified value",
		},
		{
			name: "Gt operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpGt,
				Values:   []string{"16"},
			},
			expectedResult: false,
			description:    "Gt operator should not match when label value is not greater than the specified value",
		},
		{
			name: "Lt operator - matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpLt,
				Values:   []string{"16"},
			},
			expectedResult: true,
			description:    "Lt operator should match when label value is less than the specified value",
		},
		{
			name: "Lt operator - non-matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpLt,
				Values:   []string{"4"},
			},
			expectedResult: false,
			description:    "Lt operator should not match when label value is not less than the specified value",
		},
		{
			name: "Gt operator - empty label value",
			expr: corev1.NodeSelectorRequirement{
				Key:      "non-existent-label",
				Operator: corev1.NodeSelectorOpGt,
				Values:   []string{"4"},
			},
			expectedResult: false,
			description:    "Gt operator should not match when label doesn't exist",
		},
		{
			name: "Lt operator - empty label value",
			expr: corev1.NodeSelectorRequirement{
				Key:      "non-existent-label",
				Operator: corev1.NodeSelectorOpLt,
				Values:   []string{"4"},
			},
			expectedResult: false,
			description:    "Lt operator should not match when label doesn't exist",
		},
		{
			name: "Gt operator - empty values",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpGt,
				Values:   []string{},
			},
			expectedResult: false,
			description:    "Gt operator should not match when values are empty",
		},
		{
			name: "Lt operator - empty values",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: corev1.NodeSelectorOpLt,
				Values:   []string{},
			},
			expectedResult: false,
			description:    "Lt operator should not match when values are empty",
		},
		{
			name: "Unknown operator",
			expr: corev1.NodeSelectorRequirement{
				Key:      "cpu",
				Operator: "UnknownOperator",
				Values:   []string{"8"},
			},
			expectedResult: false,
			description:    "Unknown operator should default to false",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scount.nodeMatchesExpression(tc.expr)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}

// Test the nodeMatchesSelectorTerm function
func TestNodeMatchesSelectorTerm(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"gpu-model":   "a10",
				"region":      "us-west-2",
				"accelerator": "nvidia",
			},
		},
	}

	// Create a test scount with minimal fields needed
	scount := &Scount{
		nodeName: "test-node",
		nodeInfo: testNode,
		logger:   sugaredLogger,
	}

	// Define test cases
	testCases := []struct {
		name           string
		term           corev1.NodeSelectorTerm
		expectedResult bool
		description    string
	}{
		{
			name: "single match expression - matching",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a10"},
					},
				},
			},
			expectedResult: true,
			description:    "Should match when single match expression matches",
		},
		{
			name: "single match expression - non-matching",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a100"},
					},
				},
			},
			expectedResult: false,
			description:    "Should not match when single match expression doesn't match",
		},
		{
			name: "multiple match expressions - all matching (AND)",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a10"},
					},
					{
						Key:      "accelerator",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			expectedResult: true,
			description:    "Should match when all match expressions match (AND relationship)",
		},
		{
			name: "multiple match expressions - one non-matching (AND)",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a10"},
					},
					{
						Key:      "non-existent-label",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			expectedResult: false,
			description:    "Should not match when any match expression doesn't match (AND relationship)",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scount.nodeMatchesSelectorTerm(tc.term)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}

// Test the NodeMatchesSelectorTerm function with MatchFields
func TestNodeMatchesSelectorTermWithMatchFields(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"gpu-model":   "a10",
				"accelerator": "nvidia",
			},
		},
	}

	// Create a test scount with minimal fields needed
	scount := &Scount{
		nodeName: "test-node",
		nodeInfo: testNode,
		logger:   sugaredLogger,
	}

	// Define test cases
	testCases := []struct {
		name           string
		term           corev1.NodeSelectorTerm
		expectedResult bool
		description    string
	}{
		{
			name: "match field for metadata.name - matching",
			term: corev1.NodeSelectorTerm{
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-node"},
					},
				},
			},
			expectedResult: true,
			description:    "Should match when field metadata.name matches",
		},
		{
			name: "match field for metadata.name - non-matching",
			term: corev1.NodeSelectorTerm{
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"other-node"},
					},
				},
			},
			expectedResult: false,
			description:    "Should not match when field metadata.name doesn't match",
		},
		{
			name: "match expressions and match fields - both match",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a10"},
					},
				},
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-node"},
					},
				},
			},
			expectedResult: true,
			description:    "Should match when both match expressions and match fields match",
		},
		{
			name: "match expressions and match fields - match expressions don't match",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a100"},
					},
				},
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-node"},
					},
				},
			},
			expectedResult: false,
			description:    "Should not match when match expressions don't match even if match fields match",
		},
		{
			name: "match expressions and match fields - match fields don't match",
			term: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "gpu-model",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"a10"},
					},
				},
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"other-node"},
					},
				},
			},
			expectedResult: false,
			description:    "Should not match when match fields don't match even if match expressions match",
		},
		{
			name: "match field with NotIn operator - matching",
			term: corev1.NodeSelectorTerm{
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"other-node"},
					},
				},
			},
			expectedResult: true,
			description:    "Should match when field metadata.name is not in the values",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scount.nodeMatchesSelectorTerm(tc.term)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}

// Test complex fallback scenarios for the shouldDownloadModel function
func TestShouldDownloadModelFallback(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Test cases for different annotation scenarios
	testCases := []struct {
		name            string
		nodeAnnotations map[string]string
		nodeShape       string
		storageSpec     *v1beta1.StorageSpec
		expectedResult  bool
		description     string
	}{
		{
			name:            "no annotations, empty storage spec",
			nodeAnnotations: map[string]string{},
			nodeShape:       "GPU.A10.2",
			storageSpec:     &v1beta1.StorageSpec{},
			expectedResult:  true,
			description:     "Should default to true when no annotations and empty storage spec",
		},
		{
			name: "shape in target shapes annotation",
			nodeAnnotations: map[string]string{
				constants.TargetInstanceShapes: "GPU.A10.2,GPU.A100.8",
			},
			nodeShape:      "GPU.A10.2",
			storageSpec:    &v1beta1.StorageSpec{},
			expectedResult: true,
			description:    "Should match when node shape is in target shapes annotation",
		},
		{
			name: "empty target shapes annotation",
			nodeAnnotations: map[string]string{
				constants.TargetInstanceShapes: "",
			},
			nodeShape:      "GPU.A10.2",
			storageSpec:    &v1beta1.StorageSpec{},
			expectedResult: true,
			description:    "Should default to true when target shapes annotation is empty",
		},
		{
			name: "annotation fallback not used with node selector",
			nodeAnnotations: map[string]string{
				constants.TargetInstanceShapes: "GPU.A100.8,GPU.H100.8", // Would not match
			},
			nodeShape: "GPU.A10.2",
			storageSpec: &v1beta1.StorageSpec{
				NodeSelector: map[string]string{
					"gpu-model": "a10", // This matches
				},
			},
			expectedResult: true,
			description:    "Should use node selector and not fallback to annotation",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test node with specified annotations
			testNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						constants.NodeInstanceShapeLabel: tc.nodeShape,
						"gpu-model":                      "a10",
					},
					Annotations: tc.nodeAnnotations,
				},
			}

			// Create a test scount with the node
			scount := &Scount{
				nodeName: "test-node",
				nodeInfo: testNode,
				logger:   sugaredLogger,
			}

			result := scount.shouldDownloadModel(tc.storageSpec)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}

// Test edge cases for the nodeMatchesExpression function
func TestNodeMatchesExpressionEdgeCases(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"empty-value":     "",
				"null-like-value": "null",
				"boolean-true":    "true",
				"boolean-false":   "false",
			},
		},
	}

	// Create a test scount with minimal fields needed
	scount := &Scount{
		nodeName: "test-node",
		nodeInfo: testNode,
		logger:   sugaredLogger,
	}

	// Define test cases
	testCases := []struct {
		name           string
		expr           corev1.NodeSelectorRequirement
		expectedResult bool
		description    string
	}{
		{
			name: "empty value label with In operator",
			expr: corev1.NodeSelectorRequirement{
				Key:      "empty-value",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{""},
			},
			expectedResult: true,
			description:    "In operator should match when label has empty value and empty value is in values list",
		},
		{
			name: "empty value label with NotIn operator",
			expr: corev1.NodeSelectorRequirement{
				Key:      "empty-value",
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"some-value"},
			},
			expectedResult: true,
			description:    "NotIn operator should match when label has empty value and empty value is not in values list",
		},
		{
			name: "null-like value with In operator",
			expr: corev1.NodeSelectorRequirement{
				Key:      "null-like-value",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"null"},
			},
			expectedResult: true,
			description:    "In operator should match when label has 'null' string value and 'null' is in values list",
		},
		{
			name: "boolean value matching",
			expr: corev1.NodeSelectorRequirement{
				Key:      "boolean-true",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"true"},
			},
			expectedResult: true,
			description:    "In operator should match when label has boolean-like string value",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scount.nodeMatchesExpression(tc.expr)
			if result != tc.expectedResult {
				t.Errorf("%s: Expected %v but got %v. %s", tc.name, tc.expectedResult, result, tc.description)
			}
		})
	}
}
