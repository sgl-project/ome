package runtimeselector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestFetchRuntimes(t *testing.T) {
	ctx := context.Background()

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

	fakeClient := createFakeClient()
	for _, rt := range runtimes {
		assert.NoError(t, fakeClient.Create(ctx, &rt))
	}
	for _, rt := range clusterRuntimes {
		assert.NoError(t, fakeClient.Create(ctx, &rt))
	}

	fetcher := NewDefaultRuntimeFetcher(fakeClient)

	collection, err := fetcher.FetchRuntimes(ctx, "default")
	assert.NoError(t, err)
	assert.NotNil(t, collection)

	// FetchRuntimes scopes namespace runtimes and sorts newest-first.
	assert.Len(t, collection.NamespaceRuntimes, 2)
	assert.Equal(t, "runtime-a", collection.NamespaceRuntimes[0].Name)
	assert.Equal(t, "runtime-b", collection.NamespaceRuntimes[1].Name)

	assert.Len(t, collection.ClusterRuntimes, 2)
	assert.Equal(t, "cluster-runtime-a", collection.ClusterRuntimes[0].Name)
	assert.Equal(t, "cluster-runtime-b", collection.ClusterRuntimes[1].Name)
}

func TestGetRuntime(t *testing.T) {
	ctx := context.Background()

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

	// Same-named runtimes in both scopes; the specs differ so the test can
	// tell which one a lookup resolved.
	collidingNamespaced := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "colliding-runtime",
			Namespace: "default",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{ModelFormat: &v1beta1.ModelFormat{Name: "pytorch"}},
			},
		},
	}
	collidingCluster := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "colliding-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{ModelFormat: &v1beta1.ModelFormat{Name: "tensorflow"}},
			},
		},
	}

	fakeClient := createFakeClient()
	assert.NoError(t, fakeClient.Create(ctx, namespaceRuntime))
	assert.NoError(t, fakeClient.Create(ctx, clusterRuntime))
	assert.NoError(t, fakeClient.Create(ctx, collidingNamespaced))
	assert.NoError(t, fakeClient.Create(ctx, collidingCluster))

	fetcher := NewDefaultRuntimeFetcher(fakeClient)

	tests := []struct {
		name          string
		runtimeName   string
		namespace     string
		kind          string
		expectFound   bool
		expectCluster bool
		expectFormat  string
		expectError   bool
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
		{
			name:          "no kind: namespaced wins the collision",
			runtimeName:   "colliding-runtime",
			namespace:     "default",
			expectFound:   true,
			expectCluster: false,
			expectFormat:  "pytorch",
		},
		{
			name:          "ClusterServingRuntime kind wins the collision",
			runtimeName:   "colliding-runtime",
			namespace:     "default",
			kind:          KindClusterServingRuntime,
			expectFound:   true,
			expectCluster: true,
			expectFormat:  "tensorflow",
		},
		{
			name:          "ServingRuntime kind wins the collision",
			runtimeName:   "colliding-runtime",
			namespace:     "default",
			kind:          KindServingRuntime,
			expectFound:   true,
			expectCluster: false,
			expectFormat:  "pytorch",
		},
		{
			name:          "ClusterServingRuntime kind falls back to a namespaced-only runtime",
			runtimeName:   "namespace-runtime",
			namespace:     "default",
			kind:          KindClusterServingRuntime,
			expectFound:   true,
			expectCluster: false,
		},
		{
			name:        "ServingRuntime kind never falls back to a cluster-only runtime",
			runtimeName: "cluster-runtime",
			namespace:   "default",
			kind:        KindServingRuntime,
			expectFound: false,
			expectError: true,
		},
		{
			name:          "ServingRuntime kind finds the namespaced runtime",
			runtimeName:   "namespace-runtime",
			namespace:     "default",
			kind:          KindServingRuntime,
			expectFound:   true,
			expectCluster: false,
		},
		{
			name:        "ClusterServingRuntime kind with no runtime in either scope",
			runtimeName: "non-existent",
			namespace:   "default",
			kind:        KindClusterServingRuntime,
			expectFound: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, isCluster, err := fetcher.GetRuntime(ctx, tt.runtimeName, tt.namespace, tt.kind)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, IsRuntimeNotFoundError(err))
			} else {
				assert.NoError(t, err)
			}

			if tt.expectFound {
				assert.NotNil(t, spec)
				assert.Equal(t, tt.expectCluster, isCluster)
				if tt.expectFormat != "" {
					assert.Equal(t, tt.expectFormat, spec.SupportedModelFormats[0].ModelFormat.Name)
				}
			} else {
				assert.Nil(t, spec)
			}
		})
	}
}

func TestSortingWithSameTimestamp(t *testing.T) {
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

	assert.Equal(t, "alpha", runtimes.Items[0].Name)
	assert.Equal(t, "beta", runtimes.Items[1].Name)
	assert.Equal(t, "zebra", runtimes.Items[2].Name)
}
