package runtime

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	kueuelr "sigs.k8s.io/kueue/pkg/util/limitrange"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

type Info struct {
	// Labels and Annotations to add to the RuntimeJobTemplate.
	Labels      map[string]string
	Annotations map[string]string
	// Original policy values from the runtime.
	RuntimePolicy RuntimePolicy
	// Trainer parameters to add to the RuntimeJobTemplate.
	Trainer
	// Scheduler parameters to add to the RuntimeJobTemplate.
	*Scheduler
}

type RuntimePolicy struct {
	MLPolicy       *omev1beta1.MLPolicy
	PodGroupPolicy *omev1beta1.PodGroupPolicy
}

type Trainer struct {
	NumNodes *int32
	// TODO. Potentially, we can use map for env and sort it to improve code.
	Env           []corev1.EnvVar
	ContainerPort *corev1.ContainerPort
	Volumes       []corev1.Volume
	Affinity      *corev1.Affinity
}

// TODO: Potentially, we can add ScheduleTimeoutSeconds to the Scheduler for consistency.
type Scheduler struct {
	PodLabels     map[string]string
	TotalRequests map[string]TotalResourceRequest
}

type TotalResourceRequest struct {
	Replicas    int32
	PodRequests corev1.ResourceList
}

type InfoOptions struct {
	labels          map[string]string
	annotations     map[string]string
	runtimePolicy   RuntimePolicy
	podSpecReplicas []podSpecReplica
	volumes         []corev1.Volume
	affinity        *corev1.Affinity
}

type InfoOption func(options *InfoOptions)

var defaultOptions = InfoOptions{}

type podSpecReplica struct {
	replicas int32
	name     string
	podSpec  corev1.PodSpec
}

func WithLabels(labels map[string]string) InfoOption {
	return func(o *InfoOptions) {
		o.labels = maps.Clone(labels)
	}
}

func WithAnnotations(annotations map[string]string) InfoOption {
	return func(o *InfoOptions) {
		o.annotations = maps.Clone(annotations)
	}
}

func WithMLPolicy(mlPolicy *omev1beta1.MLPolicy) InfoOption {
	return func(o *InfoOptions) {
		o.runtimePolicy.MLPolicy = mlPolicy
	}
}

func WithPodGroupPolicy(pgPolicy *omev1beta1.PodGroupPolicy) InfoOption {
	return func(o *InfoOptions) {
		o.runtimePolicy.PodGroupPolicy = pgPolicy
	}
}

func WithPodSpecReplicas(replicaName string, replicas int32, podSpec corev1.PodSpec) InfoOption {
	return func(o *InfoOptions) {
		o.podSpecReplicas = append(o.podSpecReplicas, podSpecReplica{
			name:     replicaName,
			replicas: replicas,
			podSpec:  podSpec,
		})
	}
}

func WithVolumes(volumes []corev1.Volume) InfoOption {
	return func(o *InfoOptions) {
		o.volumes = volumes
	}
}

func WithAffinity(affinity *corev1.Affinity) InfoOption {
	return func(o *InfoOptions) {
		o.affinity = affinity
	}
}

func NewInfo(opts ...InfoOption) *Info {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	info := &Info{
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
		RuntimePolicy: options.runtimePolicy,
		Scheduler: &Scheduler{
			TotalRequests: make(map[string]TotalResourceRequest, len(options.podSpecReplicas)),
		},
	}

	for _, spec := range options.podSpecReplicas {
		info.TotalRequests[spec.name] = TotalResourceRequest{
			Replicas: spec.replicas,
			// TODO: Need to address LimitRange and RuntimeClass.
			PodRequests: kueuelr.TotalRequests(&spec.podSpec),
		}
	}
	if options.labels != nil {
		info.Labels = options.labels
	}
	if options.annotations != nil {
		info.Annotations = options.annotations
	}
	if options.volumes != nil {
		info.Volumes = options.volumes
	}
	if options.affinity != nil {
		info.Affinity = options.affinity
	}

	return info
}
