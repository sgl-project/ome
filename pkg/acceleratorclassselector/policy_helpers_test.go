package acceleratorclassselector

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Helper function to create int64 pointer
func int64Ptr(v int64) *int64 {
	return &v
}

// Helper function to create string pointer
func stringPtr(v string) *string {
	return &v
}

// TestScoreFromTFLOPS tests the scoreFromTFLOPS helper function
func TestScoreFromTFLOPS(t *testing.T) {
	tests := []struct {
		name           string
		tflops         int64
		requiredTFLOPS int64
		expectedScore  float64
	}{
		{
			name:           "Zero TFLOPS",
			tflops:         0,
			requiredTFLOPS: 100,
			expectedScore:  0.0,
		},
		{
			name:           "No requirement (zero required)",
			tflops:         100,
			requiredTFLOPS: 0,
			expectedScore:  1.0,
		},
		{
			name:           "Exact match",
			tflops:         100,
			requiredTFLOPS: 100,
			expectedScore:  1.0,
		},
		{
			name:           "Exceeds requirement",
			tflops:         200,
			requiredTFLOPS: 100,
			expectedScore:  1.0, // Capped at 1.0
		},
		{
			name:           "Half of requirement",
			tflops:         50,
			requiredTFLOPS: 100,
			expectedScore:  0.5,
		},
		{
			name:           "Quarter of requirement",
			tflops:         25,
			requiredTFLOPS: 100,
			expectedScore:  0.25,
		},
		{
			name:           "Slightly below requirement",
			tflops:         95,
			requiredTFLOPS: 100,
			expectedScore:  0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreFromTFLOPS(tt.tflops, tt.requiredTFLOPS)
			if score != tt.expectedScore {
				t.Errorf("scoreFromTFLOPS(%d, %d) = %f, want %f",
					tt.tflops, tt.requiredTFLOPS, score, tt.expectedScore)
			}
		})
	}
}

// TestGetTFLOPSForPrecision tests precision-based TFLOPS extraction
func TestGetTFLOPSForPrecision(t *testing.T) {
	perf := &v1beta1.AcceleratorPerformance{
		Fp32Tflops: int64Ptr(100),
		Fp16Tflops: int64Ptr(200),
		Int8Tops:   int64Ptr(400),
		Int4Tops:   int64Ptr(800),
	}

	tests := []struct {
		name           string
		perf           *v1beta1.AcceleratorPerformance
		precision      string
		expectedTFLOPS int64
	}{
		{
			name:           "FP32 precision",
			perf:           perf,
			precision:      "fp32",
			expectedTFLOPS: 100,
		},
		{
			name:           "FP16 precision",
			perf:           perf,
			precision:      "fp16",
			expectedTFLOPS: 200,
		},
		{
			name:           "INT8 precision",
			perf:           perf,
			precision:      "int8",
			expectedTFLOPS: 400,
		},
		{
			name:           "FP8 maps to INT8",
			perf:           perf,
			precision:      "fp8",
			expectedTFLOPS: 400,
		},
		{
			name:           "INT4 precision",
			perf:           perf,
			precision:      "int4",
			expectedTFLOPS: 800,
		},
		{
			name:           "Case insensitive - FP16",
			perf:           perf,
			precision:      "FP16",
			expectedTFLOPS: 200,
		},
		{
			name:           "Nil performance struct",
			perf:           nil,
			precision:      "fp16",
			expectedTFLOPS: 0,
		},
		{
			name: "Missing FP16 field",
			perf: &v1beta1.AcceleratorPerformance{
				Fp32Tflops: int64Ptr(100),
			},
			precision:      "fp16",
			expectedTFLOPS: 0,
		},
		{
			name:           "Unknown precision",
			perf:           perf,
			precision:      "unknown",
			expectedTFLOPS: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tflops := getTFLOPSForPrecision(tt.perf, tt.precision)
			if tflops != tt.expectedTFLOPS {
				t.Errorf("getTFLOPSForPrecision(%v, %s) = %d, want %d",
					tt.perf, tt.precision, tflops, tt.expectedTFLOPS)
			}
		})
	}
}

