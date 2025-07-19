package pod

import (
	"fmt"
	"sort"

	v1 "k8s.io/api/core/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/constants"
)

const (
	// DefaultRDMAProfile is the default RDMA profile to use if none is specified
	DefaultRDMAProfile = "oci-roce"
	// DevInfVolumeName is the name of the /dev/infiniband volume
	DevInfVolumeName = "devinf"
	// DshmVolumeName is the name of the /dev/shm volume
	DshmVolumeName = "dshm"
	// DefaultContainerName is the default container name to inject into if not specified
	DefaultContainerName = "ome-container"
)

// RDMAProfiles is a map of profile names to RDMA configurations
var RDMAProfiles = map[string]RDMAProfile{
	"oci-roce": {
		EnvVars: map[string]string{
			"NCCL_NET_PLUGIN":            "none",
			"NCCL_DEBUG":                 "INFO",
			"NCCL_CROSS_NIC":             "2",
			"NCCL_SOCKET_NTHREADS":       "16",
			"NCCL_CUMEM_ENABLE":          "0",
			"NCCL_IB_SPLIT_DATA_ON_QPS":  "0",
			"NCCL_IB_QPS_PER_CONNECTION": "16",
			"NCCL_IB_GID_INDEX":          "3",
			"NCCL_IB_HCA":                "=mlx5_0,mlx5_1,mlx5_3,mlx5_4,mlx5_5,mlx5_6,mlx5_7,mlx5_8,mlx5_9,mlx5_10,mlx5_12,mlx5_13,mlx5_14,mlx5_15,mlx5_16,mlx5_17",
			"NCCL_IB_TC":                 "41",
			"NCCL_IB_SL":                 "0",
			"NCCL_IB_TIMEOUT":            "22",
			"HCOLL_ENABLE_MCAST_ALL":     "0",
			"coll_hcoll_enable":          "0",
			"UCX_TLS":                    "tcp",
			"UCX_NET_DEVICES":            "eth0",
			"RX_QUEUE_LEN":               "8192",
			"IB_RX_QUEUE_LEN":            "8192",
			"NCCL_SOCKET_IFNAME":         "eth0",
			"NCCL_IGNORE_CPU_AFFINITY":   "1",
			"GLOO_SOCKET_IFNAME":         "eth0",
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      DshmVolumeName,
				MountPath: "/dev/shm",
			},
			{
				Name:      DevInfVolumeName,
				MountPath: "/dev/infiniband",
			},
		},
		Volumes: []v1.Volume{
			{
				Name: DshmVolumeName,
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{
						Medium: v1.StorageMediumMemory,
					},
				},
			},
			{
				Name: DevInfVolumeName,
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/dev/infiniband",
					},
				},
			},
		},
		SecurityContext: &v1.SecurityContext{
			Capabilities: &v1.Capabilities{
				Add: []v1.Capability{
					"IPC_LOCK",
					"CAP_SYS_ADMIN",
				},
			},
			Privileged: &[]bool{true}[0],
		},
	},
	// Add other profiles here as needed
}

// RDMAProfile represents configuration parameters for RDMA and NCCL
type RDMAProfile struct {
	EnvVars         map[string]string
	VolumeMounts    []v1.VolumeMount
	Volumes         []v1.Volume
	SecurityContext *v1.SecurityContext
}

// RDMAInjector is responsible for injecting RDMA and NCCL configurations
type RDMAInjector struct{}

// NewRDMAInjector creates a new RDMAInjector
func NewRDMAInjector() *RDMAInjector {
	return &RDMAInjector{}
}

// InjectRDMA injects RDMA and NCCL configurations if the auto-inject annotation is set
func (ri *RDMAInjector) InjectRDMA(pod *v1.Pod) error {
	if autoInject, ok := pod.ObjectMeta.Annotations[constants.RDMAAutoInjectAnnotationKey]; ok && autoInject == "true" {
		profileName := DefaultRDMAProfile
		if profile, ok := pod.ObjectMeta.Annotations[constants.RDMAProfileAnnotationKey]; ok && profile != "" {
			profileName = profile
		}

		profile, ok := RDMAProfiles[profileName]
		if !ok {
			return fmt.Errorf("unknown RDMA profile: %s", profileName)
		}

		return ri.injectRDMAConfig(pod, profile)
	}
	return nil
}

