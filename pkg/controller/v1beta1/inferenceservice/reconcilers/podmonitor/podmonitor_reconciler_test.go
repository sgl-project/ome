package podmonitor

import (
	"context"
	"errors"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/constants"
)

type countingClient struct {
	client.Client
	createCalls int
	updateCalls int
	getCalls    int

	failGet    error
	failCreate error
	failUpdate error
}

func (c *countingClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	c.getCalls++
	if c.failGet != nil {
		return c.failGet
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *countingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c.createCalls++
	if c.failCreate != nil {
		return c.failCreate
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *countingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	if c.failUpdate != nil {
		return c.failUpdate
	}
	return c.Client.Update(ctx, obj, opts...)
}

func buildScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, monitoringv1.AddToScheme(scheme))
	return scheme
}

func TestMetricsPortName(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  *corev1.PodSpec
		expected string
	}{
		{
			name:     "nil podSpec falls back to http",
			podSpec:  nil,
			expected: "http",
		},
		{
			name:     "no containers falls back to http",
			podSpec:  &corev1.PodSpec{},
			expected: "http",
		},
		{
			name: "no ports falls back to http",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main"}},
			},
			expected: "http",
		},
		{
			name: "single http port returns http",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "main",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 30000, Name: "http"},
					},
				}},
			},
			expected: "http",
		},
		{
			name: "metrics port preferred over http",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "main",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 30080, Name: "http"},
						{ContainerPort: 9090, Name: "metrics"},
					},
				}},
			},
			expected: "metrics",
		},
		{
			name: "metrics port preferred even if listed first",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "main",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 9090, Name: "metrics"},
						{ContainerPort: 30080, Name: "http"},
					},
				}},
			},
			expected: "metrics",
		},
		{
			name: "unnamed port falls back to http",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "main",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080},
					},
				}},
			},
			expected: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, metricsPortName(tt.podSpec))
		})
	}
}

func TestPodMonitorReconciler_Create(t *testing.T) {
	scheme := buildScheme(t)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cc := &countingClient{Client: baseClient}

	meta := metav1.ObjectMeta{Name: "my-isvc-engine", Namespace: "prod"}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "main",
			Ports: []corev1.ContainerPort{
				{ContainerPort: 30000, Name: "http"},
			},
		}},
	}

	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	got := &monitoringv1.PodMonitor{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)

	assert.Equal(t, meta.Name, got.Name)
	assert.Equal(t, meta.Namespace, got.Namespace)
	assert.Equal(t, map[string]string{"app": meta.Name}, got.Labels)
	assert.Equal(t, []string{"prod"}, got.Spec.NamespaceSelector.MatchNames)
	assert.Equal(t, map[string]string{"app": meta.Name}, got.Spec.Selector.MatchLabels)
	require.Len(t, got.Spec.PodMetricsEndpoints, 1)
	assert.Equal(t, "http", *got.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, "/metrics", got.Spec.PodMetricsEndpoints[0].Path)
	assert.Equal(t, monitoringv1.Duration("10s"), got.Spec.PodMetricsEndpoints[0].Interval)
	assert.Equal(t, 1, cc.createCalls)
	assert.Equal(t, 0, cc.updateCalls)
}

func TestPodMonitorReconciler_Create_RouterWithMetricsPort(t *testing.T) {
	scheme := buildScheme(t)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cc := &countingClient{Client: baseClient}

	meta := metav1.ObjectMeta{Name: "my-isvc-router", Namespace: "prod"}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "main",
			Ports: []corev1.ContainerPort{
				{ContainerPort: 30080, Name: "http"},
				{ContainerPort: 9090, Name: "metrics"},
			},
		}},
	}

	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	got := &monitoringv1.PodMonitor{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)

	require.Len(t, got.Spec.PodMetricsEndpoints, 1)
	assert.Equal(t, "metrics", *got.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, 1, cc.createCalls)
}