// TestGetMaxTFLOPS tests getting maximum TFLOPS across all precisions
func TestGetMaxTFLOPS(t *testing.T) {
	tests := []struct {
		name        string
		perf        *v1beta1.AcceleratorPerformance
		expectedMax int64
	}{
		{
			name: "INT4 is maximum",
			perf: &v1beta1.AcceleratorPerformance{
				Fp32Tflops: int64Ptr(100),
				Fp16Tflops: int64Ptr(200),
				Int8Tops:   int64Ptr(400),
				Int4Tops:   int64Ptr(800),
			},
			expectedMax: 800,
		},
		{
			name: "FP32 is maximum",
			perf: &v1beta1.AcceleratorPerformance{
				Fp32Tflops: int64Ptr(500),
				Fp16Tflops: int64Ptr(200),
			},
			expectedMax: 500,
		},
		{
			name: "Only FP16 available",
			perf: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(200),
			},
			expectedMax: 200,
		},
		{
			name:        "Nil performance struct",
			perf:        nil,
			expectedMax: 0,
		},
		{
			name:        "Empty performance struct",
			perf:        &v1beta1.AcceleratorPerformance{},
			expectedMax: 0,
		},
		{
			name: "All zero values",
			perf: &v1beta1.AcceleratorPerformance{
				Fp32Tflops: int64Ptr(0),
				Fp16Tflops: int64Ptr(0),
			},
			expectedMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			max := getMaxTFLOPS(tt.perf)
			if max != tt.expectedMax {
				t.Errorf("getMaxTFLOPS() = %d, want %d", max, tt.expectedMax)
			}
		})
	}
}

// TestCalculateMemoryFitScore tests memory fit scoring with over-provisioning penalty
func TestCalculateMemoryFitScore(t *testing.T) {
	tests := []struct {
		name           string
		candidateMemGB int64
		requiredMemGB  *int64
		expectedScore  float64
		description    string
	}{
		{
			name:           "No memory requirement",
			candidateMemGB: 40,
			requiredMemGB:  nil,
			expectedScore:  1.0,
			description:    "Should return perfect score when no requirement",
		},
		{
			name:           "Exact match",
			candidateMemGB: 40,
			requiredMemGB:  int64Ptr(40),
			expectedScore:  1.0,
			description:    "Exact match should score 1.0",
		},
		{
			name:           "2x over-provisioned",
			candidateMemGB: 80,
			requiredMemGB:  int64Ptr(40),
			expectedScore:  0.5,
			description:    "2x over = 0.5 score (1.0 / 2.0)",
		},
		{
			name:           "10x over-provisioned",
			candidateMemGB: 400,
			requiredMemGB:  int64Ptr(40),
			expectedScore:  0.1,
			description:    "10x over = 0.1 score (1.0 / 10.0), no cap",
		},
		{
			name:           "4x over-provisioned",
			candidateMemGB: 160,
			requiredMemGB:  int64Ptr(40),
			expectedScore:  0.25,
			description:    "4x over = 0.25 score",
		},
		{
			name:           "Slightly over-provisioned (1.25x)",
			candidateMemGB: 50,
			requiredMemGB:  int64Ptr(40),
			expectedScore:  0.8,
			description:    "1.25x over = 0.8 score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create candidate with memory
			memBytes := tt.candidateMemGB * 1024 * 1024 * 1024
			memQuantity := resource.NewQuantity(memBytes, resource.BinarySI)

			candidate := candidateAccelerator{
				Name: "test-accelerator",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB: memQuantity,
					},
				},
			}

			constraints := &v1beta1.AcceleratorConstraints{
				MinMemory: tt.requiredMemGB,
			}

			score := calculateMemoryFitScore(candidate, constraints)

			// Allow small floating point tolerance
			tolerance := 0.01
			if score < tt.expectedScore-tolerance || score > tt.expectedScore+tolerance {
				t.Errorf("%s: calculateMemoryFitScore() = %f, want %f (tolerance ±%f)",
					tt.description, score, tt.expectedScore, tolerance)
			}
		})
	}
}

