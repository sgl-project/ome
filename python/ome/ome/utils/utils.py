import os
import sys

import psutil


def is_running_in_k8s():
    return os.path.isdir("/var/run/secrets/kubernetes.io/")


def get_current_k8s_namespace():
    with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace", "r") as f:
        return f.readline()


def get_default_target_namespace():
    if not is_running_in_k8s():
        return "default"
    return get_current_k8s_namespace()


def get_inference_service_namespace(inferenceservice):
    return inferenceservice.metadata.namespace or get_default_target_namespace()


def cpu_count():
    """Get the available CPU count for this system.
    Takes the minimum value from the following locations:
    - Total system cpus available on the host.
    - CPU Affinity (if set)
    - Cgroups limit (if set)
    """
    count = os.cpu_count()

    # Check CPU affinity if available
    try:
        affinity_count = len(psutil.Process().cpu_affinity())
        if affinity_count > 0:
            count = min(count, affinity_count)
    except Exception:
        pass

    # Check cgroups if available
    if sys.platform == "linux":
        try:
            with open("/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us") as f:
                quota = int(f.read())
            with open("/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us") as f:
                period = int(f.read())
            cgroups_count = int(quota / period)
            if cgroups_count > 0:
                count = min(count, cgroups_count)
        except Exception:
            pass

    return count