// TestPodMonitorReconciler_Create_ExtraEndpoints mirrors an HTTP-router
// pod: a "metrics" port scraped at /metrics plus a second "http" port whose
// /engine_metrics surface is declared via the extra-endpoints annotation. The
// built PodMonitor must carry the default endpoint first, then the extra.
func TestPodMonitorReconciler_Create_ExtraEndpoints(t *testing.T) {
	scheme := buildScheme(t)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cc := &countingClient{Client: baseClient}

	meta := metav1.ObjectMeta{
		Name:      "my-isvc-router",
		Namespace: "prod",
		Annotations: map[string]string{
			constants.ExtraPodMetricsEndpointsAnnotationKey: "http:/engine_metrics",
		},
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "main",
			Ports: []corev1.ContainerPort{
				{ContainerPort: 29000, Name: "metrics"},
				{ContainerPort: 8000, Name: "http"},
			},
		}},
	}

	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	got := &monitoringv1.PodMonitor{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)

	require.Len(t, got.Spec.PodMetricsEndpoints, 2)
	// Default /metrics endpoint stays first.
	assert.Equal(t, "metrics", *got.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, "/metrics", got.Spec.PodMetricsEndpoints[0].Path)
	assert.Equal(t, monitoringv1.Duration("10s"), got.Spec.PodMetricsEndpoints[0].Interval)
	// Annotation-declared endpoint is appended.
	assert.Equal(t, "http", *got.Spec.PodMetricsEndpoints[1].Port)
	assert.Equal(t, "/engine_metrics", got.Spec.PodMetricsEndpoints[1].Path)
	assert.Equal(t, monitoringv1.Duration("10s"), got.Spec.PodMetricsEndpoints[1].Interval)
	assert.Equal(t, 1, cc.createCalls)
}

func TestPodMonitorReconciler_Update_WhenSpecDiffers(t *testing.T) {
	scheme := buildScheme(t)
	meta := metav1.ObjectMeta{Name: "svc-update", Namespace: "ns-update"}

	oldPort := "old-port"
	existing := &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:            meta.Name,
			Namespace:       meta.Namespace,
			ResourceVersion: "123",
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": meta.Name},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{meta.Namespace},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{Port: &oldPort, Path: "/metrics", Interval: "30s"},
			},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cc := &countingClient{Client: baseClient}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "main",
			Ports: []corev1.ContainerPort{{ContainerPort: 30000, Name: "http"}},
		}},
	}
	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	got := &monitoringv1.PodMonitor{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)
	assert.Equal(t, "http", *got.Spec.PodMetricsEndpoints[0].Port)
	assert.Equal(t, monitoringv1.Duration("10s"), got.Spec.PodMetricsEndpoints[0].Interval)
	assert.Equal(t, 1, cc.updateCalls)
}

func TestPodMonitorReconciler_NoOp_WhenEqual(t *testing.T) {
	scheme := buildScheme(t)
	meta := metav1.ObjectMeta{Name: "svc-eq", Namespace: "ns-eq"}

	httpPort := "http"
	existing := &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    map[string]string{"app": meta.Name},
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": meta.Name},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{meta.Namespace},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{Port: &httpPort, Path: "/metrics", Interval: "10s"},
			},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cc := &countingClient{Client: baseClient}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "main",
			Ports: []corev1.ContainerPort{{ContainerPort: 30000, Name: "http"}},
		}},
	}
	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	assert.Equal(t, 0, cc.updateCalls)
	assert.Equal(t, 0, cc.createCalls)
}

