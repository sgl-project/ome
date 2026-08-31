package runtimepreset

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const (
	sglangEngineImage = "lmsysorg/sglang:latest"
	sglangRouterImage = "lmsysorg/sglang-router:latest"

	sglangHTTPPort      int32 = 30000
	sglangBootstrapPort int32 = 8998
	sglangRouterPort    int32 = 30080
)

func sglangPDPreset() *v1beta1.ServingRuntimeSpec {
	return &v1beta1.ServingRuntimeSpec{
		EngineConfig: &v1beta1.EngineSpec{
			Runner: sglangRunner("prefill", true),
		},
		DecoderConfig: &v1beta1.DecoderSpec{
			Runner: sglangRunner("decode", false),
		},
		RouterConfig: &v1beta1.RouterSpec{
			Runner: sglangRouterRunner(),
		},
	}
}

func sglangRunner(mode string, withBootstrap bool) *v1beta1.RunnerSpec {
	ports := []corev1.ContainerPort{{
		ContainerPort: sglangHTTPPort,
		Name:          "http",
		Protocol:      corev1.ProtocolTCP,
	}}
	cmd := []string{
		"python3", "-m", "sglang.launch_server",
		"--host", "0.0.0.0",
		"--port", "30000",
		"--model-path", "$(MODEL_PATH)",
		"--disaggregation-mode", mode,
		"--disaggregation-transfer-backend", "nixl",
	}
	if withBootstrap {
		cmd = append(cmd, "--disaggregation-bootstrap-port", "8998")
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: sglangBootstrapPort,
			Name:          "bootstrap",
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return &v1beta1.RunnerSpec{
		Container: corev1.Container{
			Name:    "ome-container",
			Image:   sglangEngineImage,
			Command: cmd,
			Env: []corev1.EnvVar{
				{Name: "PYTHONUNBUFFERED", Value: "1"},
				{Name: "SGLANG_HOST_IP", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
				}},
			},
			Ports:          ports,
			ReadinessProbe: httpProbe("/health", sglangHTTPPort, 5, 30),
			LivenessProbe:  httpProbe("/health", sglangHTTPPort, 5, 60),
		},
	}
}

func sglangRouterRunner() *v1beta1.RunnerSpec {
	return &v1beta1.RunnerSpec{
		Container: corev1.Container{
			Name:  "router",
			Image: sglangRouterImage,
			Args: []string{
				"--port", "30080",
				"--k8s-namespace", "$(NAMESPACE)",
				"--prefill-label-selector", "component=engine,ome.io/inferenceservice=$(INFERENCESERVICE_NAME)",
				"--decode-label-selector", "component=decoder,ome.io/inferenceservice=$(INFERENCESERVICE_NAME)",
				"--k8s-engine-port", "30000",
				"--bootstrap-port", "8998",
			},
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
				}},
				{Name: "INFERENCESERVICE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['ome.io/inferenceservice']"},
				}},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: sglangRouterPort,
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
			}},
			ReadinessProbe: httpProbe("/readiness", sglangRouterPort, 5, 30),
			LivenessProbe:  httpProbe("/liveness", sglangRouterPort, 5, 30),
		},
	}
}

func httpProbe(path string, port, failureThreshold, periodSeconds int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		FailureThreshold: failureThreshold,
		PeriodSeconds:    periodSeconds,
		TimeoutSeconds:   10,
	}
}