// TestCalculateComputePerformanceTFLOPSScore tests compute scoring with precision fallback
func TestCalculateComputePerformanceTFLOPSScore(t *testing.T) {
	tests := []struct {
		name                string
		performance         *v1beta1.AcceleratorPerformance
		requiredTFLOPS      *int64
		preferredPrecisions []string
		expectedScore       float64
		description         string
	}{
		{
			name: "No requirement - perfect score",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(100),
			},
			requiredTFLOPS:      nil,
			preferredPrecisions: []string{"fp16"},
			expectedScore:       1.0,
			description:         "No MinComputePerformanceTFLOPS requirement should return 1.0",
		},
		{
			name: "First precision exact match",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(100),
			},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp16"},
			expectedScore:       1.0,
			description:         "First precision meeting requirement gets full score",
		},
		{
			name: "First precision exceeds requirement",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(200),
			},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp16"},
			expectedScore:       1.0,
			description:         "Exceeding requirement caps at 1.0",
		},
		{
			name: "First precision half of requirement",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(50),
			},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp16"},
			expectedScore:       0.5,
			description:         "50 TFLOPS / 100 required = 0.5 score",
		},
		{
			name: "Second precision fallback with penalty",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(100),
			},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp8", "fp16"},
			expectedScore:       0.5,
			description:         "Second precision gets 0.5 penalty (1.0 * 0.5)",
		},
		{
			name: "Third precision fallback with penalty",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(100),
			},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"int4", "int8", "fp16"},
			expectedScore:       0.25,
			description:         "Third precision gets 0.25 penalty (1.0 * 0.5 * 0.5)",
		},
		{
			name: "No preferred precisions - use max",
			performance: &v1beta1.AcceleratorPerformance{
				Fp32Tflops: int64Ptr(50),
				Fp16Tflops: int64Ptr(100),
				Int8Tops:   int64Ptr(200),
			},
			requiredTFLOPS:      int64Ptr(200),
			preferredPrecisions: []string{},
			expectedScore:       1.0,
			description:         "No preference uses max TFLOPS (INT8 = 200)",
		},
		{
			name:                "No precision has TFLOPS data",
			performance:         &v1beta1.AcceleratorPerformance{},
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp16"},
			expectedScore:       0.0,
			description:         "No TFLOPS data returns 0.0",
		},
		{
			name:                "Nil performance struct",
			performance:         nil,
			requiredTFLOPS:      int64Ptr(100),
			preferredPrecisions: []string{"fp16"},
			expectedScore:       0.0,
			description:         "Nil performance returns 0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := candidateAccelerator{
				Name: "test-accelerator",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						Performance: tt.performance,
					},
				},
			}

			constraints := &v1beta1.AcceleratorConstraints{
				MinComputePerformanceTFLOPS: tt.requiredTFLOPS,
				PreferredPrecisions:         tt.preferredPrecisions,
			}

			score := calculateComputePerformanceTFLOPSScore(candidate, constraints)

			tolerance := 0.01
			if score < tt.expectedScore-tolerance || score > tt.expectedScore+tolerance {
				t.Errorf("%s: calculateComputePerformanceTFLOPSScore() = %f, want %f",
					tt.description, score, tt.expectedScore)
			}
		})
	}
}

