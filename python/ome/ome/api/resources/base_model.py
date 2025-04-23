import time
from typing import Optional

from kubernetes import client

from ome.api.watch import ResourceWatch
from ome.constants import constants
from ome.models import V1beta1BaseModel
from ome.utils import utils


class BaseModelClient:
    """Client for BaseModel API operations"""

    def __init__(self, base_client):
        """
        Initialize the BaseModel client
        :param base_client: The base client for API operations
        """
        self._base_client = base_client

    def create(
        self,
        basemodel: V1beta1BaseModel,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Create the base model
        :param basemodel: base model object
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the created service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: created base model
        """
        version = (
            basemodel.api_version.split("/")[1]
            if basemodel.api_version
            else constants.OME_V1BETA1_VERSION
        )

        if namespace is None:
            namespace = utils.get_default_target_namespace()

        try:
            outputs = self._base_client.api_instance.create_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_BASEMODEL,
                basemodel,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->create_namespaced_custom_object: {e}\n"
            )

        if watch:
            self.wait_ready(
                name=outputs["metadata"]["name"],
                namespace=namespace,
                timeout_seconds=timeout_seconds,
            )

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
        Get the base model
        :param name: existing base model name
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param version: api group version
        :return: base model
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        if name:
            if watch:
                self.wait_ready(
                    name=name, namespace=namespace, timeout_seconds=timeout_seconds
                )
            else:
                try:
                    return self._base_client.api_instance.get_namespaced_custom_object(
                        constants.OME_GROUP,
                        version,
                        namespace,
                        constants.OME_PLURAL_BASEMODEL,
                        name,
                    )
                except client.rest.ApiException as e:
                    raise RuntimeError(
                        f"Exception when calling CustomObjectsApi->get_namespaced_custom_object: {e}\n"
                    )
        else:
            try:
                return self._base_client.api_instance.list_namespaced_custom_object(
                    constants.OME_GROUP,
                    version,
                    namespace,
                    constants.OME_PLURAL_BASEMODEL,
                )
            except client.rest.ApiException as e:
                raise RuntimeError(
                    f"Exception when calling CustomObjectsApi->list_namespaced_custom_object: {e}\n"
                )

    def patch(
        self,
        name: str,
        basemodel: V1beta1BaseModel,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Patch existing base model
        :param name: existing base model name
        :param basemodel: patched base model
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the patched service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: patched base model
        """
        version = (
            basemodel.api_version.split("/")[1]
            if basemodel.api_version
            else constants.OME_V1BETA1_VERSION
        )

        if namespace is None:
            namespace = utils.get_default_target_namespace()

        try:
            outputs = self._base_client.api_instance.patch_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_BASEMODEL,
                name,
                basemodel,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->patch_namespaced_custom_object: {e}\n"
            )

        if watch:
            # Sleep 3 to avoid status still be True within a very short time.
            time.sleep(3)
            self.wait_ready(
                name=outputs["metadata"]["name"],
                namespace=namespace,
                timeout_seconds=timeout_seconds,
            )

        return outputs

    def replace(
        self,
        name: str,
        basemodel: V1beta1BaseModel,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Replace the existing base model
        :param name: existing base model name
        :param basemodel: replacing base model
        :param namespace: defaults to current or default namespace
        :param watch: True to watch the replaced service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: replaced base model
        """
        version = (
            basemodel.api_version.split("/")[1]
            if basemodel.api_version
            else constants.OME_V1BETA1_VERSION
        )

        if namespace is None:
            namespace = utils.get_default_target_namespace()

        if basemodel.metadata.resource_version is None:
            current_model = self.get(name, namespace=namespace)
            current_resource_version = current_model["metadata"]["resourceVersion"]
            basemodel.metadata.resource_version = current_resource_version

        try:
            outputs = self._base_client.api_instance.replace_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_BASEMODEL,
                name,
                basemodel,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->replace_namespaced_custom_object: {e}\n"
            )

        if watch:
            self.wait_ready(
                name=outputs["metadata"]["name"],
                namespace=namespace,
                timeout_seconds=timeout_seconds,
            )

        return outputs

    def delete(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Delete the base model
        :param name: base model name
        :param namespace: defaults to current or default namespace
        :param version: api group version
        :return: deletion status
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        try:
            return self._base_client.api_instance.delete_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                constants.OME_PLURAL_BASEMODEL,
                name,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->delete_namespaced_custom_object: {e}\n"
            )

    def is_ready(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Check if the base model is ready.
        :param name: base model name
        :param namespace: defaults to current or default namespace
        :param version: api group version
        :return: True if ready, False otherwise
        """
        model_status = self.get(name, namespace=namespace, version=version)
        if "status" not in model_status:
            return False

        status = model_status["status"].get("state", "")
        return status.lower() == "ready"

    @staticmethod
    def wait_ready(name=None, namespace=None, timeout_seconds=600, generation=0):
        """
        Wait until the base model is ready
        :param name: name of the base model
        :param namespace: defaults to current or default namespace
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param generation: expected generation to be observed
        :return: None
        """
        if namespace is None:
            namespace = utils.get_default_target_namespace()

        watcher = ResourceWatch()
        watcher.watch_base_model(name, namespace, timeout_seconds, generation)
