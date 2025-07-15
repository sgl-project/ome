package runtimeselector

import (
	"context"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFetchRuntimes(t *testing.T) {
	ctx := context.Background()

	// Create test data
	now := metav1.Now()
	earlier := metav1.NewTime(now.Add(-1 * time.Hour))

	runtimes := []v1beta1.ServingRuntime{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "runtime-a",
				Namespace:         "default",
				CreationTimestamp: now,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "runtime-b",
				Namespace:         "default",
				CreationTimestamp: earlier,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "runtime-c",
				Namespace:         "other",
				CreationTimestamp: now,
			},
		},
	}

	clusterRuntimes := []v1beta1.ClusterServingRuntime{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "cluster-runtime-a",
				CreationTimestamp: now,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "cluster-runtime-b",
				CreationTimestamp: earlier,
			},
		},
	}

	// Create fake client with test data
	fakeClient := createFakeClient()
	for _, rt := range runtimes {
		assert.NoError(t, fakeClient.Create(ctx, &rt))
	}
	for _, rt := range clusterRuntimes {
		assert.NoError(t, fakeClient.Create(ctx, &rt))
	}

	// Create fetcher
	fetcher := NewDefaultRuntimeFetcher(fakeClient)

	// Test fetching from a specific namespace
	collection, err := fetcher.FetchRuntimes(ctx, "default")
	assert.NoError(t, err)
	assert.NotNil(t, collection)

	// Verify namespace runtimes (should only get "default" namespace)
	assert.Len(t, collection.NamespaceRuntimes, 2)
	// Verify sorting - newer first
	assert.Equal(t, "runtime-a", collection.NamespaceRuntimes[0].Name)
	assert.Equal(t, "runtime-b", collection.NamespaceRuntimes[1].Name)

	// Verify cluster runtimes (should get all)
	assert.Len(t, collection.ClusterRuntimes, 2)
	// Verify sorting - newer first
	assert.Equal(t, "cluster-runtime-a", collection.ClusterRuntimes[0].Name)
	assert.Equal(t, "cluster-runtime-b", collection.ClusterRuntimes[1].Name)
}

func TestGetRuntime(t *testing.T) {
	ctx := context.Background()

	// Create test data
	namespaceRuntime := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-runtime",
			Namespace: "default",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name: "pytorch",
					},
				},
			},
		},
	}

	clusterRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name: "tensorflow",
					},
				},
			},
		},
	}

	// Create fake client with test data
	fakeClient := createFakeClient()
	assert.NoError(t, fakeClient.Create(ctx, namespaceRuntime))
	assert.NoError(t, fakeClient.Create(ctx, clusterRuntime))

	// Create fetcher
	fetcher := NewDefaultRuntimeFetcher(fakeClient)

	tests := []struct {
		name          string
		runtimeName   string
		namespace     string
		expectFound   bool
		expectCluster bool
		expectError   bool
		errorType     error
	}{
		{
			name:          "find namespace runtime",
			runtimeName:   "namespace-runtime",
			namespace:     "default",
			expectFound:   true,
			expectCluster: false,
		},
		{
			name:          "find cluster runtime",
			runtimeName:   "cluster-runtime",
			namespace:     "default",
			expectFound:   true,
			expectCluster: true,
		},
		{
			name:        "runtime not found",
			runtimeName: "non-existent",
			namespace:   "default",
			expectFound: false,
			expectError: true,
		},
		{
			name:        "namespace runtime not in different namespace",
			runtimeName: "namespace-runtime",
			namespace:   "other",
			expectFound: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, isCluster, err := fetcher.GetRuntime(ctx, tt.runtimeName, tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, IsRuntimeNotFoundError(err))
			} else {
				assert.NoError(t, err)
			}

			if tt.expectFound {
				assert.NotNil(t, spec)
				assert.Equal(t, tt.expectCluster, isCluster)
			} else {
				assert.Nil(t, spec)
			}
		})
	}
}

func TestSortingWithSameTimestamp(t *testing.T) {
	// Test that when timestamps are equal, sorting is by name
	sameTime := metav1.Now()

	runtimes := &v1beta1.ServingRuntimeList{
		Items: []v1beta1.ServingRuntime{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "zebra",
					CreationTimestamp: sameTime,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "alpha",
					CreationTimestamp: sameTime,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "beta",
					CreationTimestamp: sameTime,
				},
			},
		},
	}

	sortServingRuntimeList(runtimes)

	// Should be sorted alphabetically when timestamps are equal
	assert.Equal(t, "alpha", runtimes.Items[0].Name)
	assert.Equal(t, "beta", runtimes.Items[1].Name)
	assert.Equal(t, "zebra", runtimes.Items[2].Name)
}
