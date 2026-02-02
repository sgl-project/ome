package acceleratorclassselector

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// mockAcceleratorFetcher is a mock implementation of AcceleratorFetcher for testing
type mockAcceleratorFetcher struct {
	accelerators map[string]*v1beta1.AcceleratorClassSpec
}

func newMockFetcher() *mockAcceleratorFetcher {
	return &mockAcceleratorFetcher{
		accelerators: make(map[string]*v1beta1.AcceleratorClassSpec),
	}
}

func (m *mockAcceleratorFetcher) addAccelerator(name string, spec *v1beta1.AcceleratorClassSpec) {
	m.accelerators[name] = spec
}

func (m *mockAcceleratorFetcher) GetAcceleratorClass(ctx context.Context, name string) (*v1beta1.AcceleratorClassSpec, bool, error) {
	spec, found := m.accelerators[name]
	return spec, found, nil
}

func (m *mockAcceleratorFetcher) FetchAcceleratorClasses(ctx context.Context) (*AcceleratorCollection, error) {
	return &AcceleratorCollection{}, nil
}

// createRealisticAccelerators creates H100, A100, and T4 AcceleratorClass specs with realistic data
func createRealisticAccelerators() map[string]*v1beta1.AcceleratorClassSpec {
	return map[string]*v1beta1.AcceleratorClassSpec{
		"nvidia-h100-80gb": {
			Vendor: "NVIDIA",
			Family: "Hopper",
			Model:  "H100",
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB:            resource.NewQuantity(80*1024*1024*1024, resource.BinarySI),
				MemoryBandwidthGBps: resource.NewQuantity(3000, resource.DecimalSI),
				Performance: &v1beta1.AcceleratorPerformance{
					Fp32Tflops: int64Ptr(67),
					Fp16Tflops: int64Ptr(1979),
					Int8Tops:   int64Ptr(3958), // FP8 capable
					Int4Tops:   int64Ptr(7916),
				},
				Features:          []string{"cuda", "tensor-cores", "nvlink", "fp8"},
				ComputeCapability: "9.0",
			},
			Cost: &v1beta1.AcceleratorCost{
				PerHour:          resource.NewQuantity(4, resource.DecimalSI),
				SpotPerHour:      resource.NewQuantity(2, resource.DecimalSI),
				PerMillionTokens: resource.NewQuantity(100, resource.DecimalSI),
				Tier:             "high",
			},
		},
		"nvidia-a100-40gb": {
			Vendor: "NVIDIA",
			Family: "Ampere",
			Model:  "A100",
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB:            resource.NewQuantity(40*1024*1024*1024, resource.BinarySI),
				MemoryBandwidthGBps: resource.NewQuantity(1555, resource.DecimalSI),
				Performance: &v1beta1.AcceleratorPerformance{
					Fp32Tflops: int64Ptr(19),
					Fp16Tflops: int64Ptr(312),
					Int8Tops:   int64Ptr(624),
				},
				Features:          []string{"cuda", "tensor-cores", "nvlink"},
				ComputeCapability: "8.0",
			},
			Cost: &v1beta1.AcceleratorCost{
				PerHour:          resource.NewQuantity(2, resource.DecimalSI),
				SpotPerHour:      resource.NewQuantity(1, resource.DecimalSI),
				PerMillionTokens: resource.NewQuantity(50, resource.DecimalSI),
				Tier:             "medium",
			},
		},
		"nvidia-t4": {
			Vendor: "NVIDIA",
			Family: "Turing",
			Model:  "T4",
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB:            resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				MemoryBandwidthGBps: resource.NewQuantity(320, resource.DecimalSI),
				Performance: &v1beta1.AcceleratorPerformance{
					Fp32Tflops: int64Ptr(8),
					Fp16Tflops: int64Ptr(65),
					Int8Tops:   int64Ptr(130),
				},
				Features:          []string{"cuda", "tensor-cores"},
				ComputeCapability: "7.5",
			},
			Cost: &v1beta1.AcceleratorCost{
				PerHour:          resource.NewQuantity(1, resource.DecimalSI),
				SpotPerHour:      resource.NewQuantity(0, resource.DecimalSI),
				PerMillionTokens: resource.NewQuantity(20, resource.DecimalSI),
				Tier:             "low",
			},
		},
	}
}

