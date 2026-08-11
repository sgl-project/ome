// This file is copied and adapted from kubernetes-sigs/kueue
// (pkg/controller/admissionchecks/multikueue/remote_client.go), Apache-2.0.
// Synced from kueue commit ab2dff32b (2026-06-20); re-diff against upstream
// before editing or extending.
// It provides a controller-runtime client that selectively caches chosen
// kinds via a background informer cache, used by the WorkloadCluster
// connection Manager.

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workloadcluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// cacheIndexOption defines a field to index on the cache before starting
type CacheIndexOption struct {
	Object       client.Object
	Field        string
	ExtractValue client.IndexerFunc
}

// SelectivelyCachingClient embeds standard WithWatch and adds direct event handler
// registration capabilities for cached informers.
type SelectivelyCachingClient interface {
	client.WithWatch
	// AddCacheEventHandler registers resource event handlers on the cache's underlying informer.
	// Returns an error if the object kind is not cached.
	AddCacheEventHandler(ctx context.Context, obj client.Object, handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error)
}

type neverCachingClient struct {
	client.WithWatch
}

func (c *neverCachingClient) AddCacheEventHandler(ctx context.Context, obj client.Object, handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, errors.New("NeverCachingClient does not support watch event handlers")
}

// NewNeverCachingClient just wraps around a client. Useful for unit tests.
func NewNeverCachingClient(fakeClient client.WithWatch) SelectivelyCachingClient {
	return &neverCachingClient{
		WithWatch: fakeClient,
	}
}

type selectivelyCachingClient struct {
	client.WithWatch
	cachedReader client.Reader
	cache        cache.Cache
	scheme       *runtime.Scheme
	cachedKinds  sets.Set[schema.GroupKind]
	synced       *atomic.Bool
}

func (c *selectivelyCachingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}
	gk := gvk.GroupKind()
	if c.cachedKinds.Has(gk) && (c.synced.Load() || c.cache.WaitForCacheSync(ctx)) {
		return c.cachedReader.Get(ctx, key, obj, opts...)
	}
	return c.WithWatch.Get(ctx, key, obj, opts...)
}

func (c *selectivelyCachingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := apiutil.GVKForObject(list, c.scheme)
	if err != nil {
		return err
	}
	gk := gvk.GroupKind()
	gk.Kind = strings.TrimSuffix(gk.Kind, "List")
	if c.cachedKinds.Has(gk) && (c.synced.Load() || c.cache.WaitForCacheSync(ctx)) {
		return c.cachedReader.List(ctx, list, opts...)
	}
	return c.WithWatch.List(ctx, list, opts...)
}

func (c *selectivelyCachingClient) AddCacheEventHandler(ctx context.Context, obj client.Object, handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return nil, err
	}
	gk := gvk.GroupKind()
	if !c.cachedKinds.Has(gk) {
		return nil, fmt.Errorf("kind %s is not cached", gvk.String())
	}
	inf, err := c.cache.GetInformer(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("getting informer for caching: %w", err)
	}
	return inf.AddEventHandler(handler)
}

// podGroupKind is the GroupKind the remote cache must never hold: across many
// large clusters the Pod informer alone would dwarf everything the placer reads
// (aggregated derived-ISVC status, not remote Pods).
var podGroupKind = corev1.SchemeGroupVersion.WithKind("Pod").GroupKind()

// NewSelectivelyCachingClient constructs the background cache, registers indexes,
// starts it in the background, and returns the client.
//
// defaultSelector scopes every cached kind to the caller-supplied label selector
// (mirrors the manager cache's placement-origin scoping); pass nil to cache
// unfiltered. There is intentionally no in-code default selector — the value is
// the caller's to supply, per the repo's no-magic-values rule. Caching Pods is
// rejected outright: the remote cache must never LIST+WATCH cluster-wide Pods.
func NewSelectivelyCachingClient(
	ctx context.Context,
	restConfig *rest.Config,
	directClient client.WithWatch,
	scheme *runtime.Scheme,
	cachedKinds sets.Set[schema.GroupKind],
	indexes []CacheIndexOption,
	defaultSelector labels.Selector,
) (SelectivelyCachingClient, error) {
	if cachedKinds.Has(podGroupKind) {
		return nil, fmt.Errorf("refusing to cache kind %s: the remote cache must never hold cluster-wide Pods", podGroupKind.String())
	}

	cacheOpts := cache.Options{
		Scheme: scheme,
		// Strip managedFields from every cached object (typically a large
		// fraction of each object's serialized size and never read here),
		// mirroring the manager cache in cmd/manager/main.go.
		DefaultTransform: cache.TransformStripManagedFields(),
	}
	if defaultSelector != nil {
		cacheOpts.DefaultLabelSelector = defaultSelector
	}

	remoteCache, err := cache.New(restConfig, cacheOpts)
	if err != nil {
		return nil, fmt.Errorf("creating remote cache: %w", err)
	}

	for _, opt := range indexes {
		err = remoteCache.IndexField(ctx, opt.Object, opt.Field, opt.ExtractValue)
		if err != nil {
			return nil, fmt.Errorf("registering index for %s on %s: %w", opt.Field, opt.Object.GetObjectKind().GroupVersionKind().Kind, err)
		}
	}

	var synced atomic.Bool

	go func() {
		if err := remoteCache.Start(ctx); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "Remote cache execution failed")
		}
	}()

	go func() {
		if remoteCache.WaitForCacheSync(ctx) {
			synced.Store(true)
			ctrl.LoggerFrom(ctx).V(2).Info("Remote cache successfully synchronized")
		}
	}()

	cachedClient := &selectivelyCachingClient{
		WithWatch:    directClient,
		cachedReader: remoteCache,
		cache:        remoteCache,
		scheme:       scheme,
		cachedKinds:  cachedKinds,
		synced:       &synced,
	}

	return cachedClient, nil
}
