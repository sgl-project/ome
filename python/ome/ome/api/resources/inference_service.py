import time
from typing import Any, Dict, Optional, Union

from kubernetes import client

from ome.api.watch import ResourceWatch
from ome.constants import constants
from ome.models import V1beta1InferenceService
from ome.utils import utils


class InferenceServiceClient:
    """Client for InferenceService API operations"""

    def __init__(self, base_client):
        """
        Initialize the InferenceService client
        :param base_client: The base client for API operations
        """
        self._base_client = base_client

    def create(
        self,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Create the inference service
        :param inferenceservice: inference service object
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the created service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: created inference service
        """
        version = inferenceservice.api_version.split("/")[1]

        if namespace is None:
            namespace = utils.get_inference_service_namespace(inferenceservice)

        try:
            outputs = self._base_client.api_instance.create_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_INFERENCESERVICE,
                inferenceservice,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                "Exception when calling CustomObjectsApi->create_namespaced_custom_object:\
                 %s\n"
                % e
            )

        if watch:
            watcher = ResourceWatch()
            watcher.watch_inference_service(
                outputs["metadata"]["name"], namespace, timeout_seconds
            )
        else:
            return outputs

    def get(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Get the inference service
        :param name: existing inference service name
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param version: api group version
        :return: inference service
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        if name:
            if watch:
                watcher = ResourceWatch()
                watcher.watch_inference_service(name, namespace, timeout_seconds)
            else:
                try:
                    return self._base_client.api_instance.get_namespaced_custom_object(
                        constants.OME_GROUP,
                        version,
                        namespace,
                        constants.OME_PLURAL_INFERENCESERVICE,
                        name,
                    )
                except client.rest.ApiException as e:
                    raise RuntimeError(
                        "Exception when calling CustomObjectsApi->get_namespaced_custom_object:\
                        %s\n"
                        % e
                    )
        else:
            if watch:
                watcher = ResourceWatch()
                watcher.watch_inference_service(namespace, timeout_seconds)
            else:
                try:
                    return self._base_client.api_instance.list_namespaced_custom_object(
                        constants.OME_GROUP,
                        version,
                        namespace,
                        constants.OME_PLURAL_INFERENCESERVICE,
                    )
                except client.rest.ApiException as e:
                    raise RuntimeError(
                        "Exception when calling CustomObjectsApi->list_namespaced_custom_object:\
                        %s\n"
                        % e
                    )

    def patch(
        self,
        name: str,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Patch existing inference service
        :param name: existing inference service name
        :param inferenceservice: patched inference service
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the patched service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: patched inference service
        """
        version = inferenceservice.api_version.split("/")[1]
        if namespace is None:
            namespace = utils.get_inference_service_namespace(inferenceservice)

        try:
            outputs = self._base_client.api_instance.patch_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_INFERENCESERVICE,
                name,
                inferenceservice,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                "Exception when calling CustomObjectsApi->patch_namespaced_custom_object:\
                 %s\n"
                % e
            )

        if watch:
            # Sleep 3 to avoid status still be True within a very short time.
            time.sleep(3)
            watcher = ResourceWatch()
            watcher.watch_inference_service(
                outputs["metadata"]["name"], namespace, timeout_seconds
            )
        else:
            return outputs

    def replace(
        self,
        name: str,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Replace the existing inference service
        :param name: existing inference service name
        :param inferenceservice: replacing inference service
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the replaced service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: replaced inference service
        """
        version = inferenceservice.api_version.split("/")[1]

        if namespace is None:
            namespace = utils.get_inference_service_namespace(inferenceservice)

        if inferenceservice.metadata.resource_version is None:
            current_isvc = self.get(name, namespace=namespace)
            current_resource_version = current_isvc["metadata"]["resourceVersion"]
            inferenceservice.metadata.resource_version = current_resource_version

        try:
            outputs = self._base_client.api_instance.replace_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_INFERENCESERVICE,
                name,
                inferenceservice,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                "Exception when calling CustomObjectsApi->replace_namespaced_custom_object:\
                 %s\n"
                % e
            )

        if watch:
            watcher = ResourceWatch()
            watcher.watch_inference_service(
                outputs["metadata"]["name"],
                namespace,
                timeout_seconds,
                outputs["metadata"]["generation"],
            )
        else:
            return outputs

    def delete(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Delete the inference service
        :param name: inference service name
        :param namespace: defaults to current or default namespace
        :param version: api group version
        :return:
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        try:
            return self._base_client.api_instance.delete_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_INFERENCESERVICE,
                name,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                "Exception when calling CustomObjectsApi->delete_namespaced_custom_object:\
                 %s\n"
                % e
            )

    def is_ready(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Check if the inference service is ready.
        :param version:
        :param name: inference service name
        :param namespace: defaults to current or default namespace
        :return:
        """
        isvc_status = self.get(name, namespace=namespace, version=version)
        if "status" not in isvc_status:
            return False
        status = "Unknown"
        for condition in isvc_status["status"].get("conditions", {}):
            if condition.get("type", "") == "Ready":
                status = condition.get("status", "Unknown")
                return status.lower() == "true"
        return False

    @staticmethod
    def wait_ready(
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        timeout_seconds: int = 600,
        generation: int = 0,
    ):
        """
        Wait until the inference service is ready
        :param name: name of the inference service
        :param namespace: defaults to current or default namespace
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param generation: expected generation to be observed
        :return: None
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        watcher = ResourceWatch()
        watcher.watch_inference_service(name, namespace, timeout_seconds, generation)