// TestSelectBestFit_Integration tests BestFit policy with realistic data
func TestSelectBestFit_Integration(t *testing.T) {
	tests := []struct {
		name           string
		constraints    *v1beta1.AcceleratorConstraints
		candidateNames []string
		expectedName   string
		description    string
	}{
		{
			name: "40GB requirement - A100 best fit",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:           int64Ptr(40),
				PreferredPrecisions: []string{"fp16"},
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb"},
			expectedName:   "nvidia-a100-40gb",
			description:    "A100-40GB should be selected over H100-80GB to avoid over-provisioning",
		},
		{
			name: "80GB requirement - H100 only option",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:           int64Ptr(80),
				PreferredPrecisions: []string{"fp16"},
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			expectedName:   "nvidia-h100-80gb",
			description:    "Only H100 meets 80GB requirement",
		},
		{
			name: "FP8 preference with high memory - H100 selected",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:           int64Ptr(80),
				PreferredPrecisions: []string{"fp8", "fp16"},
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb"},
			expectedName:   "nvidia-h100-80gb",
			description:    "H100 exact memory match + fp8 support wins over A100",
		},
		{
			name: "NVLink required - filters out T4",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:        int64Ptr(16),
				RequiredFeatures: []string{"nvlink"},
			},
			candidateNames: []string{"nvidia-a100-40gb", "nvidia-t4"},
			expectedName:   "nvidia-a100-40gb",
			description:    "T4 lacks nvlink, A100 selected",
		},
		{
			name: "Single candidate after filtering",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
				MaxMemory: int64Ptr(20),
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			expectedName:   "nvidia-t4",
			description:    "Only T4 within memory range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock fetcher
			mockFetcher := newMockFetcher()
			accelerators := createRealisticAccelerators()
			for name, spec := range accelerators {
				mockFetcher.addAccelerator(name, spec)
			}

			// Create selector
			config := &Config{
				Client:                nil,
				EnableDetailedLogging: false,
				ConsiderAvailability:  false,
			}
			selector := &defaultSelector{
				config:  config,
				fetcher: mockFetcher,
			}

			ctx := context.Background()

			// Fetch candidates
			candidates, err := candidatesFromNames(ctx, mockFetcher, tt.candidateNames)
			if err != nil {
				t.Fatalf("Failed to fetch candidates: %v", err)
			}

			// Filter candidates
			validCandidates := filterCandidates(ctx, candidates, tt.constraints, false)

			if len(validCandidates) == 0 {
				t.Fatalf("%s: No candidates passed filtering", tt.description)
			}

			// Select best fit
			selected := selector.selectBestFit(ctx, validCandidates, tt.constraints)

			if selected == nil {
				t.Errorf("%s: selectBestFit() returned nil", tt.description)
			} else if *selected != tt.expectedName {
				t.Errorf("%s: selectBestFit() = %s, want %s", tt.description, *selected, tt.expectedName)
			}
		})
	}
}

