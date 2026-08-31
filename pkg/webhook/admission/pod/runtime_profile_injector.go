package pod

import (
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/constants"
)

// Runtime profile injectors factor out repetitive ServingRuntime YAML
// (probes, /dev/shm, prometheus annotations). Each is gated on its own
// profile annotation; profiles never overwrite operator-set values.

const DefaultRuntimeProbePort = 30000

func resolveRuntimeTargetContainer(pod *v1.Pod) string {
	if name, ok := pod.ObjectMeta.Annotations[constants.RuntimeContainerNameAnnotationKey]; ok && name != "" {
		return name
	}
	return DefaultContainerName
}

func findContainer(pod *v1.Pod, name string) *v1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	return nil
}

// ----- shm ---------------------------------------------------------------

// ShmProfiles enumerates the recognized values for
// runtime.ome.io/shm-profile. Only "default" exists today: it injects
// a memory-backed /dev/shm emptyDir without a sizeLimit, leaving
// per-pod sizing under the kernel default. Runtimes that need an
// explicit sizeLimit set the volume directly in YAML — sizing is
// deployment-specific (single-tenant devbox vs multi-pod node) and
// shouldn't be baked into a one-size-fits-all profile.
var ShmProfiles = map[string]struct{}{
	"default": {},
}

type ShmInjector struct{}

func NewShmInjector() *ShmInjector { return &ShmInjector{} }

func (s *ShmInjector) InjectShm(pod *v1.Pod) error {
	name, ok := pod.ObjectMeta.Annotations[constants.RuntimeShmProfileAnnotationKey]
	if !ok || name == "" {
		return nil
	}
	if _, ok := ShmProfiles[name]; !ok {
		return fmt.Errorf("unknown shm profile: %s", name)
	}
	target := findContainer(pod, resolveRuntimeTargetContainer(pod))
	if target == nil {
		return nil
	}

	if !volumeNameExists(pod, DshmVolumeName) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: DshmVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{Medium: v1.StorageMediumMemory},
			},
		})
	}
	if !containerHasVolumeMount(target, DshmVolumeName) {
		target.VolumeMounts = append(target.VolumeMounts, v1.VolumeMount{
			Name:      DshmVolumeName,
			MountPath: "/dev/shm",
		})
	}
	return nil
}

// ----- probes ------------------------------------------------------------

type ProbeProfile struct {
	Path string
}

var ProbeProfiles = map[string]ProbeProfile{
	"sglang-http": {Path: "/health"},
	"vllm-http":   {Path: "/health"},
}

type ProbeInjector struct{}

func NewProbeInjector() *ProbeInjector { return &ProbeInjector{} }

func (p *ProbeInjector) InjectProbes(pod *v1.Pod) error {
	name, ok := pod.ObjectMeta.Annotations[constants.RuntimeProbeProfileAnnotationKey]
	if !ok || name == "" {
		return nil
	}
	profile, ok := ProbeProfiles[name]
	if !ok {
		return fmt.Errorf("unknown probe profile: %s", name)
	}
	port, err := resolveProbePort(pod)
	if err != nil {
		return err
	}
	target := findContainer(pod, resolveRuntimeTargetContainer(pod))
	if target == nil {
		return nil
	}

	// Inject only the probes the runtime hasn't declared, so a YAML can
	// override any single probe (e.g. liveness only) without losing the
	// other two.
	if target.ReadinessProbe == nil {
		target.ReadinessProbe = httpProbe(profile.Path, port, 60, 30, 30, 5, 1)
	}
	if target.LivenessProbe == nil {
		target.LivenessProbe = httpProbe(profile.Path, port, 0, 60, 30, 5, 0)
	}
	if target.StartupProbe == nil {
		// failure × period ≈ 30 min: multi-shard model loads can be that slow.
		target.StartupProbe = httpProbe(profile.Path, port, 60, 30, 30, 60, 0)
	}
	return nil
}

func resolveProbePort(pod *v1.Pod) (int32, error) {
	return resolvePort(pod, constants.RuntimeProbePortAnnotationKey)
}

// httpProbe; pass 0 for initialDelay/successThreshold to fall through to
// the apiserver default.
func httpProbe(path string, port int32, initialDelaySec, periodSec, timeoutSec, failureThreshold, successThreshold int32) *v1.Probe {
	p := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: initialDelaySec,
		PeriodSeconds:       periodSec,
		TimeoutSeconds:      timeoutSec,
		FailureThreshold:    failureThreshold,
	}
	if successThreshold > 0 {
		p.SuccessThreshold = successThreshold
	}
	return p
}

// ----- observability ----------------------------------------------------

type ObservabilityProfile struct {
	Path string
}

var ObservabilityProfiles = map[string]ObservabilityProfile{
	"prometheus": {Path: "/metrics"},
}

type ObservabilityInjector struct{}

func NewObservabilityInjector() *ObservabilityInjector { return &ObservabilityInjector{} }

func (o *ObservabilityInjector) InjectObservability(pod *v1.Pod) error {
	name, ok := pod.ObjectMeta.Annotations[constants.RuntimeObservabilityProfileAnnotationKey]
	if !ok || name == "" {
		return nil
	}
	profile, ok := ObservabilityProfiles[name]
	if !ok {
		return fmt.Errorf("unknown observability profile: %s", name)
	}
	port, err := resolveObservabilityPort(pod)
	if err != nil {
		return err
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	setIfAbsent(pod.Annotations, constants.PrometheusScrapeAnnotationKey, "true")
	setIfAbsent(pod.Annotations, constants.PrometheusPortAnnotationKey, strconv.Itoa(int(port)))
	setIfAbsent(pod.Annotations, constants.PrometheusPathAnnotationKey, profile.Path)
	return nil
}

func resolveObservabilityPort(pod *v1.Pod) (int32, error) {
	return resolvePort(pod, constants.RuntimeObservabilityPortAnnotationKey)
}

// ----- helpers ----------------------------------------------------------

func resolvePort(pod *v1.Pod, key string) (int32, error) {
	v, ok := pod.ObjectMeta.Annotations[key]
	if !ok || v == "" {
		return DefaultRuntimeProbePort, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 || n > 65535 {
		return 0, fmt.Errorf("invalid port %q (annotation %s)", v, key)
	}
	return int32(n), nil
}

func volumeNameExists(pod *v1.Pod, name string) bool {
	for _, v := range pod.Spec.Volumes {
		if v.Name == name {
			return true
		}
	}
	return false
}

func containerHasVolumeMount(c *v1.Container, name string) bool {
	for _, m := range c.VolumeMounts {
		if m.Name == name {
			return true
		}
	}
	return false
}

func setIfAbsent(m map[string]string, k, v string) {
	if _, ok := m[k]; !ok {
		m[k] = v
	}
}
