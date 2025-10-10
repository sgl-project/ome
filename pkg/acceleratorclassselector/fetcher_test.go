package acceleratorclassselector

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Helper function to create resource.Quantity
func mustParseQuantity(value string) resource.Quantity {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		panic(err)
	}
	return q
}

func TestNewDefaultAcceleratorFetcher(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a fake client
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create fetcher
	fetcher := NewDefaultAcceleratorFetcher(c)

	// Verify fetcher is not nil and implements AcceleratorFetcher interface
	g.Expect(fetcher).NotTo(gomega.BeNil())

	// Verify it implements the interface
	var _ AcceleratorFetcher = fetcher
}

func TestDefaultAcceleratorFetcher_FetchAcceleratorClasses(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                            string
		clusterAcceleratorClasses       []v1beta1.AcceleratorClass
		expectedClusterAcceleratorCount int
		expectError                     bool
	}{
		{
			name:                            "No accelerator classes",
			clusterAcceleratorClasses:       []v1beta1.AcceleratorClass{},
			expectedClusterAcceleratorCount: 0,
			expectError:                     false,
		},
		{
			name: "Single cluster-scoped accelerator class",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-h100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-h100",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
			},
			expectedClusterAcceleratorCount: 1,
			expectError:                     false,
		},
		{
			name: "Multiple cluster-scoped accelerator classes",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-h100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-h100",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-a100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-a100",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "amd-mi300x",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "amd-mi300x",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
			},
			expectedClusterAcceleratorCount: 3,
			expectError:                     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme
			scheme := runtime.NewScheme()
			g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

			// Create objects for fake client
			objects := make([]client.Object, 0)
			for i := range tt.clusterAcceleratorClasses {
				objects = append(objects, &tt.clusterAcceleratorClasses[i])
			}

			// Create fake client
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create fetcher
			fetcher := NewDefaultAcceleratorFetcher(c)

			// Fetch accelerator classes
			collection, err := fetcher.FetchAcceleratorClasses(context.TODO())

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(collection).NotTo(gomega.BeNil())
				g.Expect(collection.ClusterAcceleratorClasses).To(gomega.HaveLen(tt.expectedClusterAcceleratorCount))

				// Verify the accelerator classes are correct
				if tt.expectedClusterAcceleratorCount > 0 {
					// Create a map of expected classes by name
					expectedByName := make(map[string]v1beta1.AcceleratorClass)
					for _, expected := range tt.clusterAcceleratorClasses {
						expectedByName[expected.Name] = expected
					}

					// Verify each returned class matches expected
					for _, actual := range collection.ClusterAcceleratorClasses {
						expected, found := expectedByName[actual.Name]
						g.Expect(found).To(gomega.BeTrue(), "unexpected accelerator class: %s", actual.Name)
						g.Expect(actual.Spec.Discovery.NodeSelector).To(gomega.Equal(expected.Spec.Discovery.NodeSelector))
					}
				}
			}
		})
	}
}