// TestCalculateBestFitScore tests the overall BestFit scoring (70-30 weights: memory-precision)
func TestCalculateBestFitScore(t *testing.T) {
	tests := []struct {
		name                string
		candidateMemGB      int64
		candidateTFLOPS     int64
		requiredMemGB       *int64
		precision           string
		preferredPrecisions []string
		expectedScore       float64
		description         string
	}{
		{
			name:                "Perfect match on memory and precision",
			candidateMemGB:      40,
			candidateTFLOPS:     100,
			requiredMemGB:       int64Ptr(40),
			precision:           "fp16",
			preferredPrecisions: []string{"fp16"},
			expectedScore:       1.0,
			description:         "Perfect match should score 1.0",
		},
		{
			name:                "Memory 2x over, precision exact",
			candidateMemGB:      80,
			candidateTFLOPS:     100,
			requiredMemGB:       int64Ptr(40),
			precision:           "fp16",
			preferredPrecisions: []string{"fp16"},
			expectedScore:       0.65, // 0.7 * 0.5 (mem) + 0.3 * 1.0 (precision)
			description:         "2x memory over-provisioning",
		},
		{
			name:                "Memory exact, precision match",
			candidateMemGB:      40,
			candidateTFLOPS:     100,
			requiredMemGB:       int64Ptr(40),
			precision:           "fp16",
			preferredPrecisions: []string{"fp16"},
			expectedScore:       1.0, // 0.7 * 1.0 (mem) + 0.3 * 1.0 (precision)
			description:         "Exact memory and precision match",
		},
		{
			name:                "Memory exact, second precision match",
			candidateMemGB:      40,
			candidateTFLOPS:     100,
			requiredMemGB:       int64Ptr(40),
			precision:           "fp16",
			preferredPrecisions: []string{"fp8", "fp16"},
			expectedScore:       0.85, // 0.7 * 1.0 (mem) + 0.3 * 0.5 (compute: fp16 with 0.5 penalty)
			description:         "Memory exact, second precision in preference list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memBytes := tt.candidateMemGB * 1024 * 1024 * 1024
			memQuantity := resource.NewQuantity(memBytes, resource.BinarySI)

			var perf *v1beta1.AcceleratorPerformance
			switch tt.precision {
			case "fp16":
				perf = &v1beta1.AcceleratorPerformance{
					Fp16Tflops: &tt.candidateTFLOPS,
				}
			case "fp32":
				perf = &v1beta1.AcceleratorPerformance{
					Fp32Tflops: &tt.candidateTFLOPS,
				}
			case "fp8":
				perf = &v1beta1.AcceleratorPerformance{
					Int8Tops: &tt.candidateTFLOPS,
				}
			}

			candidate := candidateAccelerator{
				Name: "test-accelerator",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB:    memQuantity,
						Performance: perf,
					},
				},
			}

			constraints := &v1beta1.AcceleratorConstraints{
				MinMemory:           tt.requiredMemGB,
				PreferredPrecisions: tt.preferredPrecisions,
			}

			score := calculateBestFitScore(candidate, constraints)

			tolerance := 0.02
			if score < tt.expectedScore-tolerance || score > tt.expectedScore+tolerance {
				t.Errorf("%s: calculateBestFitScore() = %f, want %f",
					tt.description, score, tt.expectedScore)
			}
		})
	}
}

