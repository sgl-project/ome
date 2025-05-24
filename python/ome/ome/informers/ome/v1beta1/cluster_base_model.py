"""
ClusterBaseModel informer implementation.
"""

import time
from typing import Callable, Dict, List, Optional

from kubernetes.client import ApiClient
from kubernetes.client.rest import ApiException

from ome import V1beta1ClusterBaseModel
from ome.api.ome_client import OMEClient
from ome.informers.internal_interfaces import (
    SharedInformerFactory,
    TweakListOptionsFunc,
)
from ome.informers.shared_informer import SharedIndexInformer, SharedInformerImpl
from ome.utils.logger import logger


class ClusterBaseModelInformer:
    """
    ClusterBaseModelInformer provides access to a shared informer and lister for ClusterBaseModels.
    """

    def __init__(
        self,
        factory: SharedInformerFactory,
        tweak_list_options: TweakListOptionsFunc = None,
    ):
        """
        Initialize a new ClusterBaseModelInformer.

        Args:
            factory: The shared informer factory
            tweak_list_options: Optional function to tweak list options
        """
        self._factory = factory
        self._tweak_list_options = tweak_list_options

    def informer(self) -> SharedIndexInformer:
        """
        Get the shared index informer for this resource.

        Returns:
            A SharedIndexInformer
        """
        return self._factory.informer_for(
            V1beta1ClusterBaseModel, self._default_informer
        )

    def lister(self):
        """
        Get a lister for this resource.

        Returns:
            A ClusterBaseModelLister
        """
        from ome.listers.ome.v1beta1.cluster_base_model import ClusterBaseModelLister

        return ClusterBaseModelLister(self.informer().get_indexer())

    def _default_informer(
        self, client: OMEClient, resync_period: float
    ) -> SharedIndexInformer:
        """
        Create a default informer for ClusterBaseModel.

        Args:
            client: The OmeClient to use
            resync_period: Resync period in seconds

        Returns:
            A SharedIndexInformer for ClusterBaseModel
        """

        # Define list and watch functions
        def list_func(**kwargs):
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

            return client.cluster_base_models.list(**list_options)

        def watch_func(**kwargs):
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

            return client.cluster_base_models.watch(**watch_options)

        # Create and return the informer
        return SharedInformerImpl(
            client=client,
            resource_type=V1beta1ClusterBaseModel,
            list_func=list_func,
            watch_func=watch_func,
            resync_period=resync_period,
        )