func TestDefaultAcceleratorFetcher_GetAcceleratorClass(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                      string
		acceleratorClassName      string
		clusterAcceleratorClasses []v1beta1.AcceleratorClass
		expectFound               bool
		expectClusterScoped       bool
		expectError               bool
		validateSpec              func(*testing.T, *v1beta1.AcceleratorClassSpec)
	}{
		{
			name:                 "Cluster-scoped accelerator class found",
			acceleratorClassName: "nvidia-h100",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-h100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-h100",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
						Resources: []v1beta1.AcceleratorResource{
							{
								Name:     "nvidia.com/gpu",
								Quantity: mustParseQuantity("1"),
							},
						},
					},
				},
			},
			expectFound:         true,
			expectClusterScoped: true,
			expectError:         false,
			validateSpec: func(t *testing.T, spec *v1beta1.AcceleratorClassSpec) {
				g.Expect(spec).NotTo(gomega.BeNil())
				g.Expect(spec.Discovery.NodeSelector).To(gomega.HaveKeyWithValue("accelerator", "nvidia-h100"))
				g.Expect(spec.Resources).To(gomega.HaveLen(1))
				g.Expect(spec.Resources[0].Name).To(gomega.Equal("nvidia.com/gpu"))
			},
		},
		{
			name:                 "Accelerator class not found",
			acceleratorClassName: "non-existent",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-h100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery:    v1beta1.AcceleratorDiscovery{},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
			},
			expectFound:         false,
			expectClusterScoped: false,
			expectError:         true,
			validateSpec: func(t *testing.T, spec *v1beta1.AcceleratorClassSpec) {
				g.Expect(spec).To(gomega.BeNil())
			},
		},
		{
			name:                      "No accelerator classes in cluster",
			acceleratorClassName:      "nvidia-a100",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{},
			expectFound:               false,
			expectClusterScoped:       false,
			expectError:               true,
			validateSpec: func(t *testing.T, spec *v1beta1.AcceleratorClassSpec) {
				g.Expect(spec).To(gomega.BeNil())
			},
		},
		{
			name:                 "Multiple accelerator classes - find specific one",
			acceleratorClassName: "amd-mi300x",
			clusterAcceleratorClasses: []v1beta1.AcceleratorClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nvidia-h100",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "nvidia-h100",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "amd-mi300x",
					},
					Spec: v1beta1.AcceleratorClassSpec{
						Discovery: v1beta1.AcceleratorDiscovery{
							NodeSelector: map[string]string{
								"accelerator": "amd-mi300x",
							},
						},
						Capabilities: v1beta1.AcceleratorCapabilities{},
						Resources: []v1beta1.AcceleratorResource{
							{
								Name:     "amd.com/gpu",
								Quantity: mustParseQuantity("1"),
							},
						},
					},
				},
			},
			expectFound:         true,
			expectClusterScoped: true,
			expectError:         false,
			validateSpec: func(t *testing.T, spec *v1beta1.AcceleratorClassSpec) {
				g.Expect(spec).NotTo(gomega.BeNil())
				g.Expect(spec.Discovery.NodeSelector).To(gomega.HaveKeyWithValue("accelerator", "amd-mi300x"))
				g.Expect(spec.Resources).To(gomega.HaveLen(1))
				g.Expect(spec.Resources[0].Name).To(gomega.Equal("amd.com/gpu"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheme
			scheme := runtime.NewScheme()
			g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

			// Create objects for fake client
			objects := make([]client.Object, 0)
			for i := range tt.clusterAcceleratorClasses {
				objects = append(objects, &tt.clusterAcceleratorClasses[i])
			}

			// Create fake client
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			// Create fetcher
			fetcher := NewDefaultAcceleratorFetcher(c)

			// Get accelerator class
			spec, isClusterScoped, err := fetcher.GetAcceleratorClass(context.TODO(), tt.acceleratorClassName)

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
				// Verify error is AcceleratorNotFoundError
				var notFoundErr *AcceleratorNotFoundError
				g.Expect(err).To(gomega.BeAssignableToTypeOf(notFoundErr))
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			if tt.expectFound {
				g.Expect(spec).NotTo(gomega.BeNil())
				g.Expect(isClusterScoped).To(gomega.Equal(tt.expectClusterScoped))
			} else {
				g.Expect(spec).To(gomega.BeNil())
			}

			// Run validation function if provided
			if tt.validateSpec != nil {
				tt.validateSpec(t, spec)
			}
		})
	}
}

func TestDefaultAcceleratorFetcher_GetAcceleratorClass_ErrorType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	// Create fake client with no accelerator classes
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create fetcher
	fetcher := NewDefaultAcceleratorFetcher(c)

	// Try to get non-existent accelerator class
	spec, isClusterScoped, err := fetcher.GetAcceleratorClass(context.TODO(), "non-existent")

	// Verify error
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(spec).To(gomega.BeNil())
	g.Expect(isClusterScoped).To(gomega.BeFalse())

	// Verify error type and message
	var notFoundErr *AcceleratorNotFoundError
	g.Expect(err).To(gomega.BeAssignableToTypeOf(notFoundErr))

	acceleratorErr, ok := err.(*AcceleratorNotFoundError)
	g.Expect(ok).To(gomega.BeTrue())
	g.Expect(acceleratorErr.AcceleratorClassName).To(gomega.Equal("non-existent"))
	g.Expect(acceleratorErr.Error()).To(gomega.ContainSubstring("accelerator class non-existent not found at cluster scope"))
}

func TestAcceleratorCollection(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Test empty collection
	emptyCollection := &AcceleratorCollection{}
	g.Expect(emptyCollection.ClusterAcceleratorClasses).To(gomega.BeEmpty())

	// Test collection with data
	collection := &AcceleratorCollection{
		ClusterAcceleratorClasses: []v1beta1.AcceleratorClass{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nvidia-h100",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nvidia-a100",
				},
			},
		},
	}
	g.Expect(collection.ClusterAcceleratorClasses).To(gomega.HaveLen(2))
	g.Expect(collection.ClusterAcceleratorClasses[0].Name).To(gomega.Equal("nvidia-h100"))
	g.Expect(collection.ClusterAcceleratorClasses[1].Name).To(gomega.Equal("nvidia-a100"))
}