// injectRDMAConfig adds RDMA and NCCL configurations to the specified container in the pod
func (ri *RDMAInjector) injectRDMAConfig(pod *v1.Pod, profile RDMAProfile) error {
	// Get the target container name from annotation or use default
	targetContainerName := DefaultContainerName
	if containerName, ok := pod.ObjectMeta.Annotations[constants.RDMAContainerNameAnnotationKey]; ok && containerName != "" {
		targetContainerName = containerName
	}

	// Find the target container first before modifying the pod
	containerFound := false
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == targetContainerName {
			containerFound = true
			break
		}
	}

	// If container not found, log and return without modifying the pod
	if !containerFound {
		logger := ctrllog.Log.WithName("rdma-injector")
		logger.Info("RDMA injection skipped: container not found",
			"container", targetContainerName,
			"pod", pod.Name,
			"namespace", pod.Namespace)
		return nil
	}

	// Add volumes to pod if they don't already exist
	ri.injectVolumes(pod, profile.Volumes)

	// Now inject into the target container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == targetContainerName {
			ri.injectContainerConfig(&pod.Spec.Containers[i], profile)
			break
		}
	}

	return nil
}

// injectContainerConfig adds environment variables, volume mounts, and security context to a container
func (ri *RDMAInjector) injectContainerConfig(container *v1.Container, profile RDMAProfile) {
	// Add environment variables in sorted order for deterministic behavior
	var keys []string
	for name := range profile.EnvVars {
		keys = append(keys, name)
	}
	// Sort keys for stable ordering
	sort.Strings(keys)

	// Add environment variables in sorted order
	for _, name := range keys {
		container.Env = append(container.Env, v1.EnvVar{
			Name:  name,
			Value: profile.EnvVars[name],
		})
	}

	// Add volume mounts if they don't already exist
	for _, mount := range profile.VolumeMounts {
		if !ri.volumeMountExists(container, mount.Name) {
			container.VolumeMounts = append(container.VolumeMounts, mount)
		}
	}

	// Set security context if not already set
	if container.SecurityContext == nil {
		container.SecurityContext = profile.SecurityContext.DeepCopy()
	} else {
		// Merge capabilities
		if container.SecurityContext.Capabilities == nil {
			container.SecurityContext.Capabilities = profile.SecurityContext.Capabilities.DeepCopy()
		} else {
			for _, cap := range profile.SecurityContext.Capabilities.Add {
				if !ri.capabilityExists(container.SecurityContext.Capabilities.Add, cap) {
					container.SecurityContext.Capabilities.Add = append(container.SecurityContext.Capabilities.Add, cap)
				}
			}
		}

		// Set privileged if not already set
		if container.SecurityContext.Privileged == nil {
			container.SecurityContext.Privileged = profile.SecurityContext.Privileged
		}
	}
}

// volumeMountExists checks if a volume mount with the given name already exists in the container
func (ri *RDMAInjector) volumeMountExists(container *v1.Container, name string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == name {
			return true
		}
	}
	return false
}

// volumeExists checks if a volume with the given name already exists in the pod
func (ri *RDMAInjector) volumeExists(pod *v1.Pod, name string) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

// injectVolumes adds volumes to the pod if they don't already exist
func (ri *RDMAInjector) injectVolumes(pod *v1.Pod, volumes []v1.Volume) {
	for _, volume := range volumes {
		if !ri.volumeExists(pod, volume.Name) {
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
		}
	}
}

// capabilityExists checks if a capability already exists in the capabilities list
func (ri *RDMAInjector) capabilityExists(capabilities []v1.Capability, capability v1.Capability) bool {
	for _, cap := range capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}