func TestPodMonitorReconciler_NoOp_WhenOperatorDefaulted(t *testing.T) {
	scheme := buildScheme(t)
	meta := metav1.ObjectMeta{Name: "svc-defaulted", Namespace: "ns-defaulted"}

	httpPort := "http"
	// Mimic what the Prometheus operator stores: the same managed fields OME
	// sets, plus operator defaults (scheme/scrapeTimeout/honorLabels).
	existing := &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    map[string]string{"app": meta.Name},
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": meta.Name},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{meta.Namespace},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
				Port:          &httpPort,
				Path:          "/metrics",
				Interval:      "10s",
				Scheme:        "http",
				ScrapeTimeout: "10s",
				HonorLabels:   true,
			}},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cc := &countingClient{Client: baseClient}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "main",
			Ports: []corev1.ContainerPort{{ContainerPort: 30000, Name: "http"}},
		}},
	}
	r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Steady state: no PUT despite the operator-defaulted endpoint fields.
	assert.Equal(t, 0, cc.updateCalls)
	assert.Equal(t, 0, cc.createCalls)
}

func TestPodMonitorReconciler_ErrorPaths(t *testing.T) {
	scheme := buildScheme(t)
	meta := metav1.ObjectMeta{Name: "svc-err", Namespace: "ns-err"}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "main",
			Ports: []corev1.ContainerPort{{ContainerPort: 30000, Name: "http"}},
		}},
	}

	t.Run("get error", func(t *testing.T) {
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		cc := &countingClient{Client: baseClient, failGet: errors.New("get failure")}
		r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
	})

	t.Run("create error", func(t *testing.T) {
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		cc := &countingClient{Client: baseClient, failCreate: errors.New("create failure")}
		r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
		assert.Equal(t, 1, cc.createCalls)
	})

	t.Run("update error", func(t *testing.T) {
		oldPort := "old"
		existing := &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: meta.Name, Namespace: meta.Namespace},
			Spec: monitoringv1.PodMonitorSpec{
				Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": meta.Name}},
				NamespaceSelector:   monitoringv1.NamespaceSelector{MatchNames: []string{meta.Namespace}},
				PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &oldPort, Path: "/old", Interval: "30s"}},
			},
		}
		baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
		cc := &countingClient{Client: baseClient, failUpdate: errors.New("update failure")}
		r := NewPodMonitorReconciler(cc, scheme, meta, podSpec)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
		assert.Equal(t, 1, cc.updateCalls)
	})
}

func Test_semanticPodMonitorEquals(t *testing.T) {
	httpPort := "http"
	metricsPort := "metrics"
	selector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "svc"}}
	nsSelector := monitoringv1.NamespaceSelector{MatchNames: []string{"ns"}}

	base := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            selector,
			NamespaceSelector:   nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &httpPort, Path: "/metrics", Interval: "10s"}},
		},
	}

	same := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            selector,
			NamespaceSelector:   nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &httpPort, Path: "/metrics", Interval: "10s"}},
		},
	}
	assert.True(t, semanticPodMonitorEquals(base, same))

	// The Prometheus operator defaults scheme/scrapeTimeout/honorLabels/etc.
	// onto the stored object. The live endpoint is richer than the minimal one
	// OME builds, but it must still be treated as equal so we don't issue a
	// no-op Update every reconcile.
	defaulted := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:          selector,
			NamespaceSelector: nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
				Port:          &httpPort,
				Path:          "/metrics",
				Interval:      "10s",
				Scheme:        "http",
				ScrapeTimeout: "10s",
				HonorLabels:   true,
			}},
		},
	}
	assert.True(t, semanticPodMonitorEquals(base, defaulted))

	diffPort := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            selector,
			NamespaceSelector:   nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &metricsPort, Path: "/metrics", Interval: "10s"}},
		},
	}
	assert.False(t, semanticPodMonitorEquals(base, diffPort))

	diffInterval := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            selector,
			NamespaceSelector:   nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &httpPort, Path: "/metrics", Interval: "30s"}},
		},
	}
	assert.False(t, semanticPodMonitorEquals(base, diffInterval))

	diffSelector := &monitoringv1.PodMonitor{
		Spec: monitoringv1.PodMonitorSpec{
			Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": "other"}},
			NamespaceSelector:   nsSelector,
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: &httpPort, Path: "/metrics", Interval: "10s"}},
		},
	}
	assert.False(t, semanticPodMonitorEquals(base, diffSelector))
}
