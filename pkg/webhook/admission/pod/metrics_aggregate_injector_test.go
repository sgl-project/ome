package pod

import (
	"strconv"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmp"

	"sigs.k8s.io/ome/pkg/constants"
)

const sklearnPrometheusPort = "8080"

func TestInjectMetricsAggregator(t *testing.T) {
	scenarios := map[string]struct {
		original *v1.Pod
		expected *v1.Pod
	}{
		"EnableMetricAggTrue": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name: "queue-proxy",
							Env: []v1.EnvVar{
								{Name: constants.ContainerPrometheusMetricsPortEnvVarKey, Value: sklearnPrometheusPort},
								{Name: constants.ContainerPrometheusMetricsPathEnvVarKey, Value: constants.DefaultPrometheusPath},
								{Name: constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey, Value: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)},
							},
							Ports: []v1.ContainerPort{
								{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"},
								{Name: constants.AggregateMetricsPortName, ContainerPort: int32(constants.QueueProxyAggregatePrometheusMetricsPort), Protocol: "TCP"},
							},
						},
					},
				},
			},
		},
		"EnableMetricAggNotSet": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "false",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
		},
		"EnableMetricAggFalse": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "false",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
		},
		"setPromAnnotationTrueWithAggTrue": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
						constants.SetPrometheusAnnotation: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation:     "true",
						constants.SetPrometheusAnnotation:     "true",
						constants.PrometheusPortAnnotationKey: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort),
						constants.PrometheusPathAnnotationKey: constants.DefaultPrometheusPath,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name: "queue-proxy",
							Env: []v1.EnvVar{
								{Name: constants.ContainerPrometheusMetricsPortEnvVarKey, Value: sklearnPrometheusPort},
								{Name: constants.ContainerPrometheusMetricsPathEnvVarKey, Value: constants.DefaultPrometheusPath},
								{Name: constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey, Value: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)},
							},
							Ports: []v1.ContainerPort{
								{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"},
								{Name: constants.AggregateMetricsPortName, ContainerPort: int32(constants.QueueProxyAggregatePrometheusMetricsPort), Protocol: "TCP"},
							},
						},
					},
				},
			},
		},
		"setPromAnnotationTrueWithAggFalse": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "false",
						constants.SetPrometheusAnnotation: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation:     "false",
						constants.SetPrometheusAnnotation:     "true",
						constants.PrometheusPortAnnotationKey: constants.DefaultPodPrometheusPort,
						constants.PrometheusPathAnnotationKey: constants.DefaultPrometheusPath,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
		},
		"SetPromAnnotationFalse": {
			original: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
						constants.SetPrometheusAnnotation: "false",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name:  "queue-proxy",
							Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
						},
					},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "default",
					Annotations: map[string]string{
						constants.EnableMetricAggregation: "true",
						constants.SetPrometheusAnnotation: "false",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "sklearn",
					},
						{
							Name: "queue-proxy",
							Env: []v1.EnvVar{
								{Name: constants.ContainerPrometheusMetricsPortEnvVarKey, Value: sklearnPrometheusPort},
								{Name: constants.ContainerPrometheusMetricsPathEnvVarKey, Value: constants.DefaultPrometheusPath},
								{Name: constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey, Value: strconv.Itoa(constants.QueueProxyAggregatePrometheusMetricsPort)},
							},
							Ports: []v1.ContainerPort{
								{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"},
								{Name: constants.AggregateMetricsPortName, ContainerPort: int32(constants.QueueProxyAggregatePrometheusMetricsPort), Protocol: "TCP"},
							},
						},
					},
				},
			},
		},
	}

	cfgMap := v1.ConfigMap{Data: map[string]string{"enableMetricAggregation": "false", "enablePrometheusScraping": "false"}}
	ma, err := newMetricsAggregator(&cfgMap)
	if err != nil {
		t.Errorf("Error creating the metrics aggregator %v", err)
	}

	for name, scenario := range scenarios {
		err := ma.InjectMetricsAggregator(scenario.original)
		if err != nil {
			return
		}
		if diff, _ := kmp.SafeDiff(scenario.expected.Spec, scenario.original.Spec); diff != "" {
			t.Errorf("Test %q unexpected result (-want +got): %v", name, diff)
		}
	}
}

// The webhook is registered with reinvocationPolicy=IfNeeded, so the
// injector can run multiple times on the same pod and must converge
// instead of stacking duplicate env vars or ports.
func TestInjectMetricsAggregatorReinvocation(t *testing.T) {
	newPod := func() *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment",
				Namespace: "default",
				Annotations: map[string]string{
					constants.EnableMetricAggregation: "true",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{Name: "sklearn"},
					{
						Name:  "queue-proxy",
						Ports: []v1.ContainerPort{{Name: "http-usermetric", ContainerPort: 9091, Protocol: "TCP"}},
					},
				},
			},
		}
	}

	queueProxy := func(t *testing.T, pod *v1.Pod) *v1.Container {
		t.Helper()
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == "queue-proxy" {
				return &pod.Spec.Containers[i]
			}
		}
		t.Fatal("queue-proxy container not found")
		return nil
	}

	envValuesByName := func(container *v1.Container, name string) []string {
		var values []string
		for _, env := range container.Env {
			if env.Name == name {
				values = append(values, env.Value)
			}
		}
		return values
	}

	ma, err := newMetricsAggregator(&v1.ConfigMap{Data: map[string]string{}})
	if err != nil {
		t.Fatalf("Error creating the metrics aggregator %v", err)
	}

	t.Run("second invocation is a no-op", func(t *testing.T) {
		pod := newPod()
		if err := ma.InjectMetricsAggregator(pod); err != nil {
			t.Fatalf("first invocation failed: %v", err)
		}
		afterFirst := pod.DeepCopy()

		if err := ma.InjectMetricsAggregator(pod); err != nil {
			t.Fatalf("second invocation failed: %v", err)
		}
		if diff, _ := kmp.SafeDiff(afterFirst.Spec, pod.Spec); diff != "" {
			t.Errorf("second invocation mutated the pod spec (-want +got): %v", diff)
		}

		qp := queueProxy(t, pod)
		for _, name := range []string{
			constants.ContainerPrometheusMetricsPortEnvVarKey,
			constants.ContainerPrometheusMetricsPathEnvVarKey,
			constants.QueueProxyAggregatePrometheusMetricsPortEnvVarKey,
		} {
			if values := envValuesByName(qp, name); len(values) != 1 {
				t.Errorf("expected exactly one %q env var, got %d (%v)", name, len(values), values)
			}
		}
		if len(qp.Ports) != 2 {
			t.Errorf("expected 2 ports after reinvocation, got %d (%v)", len(qp.Ports), qp.Ports)
		}
	})

	t.Run("annotation change between invocations updates in place", func(t *testing.T) {
		pod := newPod()
		if err := ma.InjectMetricsAggregator(pod); err != nil {
			t.Fatalf("first invocation failed: %v", err)
		}

		pod.ObjectMeta.Annotations[constants.ContainerPrometheusPortKey] = "9095"
		if err := ma.InjectMetricsAggregator(pod); err != nil {
			t.Fatalf("second invocation failed: %v", err)
		}

		qp := queueProxy(t, pod)
		values := envValuesByName(qp, constants.ContainerPrometheusMetricsPortEnvVarKey)
		if len(values) != 1 || values[0] != "9095" {
			t.Errorf("expected a single %q env var with value 9095, got %v",
				constants.ContainerPrometheusMetricsPortEnvVarKey, values)
		}
	})
}