// TestSelectCheapest_Integration tests Cheapest policy with realistic data
func TestSelectCheapest_Integration(t *testing.T) {
	tests := []struct {
		name           string
		constraints    *v1beta1.AcceleratorConstraints
		candidateNames []string
		expectedName   string
		description    string
	}{
		{
			name: "All meet requirements - T4 cheapest",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			expectedName:   "nvidia-t4",
			description:    "T4 has lowest cost ($1/hr vs $2/hr vs $4/hr)",
		},
		{
			name: "Spot pricing preferred",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(40),
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb"},
			expectedName:   "nvidia-a100-40gb",
			description:    "A100 spot ($1/hr) cheaper than H100 spot ($2/hr)",
		},
		{
			name: "High memory requirement - H100 vs A100",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(80),
			},
			candidateNames: []string{"nvidia-h100-80gb", "nvidia-a100-40gb"},
			expectedName:   "nvidia-h100-80gb",
			description:    "Only H100 meets requirement (A100 filtered out)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock fetcher
			mockFetcher := newMockFetcher()
			accelerators := createRealisticAccelerators()
			for name, spec := range accelerators {
				mockFetcher.addAccelerator(name, spec)
			}

			// Create selector
			config := &Config{
				Client:                nil,
				EnableDetailedLogging: false,
				ConsiderAvailability:  false,
			}
			selector := &defaultSelector{
				config:  config,
				fetcher: mockFetcher,
			}

			ctx := context.Background()

			// Fetch candidates
			candidates, err := candidatesFromNames(ctx, mockFetcher, tt.candidateNames)
			if err != nil {
				t.Fatalf("Failed to fetch candidates: %v", err)
			}

			// Filter candidates
			validCandidates := filterCandidates(ctx, candidates, tt.constraints, false)

			if len(validCandidates) == 0 {
				t.Fatalf("%s: No candidates passed filtering", tt.description)
			}

			// Select cheapest
			selected := selector.selectCheapest(ctx, validCandidates)

			if selected == nil {
				t.Errorf("%s: selectCheapest() returned nil", tt.description)
			} else if *selected != tt.expectedName {
				t.Errorf("%s: selectCheapest() = %s, want %s", tt.description, *selected, tt.expectedName)
			}
		})
	}
}

// TestSelectMostCapable_Integration tests MostCapable policy with realistic data
func TestSelectMostCapable_Integration(t *testing.T) {
	tests := []struct {
		name                string
		constraints         *v1beta1.AcceleratorConstraints
		candidateNames      []string
		preferredPrecisions []string
		expectedName        string
		description         string
	}{
		{
			name: "FP8 workload - H100 most capable",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			candidateNames:      []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			preferredPrecisions: []string{"fp8", "fp16"},
			expectedName:        "nvidia-h100-80gb",
			description:         "H100 has highest INT8 TOPS (3958) for fp8 workload",
		},
		{
			name: "FP16 workload - H100 most capable",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			candidateNames:      []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			preferredPrecisions: []string{"fp16"},
			expectedName:        "nvidia-h100-80gb",
			description:         "H100 has highest FP16 TFLOPS (1979)",
		},
		{
			name: "FP32 workload - H100 most capable",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			candidateNames:      []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			preferredPrecisions: []string{"fp32"},
			expectedName:        "nvidia-h100-80gb",
			description:         "H100 has highest FP32 TFLOPS (67)",
		},
		{
			name: "Memory constraint filters H100",
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
				MaxMemory: int64Ptr(50),
			},
			candidateNames:      []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
			preferredPrecisions: []string{"fp16"},
			expectedName:        "nvidia-a100-40gb",
			description:         "H100 filtered by MaxMemory, A100 most capable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock fetcher
			mockFetcher := newMockFetcher()
			accelerators := createRealisticAccelerators()
			for name, spec := range accelerators {
				mockFetcher.addAccelerator(name, spec)
			}

			// Create selector
			config := &Config{
				Client:                nil,
				EnableDetailedLogging: false,
				ConsiderAvailability:  false,
			}
			selector := &defaultSelector{
				config:  config,
				fetcher: mockFetcher,
			}

			ctx := context.Background()

			// Fetch candidates
			candidates, err := candidatesFromNames(ctx, mockFetcher, tt.candidateNames)
			if err != nil {
				t.Fatalf("Failed to fetch candidates: %v", err)
			}

			// Filter candidates
			validCandidates := filterCandidates(ctx, candidates, tt.constraints, false)

			if len(validCandidates) == 0 {
				t.Fatalf("%s: No candidates passed filtering", tt.description)
			}

			// Select most capable
			selected := selector.selectMostCapable(ctx, validCandidates, tt.preferredPrecisions)

			if selected == nil {
				t.Errorf("%s: selectMostCapable() returned nil", tt.description)
			} else if *selected != tt.expectedName {
				t.Errorf("%s: selectMostCapable() = %s, want %s", tt.description, *selected, tt.expectedName)
			}
		})
	}
}

