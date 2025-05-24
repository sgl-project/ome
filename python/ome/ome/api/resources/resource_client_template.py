"""Template for implementing new resource clients"""

from typing import Any, Dict, Optional

from kubernetes import client

from ome.constants import constants


class ResourceClientTemplate:
    """Template for a resource client

    This is a reference implementation for creating new resource clients.
    Copy this file and replace ResourceClientTemplate with your resource name.

    Replace RESOURCE_NAME, RESOURCE_KIND, and RESOURCE_PLURAL with appropriate values.
    """

    # Constants specific to this resource
    RESOURCE_KIND = "ResourceKind"
    RESOURCE_PLURAL = "resourcekinds"

    def __init__(self, base_client):
        """
        Initialize the resource client
        :param base_client: The base client for API operations
        """
        self._base_client = base_client

    def create(self, resource_object: Any, namespace: Optional[str] = None, **kwargs):
        """
        Create a new resource
        :param resource_object: resource object to create
        :param namespace: defaults to current or default namespace
        :return: created resource
        """
        version = resource_object.api_version.split("/")[1]

        # Set the namespace if needed
        # if namespace is None:
        #     namespace = utils.get_default_target_namespace()

        try:
            return self._base_client.api_instance.create_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                self.RESOURCE_PLURAL,
                resource_object,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->create_namespaced_custom_object: {e}"
            )

    def get(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
        **kwargs,
    ):
        """
        Get the resource
        :param name: existing resource name
        :param namespace: defaults to current or default namespace
        :param version: api group version
        :return: resource object or list if name is None
        """
        # Set the namespace if needed
        # if namespace is None:
        #     namespace = utils.get_default_target_namespace()

        if name:
            try:
                return self._base_client.api_instance.get_namespaced_custom_object(
                    constants.OME_GROUP,
                    version,
                    namespace,
                    self.RESOURCE_PLURAL,
                    name,
                )
            except client.rest.ApiException as e:
                raise RuntimeError(
                    f"Exception when calling CustomObjectsApi->get_namespaced_custom_object: {e}"
                )
        else:
            try:
                return self._base_client.api_instance.list_namespaced_custom_object(
                    constants.OME_GROUP,
                    version,
                    namespace,
                    self.RESOURCE_PLURAL,
                )
            except client.rest.ApiException as e:
                raise RuntimeError(
                    f"Exception when calling CustomObjectsApi->list_namespaced_custom_object: {e}"
                )

    def patch(
        self, name: str, resource_object: Any, namespace: Optional[str] = None, **kwargs
    ):
        """
        Patch existing resource
        :param name: existing resource name
        :param resource_object: patched resource
        :param namespace: defaults to current or default namespace
        :return: patched resource
        """
        version = resource_object.api_version.split("/")[1]
        # Set the namespace if needed
        # if namespace is None:
        #     namespace = utils.get_default_target_namespace()

        try:
            return self._base_client.api_instance.patch_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                self.RESOURCE_PLURAL,
                name,
                resource_object,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->patch_namespaced_custom_object: {e}"
            )

    def replace(
        self, name: str, resource_object: Any, namespace: Optional[str] = None, **kwargs
    ):
        """
        Replace the existing resource
        :param name: existing resource name
        :param resource_object: replacing resource
        :param namespace: defaults to current or default namespace
        :return: replaced resource
        """
        version = resource_object.api_version.split("/")[1]
        # Set the namespace if needed
        # if namespace is None:
        #     namespace = utils.get_default_target_namespace()

        # Update resource version if needed
        # if resource_object.metadata.resource_version is None:
        #     current = self.get(name, namespace=namespace)
        #     current_resource_version = current["metadata"]["resourceVersion"]
        #     resource_object.metadata.resource_version = current_resource_version

        try:
            return self._base_client.api_instance.replace_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                self.RESOURCE_PLURAL,
                name,
                resource_object,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->replace_namespaced_custom_object: {e}"
            )

    def delete(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Delete the resource
        :param name: resource name
        :param namespace: defaults to current or default namespace
        :param version: api group version
        :return: deletion status
        """
        # Set the namespace if needed
        # if namespace is None:
        #     namespace = utils.get_default_target_namespace()

        try:
            return self._base_client.api_instance.delete_namespaced_custom_object(
                constants.OME_GROUP,
                version,
                namespace,
                self.RESOURCE_PLURAL,
                name,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->delete_namespaced_custom_object: {e}"
            )
