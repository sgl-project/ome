package service

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestBuildServiceFiltersAnnotations(t *testing.T) {
	scenarios := map[string]struct {
		componentMeta         metav1.ObjectMeta
		expectedAnnotations   map[string]string
		unexpectedAnnotations []string
	}{
		"FilterGrafanaAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"k8s.grafana.com/scrape": "true",
					"k8s.grafana.com/port":   "8080",
					"ome.io/base-model-name": "test-model",
					"ome.io/service-type":    "ClusterIP",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/base-model-name": "test-model",
				"ome.io/service-type":    "ClusterIP",
			},
			unexpectedAnnotations: []string{
				"k8s.grafana.com/scrape",
				"k8s.grafana.com/port",
			},
		},
		"FilterLokiAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"loki.grafana.com/scrape":     "true",
					"loki.grafana.com/log-format": "json",
					"ome.io/serving-runtime":      "test-runtime",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/serving-runtime": "test-runtime",
			},
			unexpectedAnnotations: []string{
				"loki.grafana.com/scrape",
				"loki.grafana.com/log-format",
			},
		},
		"FilterPrometheusAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"prometheus.io/scrape":   "true",
					"prometheus.io/port":     "8080",
					"prometheus.io/path":     "/metrics",
					"ome.io/serving-runtime": "test-runtime",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/serving-runtime": "test-runtime",
			},
			unexpectedAnnotations: []string{
				"prometheus.io/scrape",
				"prometheus.io/port",
				"prometheus.io/path",
			},
		},
		"FilterNetworkingGKEAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"networking.gke.io/default-interface": "eth0",
					"networking.gke.io/interfaces":        "[{\"interfaceName\":\"eth0\"}]",
					"ome.io/deploymentMode":               "RawDeployment",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/deploymentMode": "RawDeployment",
			},
			unexpectedAnnotations: []string{
				"networking.gke.io/default-interface",
				"networking.gke.io/interfaces",
			},
		},
		"FilterInjectionAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					constants.ModelInitInjectionKey:        "true",
					constants.FineTunedAdapterInjectionKey: "weight-name",
					constants.ServingSidecarInjectionKey:   "true",
					"ome.io/base-model-name":               "test-model",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/base-model-name": "test-model",
			},
			unexpectedAnnotations: []string{
				constants.ModelInitInjectionKey,
				constants.FineTunedAdapterInjectionKey,
				constants.ServingSidecarInjectionKey,
			},
		},
		"FilterMixedAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"k8s.grafana.com/scrape":        "true",
					"networking.gke.io/interfaces":  "[...]",
					constants.ModelInitInjectionKey: "true",
					"rdma.ome.io/auto-inject":       "true",
					"ome.io/base-model-name":        "test-model",
					"ome.io/service-type":           "ClusterIP",
					"meta.helm.sh/release-name":     "test",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/base-model-name":    "test-model",
				"ome.io/service-type":       "ClusterIP",
				"meta.helm.sh/release-name": "test",
			},
			unexpectedAnnotations: []string{
				"k8s.grafana.com/scrape",
				"networking.gke.io/interfaces",
				constants.ModelInitInjectionKey,
				"rdma.ome.io/auto-inject",
			},
		},
		"PreserveAllNonPodOnlyAnnotations": {
			componentMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				Annotations: map[string]string{
					"ome.io/deploymentMode":     "RawDeployment",
					"ome.io/service-type":       "ClusterIP",
					"ome.io/load-balancer-ip":   "10.0.0.1",
					"custom.annotation/key":     "value",
					"meta.helm.sh/release-name": "test",
				},
			},
			expectedAnnotations: map[string]string{
				"ome.io/deploymentMode":     "RawDeployment",
				"ome.io/service-type":       "ClusterIP",
				"ome.io/load-balancer-ip":   "10.0.0.1",
				"custom.annotation/key":     "value",
				"meta.helm.sh/release-name": "test",
			},
			unexpectedAnnotations: []string{},
		},
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "test-container",
			Ports: []corev1.ContainerPort{{
				Name:          "http",
				ContainerPort: 8080,
			}},
		}},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			service := buildService(scenario.componentMeta, nil, podSpec, nil)

			// Check that expected annotations are present
			for key, expectedValue := range scenario.expectedAnnotations {
				if actualValue, ok := service.Annotations[key]; !ok {
					t.Errorf("Expected annotation %q to be present on Service", key)
				} else if actualValue != expectedValue {
					t.Errorf("Annotation %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}

			// Check that unexpected (pod-only) annotations are NOT present
			for _, key := range scenario.unexpectedAnnotations {
				if _, ok := service.Annotations[key]; ok {
					t.Errorf("Unexpected pod-only annotation %q found on Service", key)
				}
			}
		})
	}
}