// TestGetAcceleratorClassByPolicy_AllPolicies tests the main policy routing logic
func TestGetAcceleratorClassByPolicy_AllPolicies(t *testing.T) {
	tests := []struct {
		name         string
		policy       v1beta1.AcceleratorSelectionPolicy
		constraints  *v1beta1.AcceleratorConstraints
		expectedName string
		description  string
	}{
		{
			name:   "BestFit policy - A100 for 40GB",
			policy: v1beta1.BestFitPolicy,
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:           int64Ptr(40),
				PreferredPrecisions: []string{"fp16"},
			},
			expectedName: "nvidia-a100-40gb",
			description:  "BestFit should select A100 to avoid over-provisioning",
		},
		{
			name:   "Cheapest policy - T4 for 16GB",
			policy: v1beta1.CheapestPolicy,
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			expectedName: "nvidia-t4",
			description:  "Cheapest should select T4 ($1/hr)",
		},
		{
			name:   "MostCapable policy - H100 for fp8",
			policy: v1beta1.MostCapablePolicy,
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory:           int64Ptr(16),
				PreferredPrecisions: []string{"fp8"},
			},
			expectedName: "nvidia-h100-80gb",
			description:  "MostCapable should select H100 for fp8 workload",
		},
		{
			name:   "FirstAvailable policy - first in list",
			policy: v1beta1.FirstAvailablePolicy,
			constraints: &v1beta1.AcceleratorConstraints{
				MinMemory: int64Ptr(16),
			},
			expectedName: "nvidia-h100-80gb",
			description:  "FirstAvailable should select first in runtime list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock fetcher
			mockFetcher := newMockFetcher()
			accelerators := createRealisticAccelerators()
			for name, spec := range accelerators {
				mockFetcher.addAccelerator(name, spec)
			}

			// Create selector
			config := &Config{
				Client:                nil,
				EnableDetailedLogging: false,
				ConsiderAvailability:  false,
			}
			selector := &defaultSelector{
				config:  config,
				fetcher: mockFetcher,
			}

			// Create InferenceService
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					AcceleratorSelector: &v1beta1.AcceleratorSelector{
						Policy:      tt.policy,
						Constraints: tt.constraints,
					},
				},
			}

			// Create ServingRuntime
			runtime := &v1beta1.ServingRuntimeSpec{
				AcceleratorRequirements: &v1beta1.AcceleratorRequirements{
					AcceleratorClasses: []string{"nvidia-h100-80gb", "nvidia-a100-40gb", "nvidia-t4"},
				},
			}

			ctx := context.Background()

			// Call getAcceleratorClassByPolicy
			selected := selector.getAcceleratorClassByPolicy(ctx, isvc, runtime, tt.policy)

			if selected == nil {
				t.Errorf("%s: getAcceleratorClassByPolicy() returned nil", tt.description)
			} else if *selected != tt.expectedName {
				t.Errorf("%s: getAcceleratorClassByPolicy() = %s, want %s",
					tt.description, *selected, tt.expectedName)
			}
		})
	}
}

