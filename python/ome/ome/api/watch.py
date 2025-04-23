import time

from kubernetes import client
from kubernetes import watch as k8s_watch
from tabulate import tabulate

from ome.constants import constants
from ome.utils import utils
from ome.utils.logger import logger


class ResourceWatch:
    """Class for watching Kubernetes resources in the OME system."""

    def __init__(self, api_client=None):
        """Initialize the ResourceWatch with an optional API client."""
        if api_client is None:
            api_client = client.ApiClient()
        self.api_client = api_client
        self.k8s_watch = k8s_watch.Watch()

    def _watch_resource(
        self,
        resource_plural,
        name=None,
        namespace=None,
        timeout_seconds=600,
        generation=0,
        is_namespaced=True,
        status_field="state",
        ready_value="ready",
        display_fields=None,
    ):
        """
        Generic function to watch an OME resource.

        Args:
            resource_plural: The plural name of the resource (e.g., "basemodels")
            name: The name of the specific resource to watch, or None to watch all
            namespace: The namespace to watch in (only for namespaced resources)
            timeout_seconds: Maximum time to watch for
            generation: Specific generation to watch for, or 0 for any
            is_namespaced: Whether this is a namespaced resource
            status_field: Field in resource status to check for readiness
            ready_value: Value that indicates resource is ready
            display_fields: Optional dictionary mapping display fields to paths in the resource
        """
        if is_namespaced and namespace is None:
            namespace = utils.get_default_target_namespace()

        # Define default display fields if none provided
        if display_fields is None:
            display_fields = {
                "NAME": lambda r: r["metadata"]["name"],
                "STATE": lambda r: r["status"].get(status_field, "Unknown"),
                "NODES_READY": lambda r: ",".join(r["status"].get("nodesReady", []))
                if r["status"].get("nodesReady")
                else "",
                "NODES_FAILED": lambda r: ",".join(r["status"].get("nodesFailed", []))
                if r["status"].get("nodesFailed")
                else "",
            }

        headers = list(display_fields.keys())
        table_fmt = "plain"

        # Create the stream based on whether the resource is namespaced
        if is_namespaced:
            func = client.CustomObjectsApi().list_namespaced_custom_object
            args = [
                constants.OME_GROUP,
                constants.OME_V1BETA1_VERSION,
                namespace,
                resource_plural,
            ]
        else:
            func = client.CustomObjectsApi().list_cluster_custom_object
            args = [constants.OME_GROUP, constants.OME_V1BETA1_VERSION, resource_plural]

        # Add field selector if name is provided
        kwargs = {}
        if name:
            kwargs["field_selector"] = f"metadata.name={name}"

        # Create the stream
        stream = self.k8s_watch.stream(
            func, *args, timeout_seconds=timeout_seconds, **kwargs
        )

        # Process the stream
        for event in stream:
            resource = event["object"]
            resource_name = resource["metadata"]["name"]

            # Skip if we're watching a specific resource and this isn't it
            if name and name != resource_name:
                continue

            if resource.get("status", ""):
                # Check generation if specified
                if constants.OBSERVED_GENERATION in resource["status"]:
                    observed_generation = resource["status"][
                        constants.OBSERVED_GENERATION
                    ]
                    if generation != 0 and observed_generation != generation:
                        continue

                # Display the resource status
                display_values = [
                    display_func(resource) for display_func in display_fields.values()
                ]
                logger.info(
                    tabulate(
                        [display_values],
                        headers=headers,
                        tablefmt=table_fmt,
                    )
                )

                # Break if the resource is ready
                if status_field == "type":
                    # Handle condition-based status (like InferenceService)
                    for condition in resource["status"].get("conditions", []):
                        if (
                            condition.get("type") == "Ready"
                            and condition.get("status", "").lower()
                            == ready_value.lower()
                        ):
                            return
                else:
                    # Handle direct status field (like models)
                    status_value = resource["status"].get(status_field, "").lower()
                    if status_value == ready_value.lower():
                        return
            else:
                # Display placeholder if status isn't available yet
                placeholder_values = [resource_name] + [""] * (len(headers) - 1)
                logger.info(
                    tabulate(
                        [placeholder_values],
                        headers=headers,
                        tablefmt=table_fmt,
                    )
                )
                # Sleep briefly to avoid rapid polling
                time.sleep(2)

    def watch_base_model(
        self, name=None, namespace=None, timeout_seconds=600, generation=0
    ):
        """Watch a BaseModel resource."""
        self._watch_resource(
            resource_plural=constants.OME_PLURAL_BASEMODEL,
            name=name,
            namespace=namespace,
            timeout_seconds=timeout_seconds,
            generation=generation,
            is_namespaced=True,
        )

    def watch_cluster_base_model(self, name=None, timeout_seconds=600, generation=0):
        """Watch a ClusterBaseModel resource."""
        self._watch_resource(
            resource_plural=constants.OME_PLURAL_CLUSTERBASEMODEL,
            name=name,
            timeout_seconds=timeout_seconds,
            generation=generation,
            is_namespaced=False,
        )

    def watch_fine_tuned_weight(self, name=None, timeout_seconds=600, generation=0):
        """Watch a FineTunedWeight resource."""
        self._watch_resource(
            resource_plural=constants.OME_PLURAL_FINETUNEDWEIGHT,
            name=name,
            timeout_seconds=timeout_seconds,
            generation=generation,
            is_namespaced=False,
        )

    def watch_inference_service(
        self, name=None, namespace=None, timeout_seconds=600, generation=0
    ):
        """Watch an InferenceService resource."""
        # Custom display fields for inference service
        display_fields = {
            "NAME": lambda r: r["metadata"]["name"],
            "READY": lambda r: next(
                (
                    c.get("status", "Unknown")
                    for c in r["status"].get("conditions", [])
                    if c.get("type", "") == "Ready"
                ),
                "Unknown",
            ),
            "PREV": lambda r: 100 - self._get_traffic_percent(r),
            "LATEST": lambda r: self._get_traffic_percent(r),
            "URL": lambda r: r["status"].get("url", ""),
        }

        self._watch_resource(
            resource_plural=constants.OME_PLURAL_INFERENCESERVICE,
            name=name,
            namespace=namespace,
            timeout_seconds=timeout_seconds,
            generation=generation,
            is_namespaced=True,
            status_field="type",  # InferenceService uses condition type for Ready status
            ready_value="True",  # InferenceService uses "True" string as ready value
            display_fields=display_fields,
        )

    @staticmethod
    def _get_traffic_percent(isvc):
        """Helper to extract traffic percentage from an InferenceService."""
        traffic = (
            isvc["status"].get("components", {}).get("predictor", {}).get("traffic", [])
        )
        for t in traffic:
            if t.get("latestRevision", False):
                return t.get("percent", 100)
        return 100
