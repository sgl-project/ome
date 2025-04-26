import time
from typing import Any, Dict, Optional, Union

from kubernetes import client, config

from ome.api.resources.base_model import BaseModelClient
from ome.api.resources.cluster_base_model import ClusterBaseModelClient
from ome.api.resources.fine_tuned_weight import FineTunedWeightClient
from ome.api.resources.inference_service import InferenceServiceClient
from ome.constants import constants
from ome.models import V1beta1InferenceService
from ome.utils import utils


class BaseClient:
    """Base client for all OME API resources"""

    def __init__(
        self,
        config_file: Optional[str] = None,
        config_dict: Optional[Dict[str, Any]] = None,
        context: Optional[str] = None,
        client_configuration: Optional[client.Configuration] = None,
        persist_config: bool = True,
    ):
        """
        Base OME client constructor
        :param config_file: kubeconfig file, defaults to ~/.kube/config
        :param config_dict: Takes the config file as a dict.
        :param context: kubernetes context
        :param client_configuration: kubernetes configuration object
        :param persist_config:
        """
        if config_file or config_dict or not utils.is_running_in_k8s():
            if config_dict:
                config.load_kube_config_from_dict(
                    config_dict=config_dict,
                    context=context,
                    client_configuration=None,
                    persist_config=persist_config,
                )
            else:
                config.load_kube_config(
                    config_file=config_file,
                    context=context,
                    client_configuration=client_configuration,
                    persist_config=persist_config,
                )
        else:
            config.load_incluster_config()
        self.core_api = client.CoreV1Api()
        self.app_api = client.AppsV1Api()
        self.api_instance = client.CustomObjectsApi()
        self.hpa_v2_api = client.AutoscalingV2Api()


class OMEClient:
    """Client for OME API resources"""

    def __init__(
        self,
        config_file: Optional[str] = None,
        config_dict: Optional[Dict[str, Any]] = None,
        context: Optional[str] = None,
        client_configuration: Optional[client.Configuration] = None,
        persist_config: bool = True,
    ):
        """
        OME client constructor
        :param config_file: kubeconfig file, defaults to ~/.kube/config
        :param config_dict: Takes the config file as a dict.
        :param context: kubernetes context
        :param client_configuration: kubernetes configuration object
        :param persist_config:
        """
        self._base_client = BaseClient(
            config_file=config_file,
            config_dict=config_dict,
            context=context,
            client_configuration=client_configuration,
            persist_config=persist_config,
        )

        # Initialize resource clients
        self._inference_service = InferenceServiceClient(self._base_client)
        self._cluster_base_model = ClusterBaseModelClient(self._base_client)
        self._base_model = BaseModelClient(self._base_client)
        self._fine_tuned_weight = FineTunedWeightClient(self._base_client)

    @property
    def inference_service(self) -> InferenceServiceClient:
        """Get the InferenceService client"""
        return self._inference_service

    @property
    def cluster_base_model(self) -> ClusterBaseModelClient:
        """Get the ClusterBaseModel client"""
        return self._cluster_base_model

    @property
    def base_model(self) -> BaseModelClient:
        """Get the BaseModel client"""
        return self._base_model

    @property
    def fine_tuned_weight(self) -> FineTunedWeightClient:
        """Get the FineTunedWeight client"""
        return self._fine_tuned_weight

    # For backwards compatibility
    def create_inference_service(
        self,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """Backwards compatibility method for create_inference_service"""
        return self._inference_service.create(
            inferenceservice=inferenceservice,
            namespace=namespace,
            watch=watch,
            timeout_seconds=timeout_seconds,
        )

    def get_inference_service(
        self,
        name: Optional[str] = None,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """Backwards compatibility method for get_inference_service"""
        return self._inference_service.get(
            name=name,
            namespace=namespace,
            watch=watch,
            timeout_seconds=timeout_seconds,
            version=version,
        )

    def patch_inference_service(
        self,
        name: str,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """Backwards compatibility method for patch_inference_service"""
        return self._inference_service.patch(
            name=name,
            inferenceservice=inferenceservice,
            namespace=namespace,
            watch=watch,
            timeout_seconds=timeout_seconds,
        )

    def replace_inference_service(
        self,
        name: str,
        inferenceservice: V1beta1InferenceService,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
    ):
        """Backwards compatibility method for replace_inference_service"""
        return self._inference_service.replace(
            name=name,
            inferenceservice=inferenceservice,
            namespace=namespace,
            watch=watch,
            timeout_seconds=timeout_seconds,
        )

    def delete_inference_service(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """Backwards compatibility method for delete_inference_service"""
        return self._inference_service.delete(
            name=name,
            namespace=namespace,
            version=version,
        )

    def is_isvc_ready(
        self,
        name: str,
        namespace: Optional[str] = None,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """Backwards compatibility method for is_isvc_ready"""
        return self._inference_service.is_ready(
            name=name,
            namespace=namespace,
            version=version,
        )

    def wait_isvc_ready(
        self,
        name: str,
        namespace: Optional[str] = None,
        watch: bool = False,
        timeout_seconds: int = 600,
        polling_interval: int = 10,
        version: str = constants.OME_V1BETA1_VERSION,
    ):
        """Backwards compatibility method for wait_isvc_ready"""
        return self._inference_service.wait_ready(
            name=name,
            namespace=namespace,
            watch=watch,
            timeout_seconds=timeout_seconds,
            polling_interval=polling_interval,
            version=version,
        )