// TestEdgeCases tests edge cases for policy selection
func TestEdgeCases(t *testing.T) {
	t.Run("Empty candidate list", func(t *testing.T) {
		mockFetcher := newMockFetcher()
		config := &Config{
			Client:                nil,
			EnableDetailedLogging: false,
			ConsiderAvailability:  false,
		}
		selector := &defaultSelector{
			config:  config,
			fetcher: mockFetcher,
		}

		ctx := context.Background()
		validCandidates := []candidateAccelerator{}

		selected := selector.selectBestFit(ctx, validCandidates, nil)
		if selected != nil {
			t.Errorf("Empty candidates should return nil, got %s", *selected)
		}
	})

	t.Run("All candidates filtered out", func(t *testing.T) {
		mockFetcher := newMockFetcher()
		accelerators := createRealisticAccelerators()
		for name, spec := range accelerators {
			mockFetcher.addAccelerator(name, spec)
		}

		ctx := context.Background()
		candidates, _ := candidatesFromNames(ctx, mockFetcher, []string{"nvidia-t4"})

		// Filter with impossible constraint
		constraints := &v1beta1.AcceleratorConstraints{
			MinMemory: int64Ptr(100), // T4 only has 16GB
		}
		validCandidates := filterCandidates(ctx, candidates, constraints, false)

		if len(validCandidates) != 0 {
			t.Errorf("Expected all candidates filtered, got %d", len(validCandidates))
		}
	})

	t.Run("Single candidate optimization", func(t *testing.T) {
		mockFetcher := newMockFetcher()
		accelerators := createRealisticAccelerators()
		for name, spec := range accelerators {
			mockFetcher.addAccelerator(name, spec)
		}

		config := &Config{
			Client:                nil,
			EnableDetailedLogging: false,
			ConsiderAvailability:  false,
		}
		selector := &defaultSelector{
			config:  config,
			fetcher: mockFetcher,
		}

		ctx := context.Background()
		candidates, _ := candidatesFromNames(ctx, mockFetcher, []string{"nvidia-t4"})

		// Should return immediately without scoring
		selected := selector.selectBestFit(ctx, candidates, nil)
		if selected == nil || *selected != "nvidia-t4" {
			t.Errorf("Single candidate should be selected immediately")
		}
	})

	t.Run("Cheapest with no cost data for multiple candidates", func(t *testing.T) {
		mockFetcher := newMockFetcher()
		// Add multiple accelerators without cost data
		mockFetcher.addAccelerator("no-cost-gpu-1", &v1beta1.AcceleratorClassSpec{
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB: resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
			},
		})
		mockFetcher.addAccelerator("no-cost-gpu-2", &v1beta1.AcceleratorClassSpec{
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB: resource.NewQuantity(32*1024*1024*1024, resource.BinarySI),
			},
		})

		config := &Config{
			Client:                nil,
			EnableDetailedLogging: false,
			ConsiderAvailability:  false,
		}
		selector := &defaultSelector{
			config:  config,
			fetcher: mockFetcher,
		}

		ctx := context.Background()
		candidates, _ := candidatesFromNames(ctx, mockFetcher, []string{"no-cost-gpu-1", "no-cost-gpu-2"})

		selected := selector.selectCheapest(ctx, candidates)
		if selected != nil {
			t.Errorf("Cheapest with no cost data should return nil, got %s", *selected)
		}
	})

	t.Run("Single candidate without cost data - still selected", func(t *testing.T) {
		mockFetcher := newMockFetcher()
		// Add single accelerator without cost data
		mockFetcher.addAccelerator("no-cost-gpu", &v1beta1.AcceleratorClassSpec{
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB: resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
			},
		})

		config := &Config{
			Client:                nil,
			EnableDetailedLogging: false,
			ConsiderAvailability:  false,
		}
		selector := &defaultSelector{
			config:  config,
			fetcher: mockFetcher,
		}

		ctx := context.Background()
		candidates, _ := candidatesFromNames(ctx, mockFetcher, []string{"no-cost-gpu"})

		selected := selector.selectCheapest(ctx, candidates)
		if selected == nil || *selected != "no-cost-gpu" {
			t.Errorf("Single candidate should be selected even without cost data")
		}
	})
}