func TestBuildServicePreservesOtherMetadata(t *testing.T) {
	componentMeta := metav1.ObjectMeta{
		Name:      "test-service",
		Namespace: "test-namespace",
		Labels: map[string]string{
			"app":     "test-app",
			"version": "v1",
		},
		Annotations: map[string]string{
			"k8s.grafana.com/scrape": "true",
			"ome.io/service-type":    "ClusterIP",
		},
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "test-container",
			Ports: []corev1.ContainerPort{{
				Name:          "http",
				ContainerPort: 8080,
			}},
		}},
	}

	service := buildService(componentMeta, nil, podSpec, nil)

	// Name and Namespace should be preserved
	if service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got %q", service.Name)
	}
	if service.Namespace != "test-namespace" {
		t.Errorf("Expected service namespace to be 'test-namespace', got %q", service.Namespace)
	}

	// Labels should be preserved
	expectedLabels := map[string]string{
		"app":     "test-app",
		"version": "v1",
	}
	if diff := cmp.Diff(expectedLabels, service.Labels); diff != "" {
		t.Errorf("Labels mismatch (-want +got):\n%s", diff)
	}

	// Pod-only annotation should be filtered
	if _, ok := service.Annotations["k8s.grafana.com/scrape"]; ok {
		t.Error("Pod-only annotation should have been filtered")
	}

	// Non-pod-only annotation should be preserved
	if service.Annotations["ome.io/service-type"] != "ClusterIP" {
		t.Error("Non-pod-only annotation should have been preserved")
	}
}

func TestBuildServiceWithNilAnnotations(t *testing.T) {
	componentMeta := metav1.ObjectMeta{
		Name:        "test-service",
		Namespace:   "default",
		Annotations: nil,
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "test-container",
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
			}},
		}},
	}

	// Should not panic with nil annotations
	service := buildService(componentMeta, nil, podSpec, nil)

	if service == nil {
		t.Error("Expected service to be created, got nil")
	}
}

func TestBuildServiceWithEmptyAnnotations(t *testing.T) {
	componentMeta := metav1.ObjectMeta{
		Name:        "test-service",
		Namespace:   "default",
		Annotations: map[string]string{},
	}

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "test-container",
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
			}},
		}},
	}

	service := buildService(componentMeta, nil, podSpec, nil)

	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}
	if len(service.Annotations) != 0 {
		t.Errorf("Expected empty annotations, got %v", service.Annotations)
	}
}
func TestBuildServiceSetsConfiguredPortAppProtocols(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "router",
			Ports: []corev1.ContainerPort{
				{Name: "http", ContainerPort: 8000},
				{Name: "metrics", ContainerPort: 29000},
			},
		}},
	}
	componentExt := &v1beta1.ComponentExtensionSpec{
		ServicePortAppProtocols: map[string]string{
			"http": "kubernetes.io/h2c",
		},
	}

	service := buildService(metav1.ObjectMeta{Name: "test-service"}, componentExt, podSpec, nil)

	if got := service.Spec.Ports[0].AppProtocol; got == nil || *got != "kubernetes.io/h2c" {
		t.Errorf("http appProtocol: got %v, want kubernetes.io/h2c", got)
	}
	if got := service.Spec.Ports[1].AppProtocol; got != nil {
		t.Errorf("metrics appProtocol: got %q, want nil", *got)
	}
}

func TestServiceReconcilerAddsAppProtocolToExistingService(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "test-service", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.0.0.1",
			Selector:  map[string]string{"app": "test-service"},
			Ports: []corev1.ServicePort{{
				Name: "http",
				Port: 8000,
			}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	podSpec := &corev1.PodSpec{Containers: []corev1.Container{{
		Name: "router",
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: 8000,
		}},
	}}}
	componentExt := &v1beta1.ComponentExtensionSpec{
		ServicePortAppProtocols: map[string]string{"http": "kubernetes.io/h2c"},
	}
	reconciler := NewServiceReconciler(
		c,
		scheme,
		metav1.ObjectMeta{Name: "test-service", Namespace: "default"},
		componentExt,
		podSpec,
		nil,
	)

	if _, err := reconciler.Reconcile(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := &corev1.Service{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "test-service", Namespace: "default"}, got); err != nil {
		t.Fatalf("get Service after reconcile: %v", err)
	}
	if got.Spec.ClusterIP != existing.Spec.ClusterIP {
		t.Errorf("ClusterIP: got %q, want %q", got.Spec.ClusterIP, existing.Spec.ClusterIP)
	}
	if got.Spec.Ports[0].AppProtocol == nil || *got.Spec.Ports[0].AppProtocol != "kubernetes.io/h2c" {
		t.Errorf("appProtocol: got %v, want kubernetes.io/h2c", got.Spec.Ports[0].AppProtocol)
	}
}
