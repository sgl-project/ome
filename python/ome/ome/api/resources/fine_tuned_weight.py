import time
from typing import Optional

from kubernetes import client

from ome.api.watch import ResourceWatch
from ome.constants import constants
from ome.models import V1beta1FineTunedWeight


class FineTunedWeightClient:
    """Client for FineTunedWeight API operations"""

    def __init__(self, base_client):
        """
        Initialize the FineTunedWeight client
        :param base_client: The base client for API operations
        """
        self._base_client = base_client

    def create(
        self,
        finetunedweight: V1beta1FineTunedWeight,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Create the fine-tuned weight
        :param finetunedweight: fine-tuned weight object
        :param watch: True to watch the created service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: created fine-tuned weight
        """
        version = (
            finetunedweight.api_version.split("/")[1]
            if finetunedweight.api_version
            else constants.OME_V1BETA1_VERSION
        )

        try:
            outputs = self._base_client.api_instance.create_cluster_custom_object(
                constants.OME_GROUP,
                version,
                constants.OME_PLURAL_FINETUNEDWEIGHT,
                finetunedweight,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->create_cluster_custom_object: {e}\n"
            )

        if watch:
            self.wait_ready(
                name=outputs["metadata"]["name"],
                timeout_seconds=timeout_seconds,
            )

        return outputs

    def get(
        self,
        name: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Get the fine-tuned weight
        :param name: existing fine-tuned weight name
        :param watch: True to watch the service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param version: api group version
        :return: fine-tuned weight
        """
        if name:
            if watch:
                self.wait_ready(name=name, timeout_seconds=timeout_seconds)
            else:
                try:
                    return self._base_client.api_instance.get_cluster_custom_object(
                        constants.OME_GROUP,
                        version,
                        constants.OME_PLURAL_FINETUNEDWEIGHT,
                        name,
                    )
                except client.rest.ApiException as e:
                    raise RuntimeError(
                        f"Exception when calling CustomObjectsApi->get_cluster_custom_object: {e}\n"
                    )
        else:
            try:
                return self._base_client.api_instance.list_cluster_custom_object(
                    constants.OME_GROUP,
                    version,
                    constants.OME_PLURAL_FINETUNEDWEIGHT,
                )
            except client.rest.ApiException as e:
                raise RuntimeError(
                    f"Exception when calling CustomObjectsApi->list_cluster_custom_object: {e}\n"
                )

    def patch(
        self,
        name: str,
        finetunedweight: V1beta1FineTunedWeight,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Patch existing fine-tuned weight
        :param name: existing fine-tuned weight name
        :param finetunedweight: patched fine-tuned weight
        :param watch: True to watch the patched service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: patched fine-tuned weight
        """
        version = (
            finetunedweight.api_version.split("/")[1]
            if finetunedweight.api_version
            else constants.OME_V1BETA1_VERSION
        )

        try:
            outputs = self._base_client.api_instance.patch_cluster_custom_object(
                constants.OME_GROUP,
                version,
                constants.OME_PLURAL_FINETUNEDWEIGHT,
                name,
                finetunedweight,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->patch_cluster_custom_object: {e}\n"
            )

        if watch:
            # Sleep 3 to avoid status still be True within a very short time.
            time.sleep(3)
            self.wait_ready(
                name=outputs["metadata"]["name"],
                timeout_seconds=timeout_seconds,
            )

        return outputs

    def replace(
        self,
        name: str,
        finetunedweight: V1beta1FineTunedWeight,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """
        Replace the existing fine-tuned weight
        :param name: existing fine-tuned weight name
        :param finetunedweight: replacing fine-tuned weight
        :param watch: True to watch the replaced service until timeout elapsed or status is ready
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :return: replaced fine-tuned weight
        """
        version = (
            finetunedweight.api_version.split("/")[1]
            if finetunedweight.api_version
            else constants.OME_V1BETA1_VERSION
        )

        if finetunedweight.metadata.resource_version is None:
            current_weight = self.get(name)
            current_resource_version = current_weight["metadata"]["resourceVersion"]
            finetunedweight.metadata.resource_version = current_resource_version

        try:
            outputs = self._base_client.api_instance.replace_cluster_custom_object(
                constants.OME_GROUP,
                version,
                constants.OME_PLURAL_FINETUNEDWEIGHT,
                name,
                finetunedweight,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->replace_cluster_custom_object: {e}\n"
            )

        if watch:
            self.wait_ready(
                name=outputs["metadata"]["name"],
                timeout_seconds=timeout_seconds,
            )

        return outputs

    def delete(
        self,
        name: str,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Delete the fine-tuned weight
        :param name: fine-tuned weight name
        :param version: api group version
        :return: deletion status
        """
        try:
            return self._base_client.api_instance.delete_cluster_custom_object(
                constants.OME_GROUP,
                version,
                constants.OME_PLURAL_FINETUNEDWEIGHT,
                name,
            )
        except client.rest.ApiException as e:
            raise RuntimeError(
                f"Exception when calling CustomObjectsApi->delete_cluster_custom_object: {e}\n"
            )

    def is_ready(
        self,
        name: str,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """
        Check if the fine-tuned weight is ready.
        :param name: fine-tuned weight name
        :param version: api group version
        :return: True if ready, False otherwise
        """
        weight_status = self.get(name, version=version)
        if "status" not in weight_status:
            return False

        status = weight_status["status"].get("state", "")
        return status.lower() == "ready"

    @staticmethod
    def wait_ready(name=None, timeout_seconds=600, generation=0):
        """
        Wait until the fine tuned weight is ready
        :param name: name of the fine tuned weight
        :param timeout_seconds: timeout seconds for watch, default to 600s
        :param generation: expected generation to be observed
        :return: None
        """
        watcher = ResourceWatch()
        watcher.watch_fine_tuned_weight(name, timeout_seconds, generation)
