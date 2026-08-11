package workloadcluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNeverCachingClient_NoCacheHandler(t *testing.T) {
	fc := fake.NewClientBuilder().Build()
	c := NewNeverCachingClient(fc)
	if _, err := c.AddCacheEventHandler(context.Background(), &corev1.Pod{}, nil); err == nil {
		t.Errorf("AddCacheEventHandler on NeverCachingClient: got nil error, want error")
	}
}

// NewSelectivelyCachingClient must refuse to cache Pods before any cluster
// contact: a cluster-wide Pod informer is exactly the OOM the remote cache
// guards against. The guard returns before cache.New, so a nil restConfig
// proves it short-circuits.
func TestNewSelectivelyCachingClient_RejectsPodKind(t *testing.T) {
	podGK := schema.GroupKind{Group: "", Kind: "Pod"}
	_, err := NewSelectivelyCachingClient(
		context.Background(),
		(*rest.Config)(nil),
		fake.NewClientBuilder().Build(),
		nil,
		sets.New(podGK),
		nil,
		nil,
	)
	if err == nil {
		t.Fatalf("NewSelectivelyCachingClient with Pod in cachedKinds: got nil error, want rejection")
	}
}