// TestGetCandidateCost tests cost extraction with priority
func TestGetCandidateCost(t *testing.T) {
	tests := []struct {
		name         string
		cost         *v1beta1.AcceleratorCost
		expectedType string
		shouldError  bool
		description  string
	}{
		{
			name: "Spot pricing preferred",
			cost: &v1beta1.AcceleratorCost{
				SpotPerHour:      resource.NewQuantity(1, resource.DecimalSI),
				PerHour:          resource.NewQuantity(2, resource.DecimalSI),
				PerMillionTokens: resource.NewQuantity(3, resource.DecimalSI),
			},
			expectedType: "spot-hourly",
			shouldError:  false,
			description:  "Spot pricing should be preferred over on-demand",
		},
		{
			name: "On-demand when no spot",
			cost: &v1beta1.AcceleratorCost{
				PerHour:          resource.NewQuantity(2, resource.DecimalSI),
				PerMillionTokens: resource.NewQuantity(3, resource.DecimalSI),
			},
			expectedType: "hourly",
			shouldError:  false,
			description:  "On-demand should be used when spot unavailable",
		},
		{
			name: "Token pricing when no hourly",
			cost: &v1beta1.AcceleratorCost{
				PerMillionTokens: resource.NewQuantity(3, resource.DecimalSI),
			},
			expectedType: "per-million-tokens",
			shouldError:  false,
			description:  "Token pricing as fallback",
		},
		{
			name: "Cost tier fallback",
			cost: &v1beta1.AcceleratorCost{
				Tier: "low",
			},
			expectedType: "tier",
			shouldError:  false,
			description:  "Tier string as last resort",
		},
		{
			name:        "No cost information",
			cost:        nil,
			shouldError: true,
			description: "Should error when no cost info",
		},
		{
			name:        "Empty cost struct",
			cost:        &v1beta1.AcceleratorCost{},
			shouldError: true,
			description: "Should error when cost struct has no data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := candidateAccelerator{
				Name: "test-accelerator",
				Spec: &v1beta1.AcceleratorClassSpec{
					Cost: tt.cost,
				},
			}

			cost, costType, err := getCandidateCost(candidate)

			if tt.shouldError {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
				if costType != tt.expectedType {
					t.Errorf("%s: costType = %s, want %s", tt.description, costType, tt.expectedType)
				}
				if cost == nil {
					t.Errorf("%s: cost is nil", tt.description)
				}
			}
		})
	}
}

