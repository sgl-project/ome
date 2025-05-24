"""
InferenceService informer implementation.
"""

import time
from typing import Callable, Dict, List, Optional

from kubernetes.client import ApiClient
from kubernetes.client.rest import ApiException

from ome import V1beta1InferenceService
from ome.api.ome_client import OMEClient
from ome.informers.internal_interfaces import (
    SharedInformerFactory,
    TweakListOptionsFunc,
)
from ome.informers.shared_informer import SharedIndexInformer, SharedInformerImpl
from ome.utils.logger import logger


class InferenceServiceInformer:
    """
    InferenceServiceInformer provides access to a shared informer and lister for InferenceServices.
    """

    def __init__(
        self,
        factory: SharedInformerFactory,
        namespace: str,
        tweak_list_options: TweakListOptionsFunc = None,
    ):
        """
        Initialize a new InferenceServiceInformer.

        Args:
            factory: The shared informer factory
            namespace: Namespace to restrict the informer to
            tweak_list_options: Optional function to tweak list options
        """
        self._factory = factory
        self._namespace = namespace
        self._tweak_list_options = tweak_list_options

    def informer(self) -> SharedIndexInformer:
        """
        Get the shared index informer for this resource.

        Returns:
            A SharedIndexInformer
        """
        return self._factory.informer_for(
            V1beta1InferenceService, self._default_informer
        )

    def lister(self):
        """
        Get a lister for this resource.

        Returns:
            An InferenceServiceLister
        """
        from ome.listers.ome.v1beta1.inference_service import InferenceServiceLister

        return InferenceServiceLister(self.informer().get_indexer())

    def _default_informer(
        self, client: OMEClient, resync_period: float
    ) -> SharedIndexInformer:
        """
        Create a default informer for InferenceService.

        Args:
            client: The OmeClient to use
            resync_period: Resync period in seconds

        Returns:
            A SharedIndexInformer for InferenceService
        """

        # Define list and watch functions
        def list_func(**kwargs):
            namespace = kwargs.get("namespace", self._namespace)
            field_selector = kwargs.get("field_selector")
            label_selector = kwargs.get("label_selector")

            list_options = {}
            if field_selector:
                list_options["field_selector"] = field_selector
            if label_selector:
                list_options["label_selector"] = label_selector

            # Apply any custom list options tweaks
            if self._tweak_list_options:
                self._tweak_list_options(list_options)

            return client.inference_services.list(namespace=namespace, **list_options)

        def watch_func(**kwargs):
            namespace = kwargs.get("namespace", self._namespace)
            field_selector = kwargs.get("field_selector")
            label_selector = kwargs.get("label_selector")
            resource_version = kwargs.get("resource_version")

            watch_options = {}
            if field_selector:
                watch_options["field_selector"] = field_selector
            if label_selector:
                watch_options["label_selector"] = label_selector
            if resource_version:
                watch_options["resource_version"] = resource_version

            # Apply any custom list options tweaks
            if self._tweak_list_options:
                self._tweak_list_options(watch_options)

            return client.inference_services.watch(namespace=namespace, **watch_options)

        # Create and return the informer
        return SharedInformerImpl(
            client=client,
            resource_type=V1beta1InferenceService,
            list_func=list_func,
            watch_func=watch_func,
            namespace=self._namespace,
            resync_period=resync_period,
        )