// TestCompareCosts tests cost comparison logic
func TestCompareCosts(t *testing.T) {
	tests := []struct {
		name     string
		cost1    *resource.Quantity
		cost2    *resource.Quantity
		expected int
	}{
		{
			name:     "cost1 < cost2",
			cost1:    resource.NewQuantity(1, resource.DecimalSI),
			cost2:    resource.NewQuantity(2, resource.DecimalSI),
			expected: -1,
		},
		{
			name:     "cost1 > cost2",
			cost1:    resource.NewQuantity(3, resource.DecimalSI),
			cost2:    resource.NewQuantity(2, resource.DecimalSI),
			expected: 1,
		},
		{
			name:     "cost1 == cost2",
			cost1:    resource.NewQuantity(2, resource.DecimalSI),
			cost2:    resource.NewQuantity(2, resource.DecimalSI),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareCosts(tt.cost1, tt.cost2)
			if result != tt.expected {
				t.Errorf("compareCosts() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestSortScoredCandidates tests candidate sorting
func TestSortScoredCandidates(t *testing.T) {
	candidates := []scoredCandidate{
		{candidateAccelerator: candidateAccelerator{Name: "c1"}, Score: 0.5},
		{candidateAccelerator: candidateAccelerator{Name: "c2"}, Score: 0.9},
		{candidateAccelerator: candidateAccelerator{Name: "c3"}, Score: 0.3},
		{candidateAccelerator: candidateAccelerator{Name: "c4"}, Score: 0.7},
	}

	t.Run("Sort descending (highest first)", func(t *testing.T) {
		testCandidates := make([]scoredCandidate, len(candidates))
		copy(testCandidates, candidates)

		sortScoredCandidates(testCandidates, false)

		expected := []string{"c2", "c4", "c1", "c3"}
		for i, name := range expected {
			if testCandidates[i].Name != name {
				t.Errorf("Position %d: got %s, want %s", i, testCandidates[i].Name, name)
			}
		}
	})

	t.Run("Sort ascending (lowest first)", func(t *testing.T) {
		testCandidates := make([]scoredCandidate, len(candidates))
		copy(testCandidates, candidates)

		sortScoredCandidates(testCandidates, true)

		expected := []string{"c3", "c1", "c4", "c2"}
		for i, name := range expected {
			if testCandidates[i].Name != name {
				t.Errorf("Position %d: got %s, want %s", i, testCandidates[i].Name, name)
			}
		}
	})

	t.Run("Empty slice", func(t *testing.T) {
		testCandidates := []scoredCandidate{}
		sortScoredCandidates(testCandidates, false)
		// Should not panic
	})

	t.Run("Single element", func(t *testing.T) {
		testCandidates := []scoredCandidate{
			{candidateAccelerator: candidateAccelerator{Name: "only"}, Score: 1.0},
		}
		sortScoredCandidates(testCandidates, false)
		if testCandidates[0].Name != "only" {
			t.Errorf("Single element changed: got %s", testCandidates[0].Name)
		}
	})
}

// TestCalculateCapabilityScore tests MostCapable scoring
func TestCalculateCapabilityScore(t *testing.T) {
	tests := []struct {
		name                string
		performance         *v1beta1.AcceleratorPerformance
		memoryGB            int64
		bandwidthGBps       int64
		preferredPrecisions []string
		expectedNonZero     bool
		description         string
	}{
		{
			name: "High performance GPU",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(1000),
			},
			memoryGB:            80,
			bandwidthGBps:       2000,
			preferredPrecisions: []string{"fp16"},
			expectedNonZero:     true,
			description:         "Should score high for capable GPU",
		},
		{
			name: "Low performance GPU",
			performance: &v1beta1.AcceleratorPerformance{
				Fp16Tflops: int64Ptr(100),
			},
			memoryGB:            16,
			bandwidthGBps:       500,
			preferredPrecisions: []string{"fp16"},
			expectedNonZero:     true,
			description:         "Should score lower but non-zero",
		},
		{
			name:                "No performance data - memory fallback",
			performance:         nil,
			memoryGB:            40,
			bandwidthGBps:       1000,
			preferredPrecisions: []string{"fp16"},
			expectedNonZero:     true,
			description:         "Should use memory fallback when no performance data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memBytes := tt.memoryGB * 1024 * 1024 * 1024
			memQuantity := resource.NewQuantity(memBytes, resource.BinarySI)
			bandwidthQuantity := resource.NewQuantity(tt.bandwidthGBps, resource.DecimalSI)

			candidate := candidateAccelerator{
				Name: "test-accelerator",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB:            memQuantity,
						MemoryBandwidthGBps: bandwidthQuantity,
						Performance:         tt.performance,
					},
				},
			}

			score := calculateCapabilityScore(candidate, tt.preferredPrecisions)

			if tt.expectedNonZero && score == 0.0 {
				t.Errorf("%s: expected non-zero score, got 0.0", tt.description)
			}
			if !tt.expectedNonZero && score != 0.0 {
				t.Errorf("%s: expected 0.0 score, got %f", tt.description, score)
			}
		})
	}
}

// TestMeetsRequirements tests the filtering logic for individual candidates
func TestMeetsRequirements(t *testing.T) {
	tests := []struct {
		name                 string
		candidate            candidateAccelerator
		constraints          *v1beta1.AcceleratorConstraints
		considerAvailability bool
		expectedEligible     bool
		expectedReason       string
	}{
		{
			name: "No constraints - always eligible",
			candidate: candidateAccelerator{
				Name: "test-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{},
			},
			constraints:      nil,
			expectedEligible: true,
			expectedReason:   "",
		},
		{
			name: "Explicitly excluded",
			candidate: candidateAccelerator{
				Name: "excluded-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				ExcludedClasses: []string{"excluded-gpu", "other-gpu"},
			},
			expectedEligible: false,
			expectedReason:   "explicitly excluded",
		},
		{
			name: "Below MinMemory",
			candidate: candidateAccelerator{
				Name: "small-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB: resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
					},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(32),
			},
			expectedEligible: false,
			expectedReason:   "memory 16GB < required 32GB",
		},
		{
			name: "Meets MinMemory",
			candidate: candidateAccelerator{
				Name: "medium-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB: resource.NewQuantity(40*1024*1024*1024, resource.BinarySI),
					},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(32),
			},
			expectedEligible: true,
			expectedReason:   "",
		},
		{
			name: "Exceeds MaxMemory",
			candidate: candidateAccelerator{
				Name: "large-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						MemoryGB: resource.NewQuantity(80*1024*1024*1024, resource.BinarySI),
					},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				MaxMemory: int64Ptr(64),
			},
			expectedEligible: false,
			expectedReason:   "memory 80GB > max allowed 64GB",
		},
		{
			name: "Missing required feature",
			candidate: candidateAccelerator{
				Name: "basic-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						Features: []string{"cuda", "tensor-cores"},
					},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				RequiredFeatures: []string{"nvlink"},
			},
			expectedEligible: false,
			expectedReason:   "missing required feature: nvlink",
		},
		{
			name: "Has all required features",
			candidate: candidateAccelerator{
				Name: "full-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{
						Features: []string{"cuda", "tensor-cores", "nvlink"},
					},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				RequiredFeatures: []string{"tensor-cores", "nvlink"},
			},
			expectedEligible: true,
			expectedReason:   "",
		},
		{
			name: "Architecture family mismatch",
			candidate: candidateAccelerator{
				Name: "amd-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Vendor: "AMD",
					Family: "RDNA",
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				ArchitectureFamilies: []string{"nvidia-ampere", "nvidia-hopper"},
			},
			expectedEligible: false,
			expectedReason:   "architecture family rdna not in allowed list",
		},
		{
			name: "Architecture family match",
			candidate: candidateAccelerator{
				Name: "nvidia-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{
					Vendor: "NVIDIA",
					Family: "Ampere",
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				ArchitectureFamilies: []string{"nvidia-ampere", "nvidia-hopper"},
			},
			expectedEligible: true,
			expectedReason:   "",
		},
		{
			name: "Zero availability when considerAvailability=true",
			candidate: candidateAccelerator{
				Name: "unavailable-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{},
				Status: &v1beta1.AcceleratorClassStatus{
					AvailableAccelerators: 0,
				},
			},
			constraints:          &v1beta1.AcceleratorConstraints{},
			considerAvailability: true,
			expectedEligible:     false,
			expectedReason:       "no accelerators currently available",
		},
		{
			name: "Zero availability when considerAvailability=false",
			candidate: candidateAccelerator{
				Name: "unavailable-gpu",
				Spec: &v1beta1.AcceleratorClassSpec{},
				Status: &v1beta1.AcceleratorClassStatus{
					AvailableAccelerators: 0,
				},
			},
			constraints:          &v1beta1.AcceleratorConstraints{},
			considerAvailability: false,
			expectedEligible:     true,
			expectedReason:       "",
		},
		{
			name: "Missing memory specification when MinMemory required",
			candidate: candidateAccelerator{
				Name: "no-mem-spec",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(40),
			},
			expectedEligible: false,
			expectedReason:   "missing memory specification for MinMemory check",
		},
		{
			name: "MinComputePerformanceTFLOPS not a hard filter (soft constraint)",
			candidate: candidateAccelerator{
				Name: "no-perf-spec",
				Spec: &v1beta1.AcceleratorClassSpec{
					Capabilities: v1beta1.AcceleratorCapabilities{},
				},
			},
			constraints: &v1beta1.AcceleratorConstraints{
				MinComputePerformanceTFLOPS: int64Ptr(100),
			},
			expectedEligible: true,
			expectedReason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligible, reason := meetsRequirements(tt.candidate, tt.constraints, tt.considerAvailability)

			if eligible != tt.expectedEligible {
				t.Errorf("meetsRequirements() eligible = %v, want %v (reason: %s)",
					eligible, tt.expectedEligible, reason)
			}

			if !eligible && tt.expectedReason != "" {
				if reason != tt.expectedReason {
					t.Errorf("meetsRequirements() reason = %q, want %q", reason, tt.expectedReason)
				}
			}
		})
	}
}
